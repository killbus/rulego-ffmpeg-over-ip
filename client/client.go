// Package client implements the ffmpeg-over-ip v5.2.1 protocol-v6 client.
package client

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	stdinChunkSize       = 32 * 1024
	livenessPollInterval = 5 * time.Second
	keepaliveSendAfter   = 30 * time.Second
	keepaliveRecvAfter   = 150 * time.Second
	cancelGrace          = 5 * time.Second
)

type Config struct {
	Address     string
	AuthSecret  string
	DialTimeout time.Duration
}

type Invocation struct {
	Program     string
	Args        []string
	Stdin       io.Reader
	OnFileReady func(string)
}

type OutputFunc func(channel string, data []byte)

// Error is deliberately safe to return through RuleGo: it never contains the
// authentication secret, argv, server payload, or filesystem paths.
type Error struct {
	Kind     string
	Message  string
	ExitCode *int
}

func (e *Error) Error() string { return e.Message }

func safeError(kind, message string) *Error { return &Error{Kind: kind, Message: message} }

// Run dials one TCP or Unix socket and owns it for exactly one invocation.
func Run(ctx context.Context, config Config, invocation Invocation, output OutputFunc) (int, error) {
	if err := ValidateInvocation(invocation.Program, invocation.Args); err != nil {
		return 0, safeError("invalid_input", err.Error())
	}
	network, address, err := parseAddress(config.Address)
	if err != nil || config.AuthSecret == "" {
		return 0, safeError("configuration", "invalid client configuration")
	}
	dialer := net.Dialer{Timeout: config.DialTimeout}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		if ctx.Err() != nil {
			return 0, contextError(ctx.Err())
		}
		return 0, safeError("transport", "remote connection failed")
	}
	return runConn(ctx, conn, config.AuthSecret, invocation, output)
}

func parseAddress(address string) (string, string, error) {
	if strings.HasPrefix(address, "unix:") {
		if len(address) == len("unix:") {
			return "", "", errors.New("empty Unix socket path")
		}
		return "unix", strings.TrimPrefix(address, "unix:"), nil
	}
	if address == "" {
		return "", "", errors.New("empty address")
	}
	return "tcp", address, nil
}

type serializedWriter struct {
	mu       sync.Mutex
	conn     net.Conn
	lastSend atomic.Int64
}

func newSerializedWriter(conn net.Conn) *serializedWriter {
	w := &serializedWriter{conn: conn}
	w.lastSend.Store(time.Now().UnixNano())
	return w
}

func (w *serializedWriter) write(typ uint8, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := writeFrame(w.conn, typ, payload); err != nil {
		return err
	}
	w.lastSend.Store(time.Now().UnixNano())
	return nil
}

func (w *serializedWriter) cancel() error {
	return w.write(msgCancel, nil)
}

func runConn(ctx context.Context, conn net.Conn, secret string, invocation Invocation, output OutputFunc) (exitCode int, result error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ValidateInvocation(invocation.Program, invocation.Args); err != nil {
		_ = conn.Close()
		return 0, safeError("invalid_input", err.Error())
	}

	var nonce [nonceLen]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		_ = conn.Close()
		return 0, safeError("internal", "could not create command nonce")
	}
	payload, _ := commandPayload(secret, invocation.Program, invocation.Args, nonce)
	w := newSerializedWriter(conn)
	if err := w.write(msgCommand, payload); err != nil {
		_ = conn.Close()
		return 0, safeError("transport", "remote command send failed")
	}

	files := newFileHandler(invocation.OnFileReady)
	var lastRecv atomic.Int64
	lastRecv.Store(time.Now().UnixNano())
	asyncErr := make(chan error, 1)
	stdinDone := make(chan struct{})
	go sendStdin(invocation.Stdin, w, asyncErr, stdinDone)

	done := make(chan struct{})
	var cancelOnce sync.Once
	var finished atomic.Bool
	ticker := time.NewTicker(livenessPollInterval)
	defer ticker.Stop()
	go monitorLiveness(ctx, done, ticker.C, w, &lastRecv, conn, asyncErr)
	go func() {
		select {
		case <-ctx.Done():
			if finished.Load() {
				return
			}
			deadline := time.Now().Add(cancelGrace)
			_ = conn.SetWriteDeadline(deadline)
			cancelOnce.Do(func() { _ = w.cancel() })
			remaining := time.Until(deadline)
			if remaining < 0 {
				remaining = 0
			}
			select {
			case <-done:
			case <-time.After(remaining):
				_ = conn.Close()
			}
		case <-done:
		}
	}()

	defer func() {
		finished.Store(true)
		close(done)
		_ = conn.Close()
		<-stdinDone
		files.closeAll()
	}()

	for {
		message, err := readFrame(conn)
		if err != nil {
			select {
			case failure := <-asyncErr:
				return 0, failure
			default:
			}
			if ctx.Err() != nil {
				return 0, contextError(ctx.Err())
			}
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return 0, safeError("transport", "remote session disconnected")
			}
			return 0, safeError("protocol", "invalid remote protocol frame")
		}
		lastRecv.Store(time.Now().UnixNano())

		switch message.typ {
		case msgStdout:
			if output != nil {
				output("stdout", message.payload)
			}
		case msgStderr:
			if output != nil {
				output("stderr", message.payload)
			}
		case msgExitCode:
			if len(message.payload) != 4 {
				return 0, safeError("protocol", "invalid exit frame")
			}
			code := int(binary.BigEndian.Uint32(message.payload))
			if ctx.Err() != nil {
				failure := contextError(ctx.Err())
				failure.ExitCode = &code
				return code, failure
			}
			if code != 0 {
				return code, &Error{Kind: "exit", Message: "remote process failed", ExitCode: &code}
			}
			return code, nil
		case msgError:
			return 0, safeError("server", "remote server rejected invocation")
		case msgPing:
			if err := w.write(msgPong, message.payload); err != nil {
				return 0, safeError("transport", "remote pong send failed")
			}
		case msgPong:
		case msgOpen, msgRead, msgWrite, msgSeek, msgClose, msgFstat, msgFtruncate, msgUnlink, msgRename, msgMkdir:
			if err := files.handle(message.typ, message.payload, w.write); err != nil {
				return 0, safeError("protocol", "invalid file operation frame")
			}
		default:
			return 0, safeError("protocol", "unsupported remote protocol frame")
		}
	}
}

func monitorLiveness(ctx context.Context, done <-chan struct{}, ticks <-chan time.Time, writer *serializedWriter, lastRecv *atomic.Int64, conn net.Conn, failures chan<- error) {
	for {
		select {
		case now := <-ticks:
			ping, timedOut := livenessAction(now, time.Unix(0, writer.lastSend.Load()), time.Unix(0, lastRecv.Load()))
			if timedOut {
				reportAsyncError(failures, safeError("timeout", "remote session receive timeout"))
				_ = conn.Close()
				return
			}
			if ping && writer.write(msgPing, nil) != nil {
				reportAsyncError(failures, safeError("transport", "remote keepalive failed"))
				_ = conn.Close()
				return
			}
		case <-ctx.Done():
			return
		case <-done:
			return
		}
	}
}

func reportAsyncError(failures chan<- error, err error) {
	select {
	case failures <- err:
	default:
	}
}

func sendStdin(reader io.Reader, writer *serializedWriter, failures chan<- error, done chan<- struct{}) {
	defer close(done)
	if reader != nil {
		buffer := make([]byte, stdinChunkSize)
		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				if writeErr := writer.write(msgStdin, buffer[:n]); writeErr != nil {
					reportAsyncError(failures, safeError("transport", "stdin forwarding failed"))
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					reportAsyncError(failures, safeError("transport", "stdin forwarding failed"))
					return
				}
				break
			}
			if n == 0 {
				reportAsyncError(failures, safeError("transport", "stdin forwarding failed"))
				return
			}
		}
	}
	if err := writer.write(msgStdinClose, nil); err != nil {
		reportAsyncError(failures, safeError("transport", "stdin forwarding failed"))
	}
}

func livenessAction(now, lastSend, lastRecv time.Time) (ping, timedOut bool) {
	return now.Sub(lastSend) >= keepaliveSendAfter, now.Sub(lastRecv) >= keepaliveRecvAfter
}

func contextError(err error) *Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return safeError("timeout", "remote session timed out")
	}
	return safeError("canceled", "remote session canceled")
}
