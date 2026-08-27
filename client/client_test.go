package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCommandPreservesArgvAndSignature(t *testing.T) {
	var nonce [nonceLen]byte
	for index := range nonce {
		nonce[index] = byte(index)
	}
	args := []string{"-i", "a b!?", "pipe:1"}
	payload, err := commandPayload("test-secret", "ffmpeg", args, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if payload[0] != ProtocolVersion || payload[49] != programFFmpeg {
		t.Fatalf("unexpected command prefix: %x", payload[:50])
	}
	wantSignature, _ := hex.DecodeString("23778eb96aa4fe7c022bc1afa6c0571a399a177d3b20746964e0b8b8a93b6fa9")
	if !bytes.Equal(payload[17:49], wantSignature) {
		t.Fatalf("signature = %x", payload[17:49])
	}
	if got := decodeArgsForTest(t, payload[50:]); !reflect.DeepEqual(got, args) {
		t.Fatalf("args = %#v, want %#v", got, args)
	}
	if err := ValidateInvocation("sh", nil); err == nil {
		t.Fatal("unsupported program accepted")
	}
	if err := ValidateInvocation("ffmpeg", []string{strings.Repeat("x", 1<<16)}); err == nil {
		t.Fatal("oversized argument accepted")
	}
	if err := ValidateInvocation("ffmpeg", make([]string, 1<<16)); err == nil {
		t.Fatal("oversized argument count accepted")
	}
}

func TestFrameLimitIsCheckedBeforePayloadRead(t *testing.T) {
	var header [5]byte
	header[0] = msgStdout
	binary.BigEndian.PutUint32(header[1:], MaxPayloadLen+1)
	if _, err := readFrame(bytes.NewReader(header[:])); err == nil {
		t.Fatal("oversized frame accepted")
	}
}

func TestSessionStreamsInWireOrderAndForwardsStdin(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	serverErr := make(chan error, 1)
	go func() {
		defer serverConn.Close()
		command, err := readFrame(serverConn)
		if err != nil || command.typ != msgCommand {
			serverErr <- errors.New("missing command")
			return
		}
		var stdin bytes.Buffer
		for {
			message, err := readFrame(serverConn)
			if err != nil {
				serverErr <- err
				return
			}
			if message.typ == msgStdin {
				stdin.Write(message.payload)
				continue
			}
			if message.typ != msgStdinClose {
				serverErr <- errors.New("missing stdin close")
				return
			}
			break
		}
		if stdin.String() != "input bytes" {
			serverErr <- errors.New("stdin mismatch")
			return
		}
		for _, message := range []frame{
			{typ: msgStdout, payload: []byte("one")},
			{typ: msgStderr, payload: []byte("two")},
			{typ: msgStdout, payload: []byte("three")},
			{typ: msgPing, payload: []byte("echo-me")},
		} {
			if err := writeFrame(serverConn, message.typ, message.payload); err != nil {
				serverErr <- err
				return
			}
		}
		pong, err := readFrame(serverConn)
		if err != nil || pong.typ != msgPong || string(pong.payload) != "echo-me" {
			serverErr <- errors.New("ping payload was not echoed")
			return
		}
		exit := make([]byte, 4)
		if err := writeFrame(serverConn, msgExitCode, exit); err != nil {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	var events []string
	code, err := runConn(context.Background(), clientConn, "test-secret", Invocation{
		Program: "ffmpeg",
		Args:    []string{"-i", "name with spaces", "pipe:1"},
		Stdin:   strings.NewReader("input bytes"),
	}, func(channel string, data []byte) {
		events = append(events, channel+":"+string(data))
	})
	if err != nil || code != 0 {
		t.Fatalf("runConn = %d, %v", code, err)
	}
	if want := []string{"stdout:one", "stderr:two", "stdout:three"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestCancellationSendsOneCancel(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	observed := make(chan int, 1)
	go func() {
		defer serverConn.Close()
		count := 0
		_, _ = readFrame(serverConn)
		for count == 0 {
			message, err := readFrame(serverConn)
			if err != nil {
				observed <- count
				return
			}
			if message.typ == msgCancel {
				count++
			} else if message.typ == msgStdinClose {
				close(ready)
			}
		}
		exit := make([]byte, 4)
		_ = writeFrame(serverConn, msgExitCode, exit)
		observed <- count
	}()
	result := make(chan error, 1)
	go func() {
		_, err := runConn(ctx, clientConn, "secret", Invocation{Program: "ffprobe", Args: []string{}}, nil)
		result <- err
	}()
	<-ready
	cancel()
	cancel()
	err := <-result
	var sessionErr *Error
	if !errors.As(err, &sessionErr) || sessionErr.Kind != "canceled" || sessionErr.ExitCode == nil || *sessionErr.ExitCode != 0 {
		t.Fatalf("error = %#v", err)
	}
	if count := <-observed; count != 1 {
		t.Fatalf("cancel count = %d", count)
	}
}

func TestServerErrorAndDisconnectAreSafe(t *testing.T) {
	secret := "never-return-this-secret"
	argument := "never-return-this-argument"
	for _, testCase := range []struct {
		name     string
		response func(net.Conn)
		kind     string
	}{
		{
			name: "server error",
			response: func(conn net.Conn) {
				_ = writeFrame(conn, msgError, []byte(secret+argument))
			},
			kind: "server",
		},
		{
			name:     "disconnect",
			response: func(net.Conn) {},
			kind:     "transport",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			go func() {
				defer serverConn.Close()
				_, _ = readFrame(serverConn)
				_, _ = readFrame(serverConn)
				testCase.response(serverConn)
			}()
			_, err := runConn(context.Background(), clientConn, secret, Invocation{Program: "ffmpeg", Args: []string{argument}}, nil)
			var sessionErr *Error
			if !errors.As(err, &sessionErr) || sessionErr.Kind != testCase.kind {
				t.Fatalf("error = %#v", err)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), argument) {
				t.Fatalf("sensitive value leaked in %q", err)
			}
		})
	}
}

func TestNonzeroExitAndMalformedFrameAreDeterministic(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		response func(net.Conn)
		kind     string
		code     uint32
	}{
		{
			name: "nonzero exit",
			response: func(conn net.Conn) {
				payload := make([]byte, 4)
				binary.BigEndian.PutUint32(payload, 23)
				_ = writeFrame(conn, msgExitCode, payload)
			},
			kind: "exit",
			code: 23,
		},
		{
			name: "maximum wire exit code",
			response: func(conn net.Conn) {
				payload := make([]byte, 4)
				binary.BigEndian.PutUint32(payload, ^uint32(0))
				_ = writeFrame(conn, msgExitCode, payload)
			},
			kind: "exit",
			code: ^uint32(0),
		},
		{
			name: "malformed exit",
			response: func(conn net.Conn) {
				_ = writeFrame(conn, msgExitCode, []byte{0})
			},
			kind: "protocol",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			go func() {
				defer serverConn.Close()
				_, _ = readFrame(serverConn)
				_, _ = readFrame(serverConn)
				testCase.response(serverConn)
			}()
			code, err := runConn(context.Background(), clientConn, "secret", Invocation{Program: "ffmpeg", Args: []string{}}, nil)
			var sessionErr *Error
			if !errors.As(err, &sessionErr) || sessionErr.Kind != testCase.kind || code != int(testCase.code) {
				t.Fatalf("runConn = %d, %#v", code, err)
			}
		})
	}
}

func TestConcurrentSessionsRemainIsolated(t *testing.T) {
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			clientConn, serverConn := net.Pipe()
			go func() {
				defer serverConn.Close()
				_, _ = readFrame(serverConn)
				_, _ = readFrame(serverConn)
				_ = writeFrame(serverConn, msgStdout, []byte{byte(index)})
				_ = writeFrame(serverConn, msgExitCode, make([]byte, 4))
			}()
			var got byte
			_, err := runConn(context.Background(), clientConn, "secret", Invocation{Program: "ffmpeg", Args: []string{}}, func(_ string, data []byte) {
				got = data[0]
			})
			if err != nil || got != byte(index) {
				t.Errorf("session %d: output=%d error=%v", index, got, err)
			}
		}()
	}
	wait.Wait()
}

func TestLivenessBoundaries(t *testing.T) {
	now := time.Unix(1000, 0)
	ping, timeout := livenessAction(now, now.Add(-keepaliveSendAfter), now.Add(-keepaliveRecvAfter+time.Nanosecond))
	if !ping || timeout {
		t.Fatalf("at send boundary: ping=%v timeout=%v", ping, timeout)
	}
	_, timeout = livenessAction(now, now, now.Add(-keepaliveRecvAfter))
	if !timeout {
		t.Fatal("receive timeout boundary was not detected")
	}
}

func TestLivenessSendsPingWhileInboundTrafficContinues(t *testing.T) {
	now := time.Now()
	for elapsed := livenessPollInterval; elapsed <= keepaliveSendAfter; elapsed += livenessPollInterval {
		// Model a fresh inbound output frame before every poll. It must not
		// reset the independently tracked outbound-idle interval.
		ping, timeout := livenessAction(now.Add(elapsed), now, now.Add(elapsed))
		if timeout || ping != (elapsed == keepaliveSendAfter) {
			t.Fatalf("after %s: ping=%v timeout=%v", elapsed, ping, timeout)
		}
	}
}

func TestLivenessReceiveTimeoutClosesSession(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	writer := newSerializedWriter(clientConn)
	now := time.Now()
	var lastRecv atomic.Int64
	lastRecv.Store(now.Add(-keepaliveRecvAfter).UnixNano())
	ticks := make(chan time.Time, 1)
	done := make(chan struct{})
	failures := make(chan error, 1)
	go monitorLiveness(context.Background(), done, ticks, writer, &lastRecv, clientConn, failures)
	ticks <- now
	err := <-failures
	var sessionErr *Error
	if !errors.As(err, &sessionErr) || sessionErr.Kind != "timeout" {
		t.Fatalf("timeout error = %#v", err)
	}
	if _, err := serverConn.Write([]byte("closed")); err == nil {
		t.Fatal("timed-out connection remained open")
	}
}

func TestFileOperationsAndReadCap(t *testing.T) {
	var ready []string
	handler := newFileHandler(func(path string) { ready = append(ready, path) }, 0)
	defer handler.closeAll()
	directory := t.TempDir()
	path := filepath.Join(directory, "source")
	renamed := filepath.Join(directory, "renamed")
	var response frame
	send := func(typ uint8, payload []byte) error {
		response = frame{typ: typ, payload: append([]byte(nil), payload...)}
		return nil
	}

	open := make([]byte, 10+len(path))
	binary.BigEndian.PutUint16(open, 1)
	binary.BigEndian.PutUint16(open[2:], 7)
	binary.BigEndian.PutUint32(open[4:], 0x42)
	binary.BigEndian.PutUint16(open[8:], 0o600)
	copy(open[10:], path)
	if err := handler.handle(msgOpen, open, send); err != nil || response.typ != msgOpenOK {
		t.Fatalf("open: type=%x err=%v", response.typ, err)
	}

	write := append([]byte{0, 2, 0, 7}, []byte("abcdef")...)
	if err := handler.handle(msgWrite, write, send); err != nil || response.typ != msgWriteOK {
		t.Fatalf("write: type=%x err=%v", response.typ, err)
	}
	seek := make([]byte, 13)
	binary.BigEndian.PutUint16(seek, 3)
	binary.BigEndian.PutUint16(seek[2:], 7)
	seek[12] = 0
	if err := handler.handle(msgSeek, seek, send); err != nil || response.typ != msgSeekOK {
		t.Fatalf("seek: type=%x err=%v", response.typ, err)
	}
	read := make([]byte, 8)
	binary.BigEndian.PutUint16(read, 4)
	binary.BigEndian.PutUint16(read[2:], 7)
	binary.BigEndian.PutUint32(read[4:], 6)
	if err := handler.handle(msgRead, read, send); err != nil || response.typ != msgReadOK || string(response.payload[2:]) != "abcdef" {
		t.Fatalf("read: type=%x payload=%q err=%v", response.typ, response.payload, err)
	}
	fstat := []byte{0, 5, 0, 7}
	if err := handler.handle(msgFstat, fstat, send); err != nil || response.typ != msgFstatOK {
		t.Fatalf("fstat: type=%x err=%v", response.typ, err)
	}
	truncate := make([]byte, 12)
	binary.BigEndian.PutUint16(truncate, 6)
	binary.BigEndian.PutUint16(truncate[2:], 7)
	binary.BigEndian.PutUint64(truncate[4:], 2)
	if err := handler.handle(msgFtruncate, truncate, send); err != nil || response.typ != msgFtruncateOK {
		t.Fatalf("truncate: type=%x err=%v", response.typ, err)
	}
	if err := handler.handle(msgClose, []byte{0, 7, 0, 7}, send); err != nil || response.typ != msgCloseOK {
		t.Fatalf("close: type=%x err=%v", response.typ, err)
	}
	if !reflect.DeepEqual(ready, []string{path}) {
		t.Fatalf("ready after close = %#v", ready)
	}

	rename := make([]byte, 4+len(path)+len(renamed))
	binary.BigEndian.PutUint16(rename, 8)
	binary.BigEndian.PutUint16(rename[2:], uint16(len(path)))
	copy(rename[4:], path)
	copy(rename[4+len(path):], renamed)
	if err := handler.handle(msgRename, rename, send); err != nil || response.typ != msgRenameOK {
		t.Fatalf("rename: type=%x err=%v", response.typ, err)
	}
	if !reflect.DeepEqual(ready, []string{path, renamed}) {
		t.Fatalf("ready after rename = %#v", ready)
	}
	if info, err := os.Stat(renamed); err != nil || info.Size() != 2 {
		t.Fatalf("renamed file: info=%v err=%v", info, err)
	}
	unlink := append([]byte{0, 9}, []byte(renamed)...)
	if err := handler.handle(msgUnlink, unlink, send); err != nil || response.typ != msgUnlinkOK {
		t.Fatalf("unlink: type=%x err=%v", response.typ, err)
	}
	subdir := filepath.Join(directory, "made")
	mkdir := make([]byte, 4+len(subdir))
	binary.BigEndian.PutUint16(mkdir, 10)
	binary.BigEndian.PutUint16(mkdir[2:], 0o700)
	copy(mkdir[4:], subdir)
	if err := handler.handle(msgMkdir, mkdir, send); err != nil || response.typ != msgMkdirOK {
		t.Fatalf("mkdir: type=%x err=%v", response.typ, err)
	}

	handler.readBuf = nil
	oversized := make([]byte, 8)
	binary.BigEndian.PutUint16(oversized, 11)
	binary.BigEndian.PutUint32(oversized[4:], MaxFileReadLen+1)
	if err := handler.handle(msgRead, oversized, send); err != nil || response.typ != msgIOError || binary.BigEndian.Uint32(response.payload[2:]) != uint32(fioERANGE) {
		t.Fatalf("oversized read: type=%x payload=%x err=%v", response.typ, response.payload, err)
	}
	if cap(handler.readBuf) != 0 {
		t.Fatalf("oversized read allocated %d bytes", cap(handler.readBuf))
	}
}

func TestFileOutputLimitCountsExactAggregateAcrossFiles(t *testing.T) {
	if err := os.MkdirAll("tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp("tmp", "output-limit-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })

	handler := newFileHandler(nil, 5)
	defer handler.closeAll()
	var response frame
	send := func(typ uint8, payload []byte) error {
		response = frame{typ: typ, payload: append([]byte(nil), payload...)}
		return nil
	}
	open := func(requestID, fileID uint16, path string) {
		t.Helper()
		payload := make([]byte, 10+len(path))
		binary.BigEndian.PutUint16(payload, requestID)
		binary.BigEndian.PutUint16(payload[2:], fileID)
		binary.BigEndian.PutUint32(payload[4:], 0x241)
		binary.BigEndian.PutUint16(payload[8:], 0o600)
		copy(payload[10:], path)
		if err := handler.handle(msgOpen, payload, send); err != nil || response.typ != msgOpenOK {
			t.Fatalf("open: type=%x err=%v", response.typ, err)
		}
	}
	write := func(requestID, fileID uint16, data string) error {
		t.Helper()
		payload := make([]byte, 4+len(data))
		binary.BigEndian.PutUint16(payload, requestID)
		binary.BigEndian.PutUint16(payload[2:], fileID)
		copy(payload[4:], data)
		return handler.handle(msgWrite, payload, send)
	}

	first := filepath.Join(directory, "first")
	second := filepath.Join(directory, "second")
	open(1, 7, first)
	open(2, 8, second)
	if err := write(3, 7, "abc"); err != nil || response.typ != msgWriteOK {
		t.Fatalf("first write: type=%x err=%v", response.typ, err)
	}
	if err := write(4, 8, "de"); err != nil || response.typ != msgWriteOK || handler.writtenBytes != 5 {
		t.Fatalf("exact aggregate limit: type=%x bytes=%d err=%v", response.typ, handler.writtenBytes, err)
	}
	if err := write(5, 7, "x"); !errors.Is(err, errOutputLimitExceeded) || response.typ != msgIOError || binary.BigEndian.Uint32(response.payload[2:]) != uint32(fioENOSPC) {
		t.Fatalf("over-limit write: type=%x payload=%x err=%v", response.typ, response.payload, err)
	}
	truncate := make([]byte, 12)
	binary.BigEndian.PutUint16(truncate, 6)
	binary.BigEndian.PutUint16(truncate[2:], 8)
	binary.BigEndian.PutUint64(truncate[4:], 3)
	if err := handler.handle(msgFtruncate, truncate, send); !errors.Is(err, errOutputLimitExceeded) || response.typ != msgIOError {
		t.Fatalf("over-limit truncate: type=%x payload=%x err=%v", response.typ, response.payload, err)
	}
	if handler.writtenBytes != 5 {
		t.Fatalf("accepted bytes = %d", handler.writtenBytes)
	}
	for path, want := range map[string]int64{first: 3, second: 2} {
		if info, err := os.Stat(path); err != nil || info.Size() != want {
			t.Fatalf("%s: size=%v err=%v", path, info, err)
		}
	}
}

func TestOutputLimitCancelsRemoteInvocation(t *testing.T) {
	if err := os.MkdirAll("tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp("tmp", "output-cancel-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	path := filepath.Join(directory, "output")

	clientConn, serverConn := net.Pipe()
	serverErr := make(chan error, 1)
	go func() {
		defer serverConn.Close()
		if command, err := readFrame(serverConn); err != nil || command.typ != msgCommand {
			serverErr <- errors.New("missing command")
			return
		}
		if eof, err := readFrame(serverConn); err != nil || eof.typ != msgStdinClose {
			serverErr <- errors.New("missing stdin close")
			return
		}
		open := make([]byte, 10+len(path))
		binary.BigEndian.PutUint16(open, 1)
		binary.BigEndian.PutUint16(open[2:], 7)
		binary.BigEndian.PutUint32(open[4:], 0x241)
		binary.BigEndian.PutUint16(open[8:], 0o600)
		copy(open[10:], path)
		if err := writeFrame(serverConn, msgOpen, open); err != nil {
			serverErr <- err
			return
		}
		if response, err := readFrame(serverConn); err != nil || response.typ != msgOpenOK {
			serverErr <- errors.New("file open failed")
			return
		}
		write := append([]byte{0, 2, 0, 7}, []byte("123456")...)
		if err := writeFrame(serverConn, msgWrite, write); err != nil {
			serverErr <- err
			return
		}
		response, err := readFrame(serverConn)
		if err != nil || response.typ != msgIOError || binary.BigEndian.Uint32(response.payload[2:]) != uint32(fioENOSPC) {
			serverErr <- errors.New("output limit was not rejected")
			return
		}
		if cancel, err := readFrame(serverConn); err != nil || cancel.typ != msgCancel {
			serverErr <- errors.New("remote invocation was not canceled")
			return
		}
		serverErr <- nil
	}()

	_, err = runConn(context.Background(), clientConn, "secret", Invocation{
		Program:        "ffmpeg",
		Args:           []string{},
		MaxOutputBytes: 5,
	}, nil)
	var sessionErr *Error
	if !errors.As(err, &sessionErr) || sessionErr.Kind != "output_limit" {
		t.Fatalf("run error = %#v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != 0 {
		t.Fatalf("over-limit bytes were accepted: info=%v err=%v", info, err)
	}
}

func decodeArgsForTest(t *testing.T, payload []byte) []string {
	t.Helper()
	count := int(binary.BigEndian.Uint16(payload))
	offset := 2
	args := make([]string, 0, count)
	for range count {
		length := int(binary.BigEndian.Uint16(payload[offset:]))
		offset += 2
		args = append(args, string(payload[offset:offset+length]))
		offset += length
	}
	return args
}
