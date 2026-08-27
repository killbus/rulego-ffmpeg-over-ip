package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
)

func TestInvocationBoundary(t *testing.T) {
	request, stdin, err := decodeInvocation(`{"program":"ffmpeg","args":["-i","a b!?","pipe:1"],"stdinBase64":"AAEC/w=="}`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"-i", "a b!?", "pipe:1"}; !reflect.DeepEqual(request.Args, want) {
		t.Fatalf("args = %#v", request.Args)
	}
	decoded, err := io.ReadAll(stdin)
	if err != nil || !reflect.DeepEqual(decoded, []byte{0, 1, 2, 255}) {
		t.Fatalf("stdin = %v, %v", decoded, err)
	}
	for _, invalid := range []string{
		`{"program":"bash","args":[]}`,
		`{"program":"ffmpeg"}`,
		`{"program":"ffmpeg","args":[],"extra":true}`,
		`{"program":"ffmpeg","args":[],"stdinBase64":"%%%"}`,
		`{"program":"ffmpeg","args":[],"session":{}}`,
		`{"program":"ffmpeg","args":[]} {}`,
	} {
		if _, _, err := decodeInvocation(invalid); err == nil {
			t.Fatalf("accepted invalid invocation %s", invalid)
		}
	}
}

func TestPluginAndConfiguration(t *testing.T) {
	components := Plugins.Components()
	if len(components) != 2 || components[0].Type() != componentType || components[1].Type() != producerComponentType {
		t.Fatalf("components = %#v", components)
	}
	definition := components[0].(*ffmpegOverIPNode).Def()
	if definition.RelationTypes == nil || !reflect.DeepEqual(*definition.RelationTypes, []string{types.Stream, types.Success, types.Failure}) {
		t.Fatalf("relations = %#v", definition.RelationTypes)
	}
	node := (&ffmpegOverIPNode{}).New().(*ffmpegOverIPNode)
	if err := node.Init(types.Config{}, types.Configuration{"address": "unix:tmp/ffmpeg.sock", "authSecret": "secret"}); err != nil {
		t.Fatal(err)
	}
	if node.Config.DialTimeoutMs != 5000 {
		t.Fatalf("default dial timeout = %d", node.Config.DialTimeoutMs)
	}
	if err := node.Init(types.Config{}, types.Configuration{"address": "localhost:5050"}); err == nil {
		t.Fatal("missing secret accepted")
	}
	if err := node.Init(types.Config{}, types.Configuration{"address": "localhost:5050", "authSecret": "secret", "sessionTimeoutMs": int64(1<<63 - 1)}); err == nil {
		t.Fatal("overflowing timeout accepted")
	}
	producer := (&ffmpegOverIPProducerNode{}).New().(*ffmpegOverIPProducerNode)
	if err := producer.Init(types.Config{}, types.Configuration{"address": "unix:tmp/ffmpeg.sock", "authSecret": "secret"}); err != nil {
		t.Fatal(err)
	}
	if producer.Config.DialTimeoutMs != 5000 {
		t.Fatalf("producer default dial timeout = %d", producer.Config.DialTimeoutMs)
	}
}

func TestProducerBoundary(t *testing.T) {
	valid := `{"key":"asset","invocation":{"program":"ffmpeg","args":[]},"awaitFile":"segment.ts","awaitOnly":true,"runTimeoutMs":1,"cacheTtlMs":1}`
	request, _, err := decodeProducerRequest(valid)
	if err != nil || request.AwaitFile != "segment.ts" || !request.AwaitOnly {
		t.Fatalf("valid producer request: request=%#v err=%v", request, err)
	}
	for _, invalid := range []string{
		`{"key":"","invocation":{"program":"ffmpeg","args":[]},"awaitFile":"segment.ts","runTimeoutMs":1,"cacheTtlMs":1}`,
		`{"key":"asset","invocation":{"program":"ffmpeg","args":[]},"awaitFile":"","runTimeoutMs":1,"cacheTtlMs":1}`,
		`{"key":"asset","invocation":{"program":"ffmpeg","args":[]},"awaitFile":"segment.ts","runTimeoutMs":0,"cacheTtlMs":1}`,
		`{"key":"asset","invocation":{"program":"ffmpeg","args":[]},"awaitFile":"segment.ts","runTimeoutMs":1,"cacheTtlMs":0}`,
	} {
		if _, _, err := decodeProducerRequest(invalid); err == nil {
			t.Fatalf("accepted invalid producer request %s", invalid)
		}
	}
}

func TestProducerDoesNotPublishFileClosedAfterCancellation(t *testing.T) {
	node := &ffmpegOverIPProducerNode{}
	job := &producerJob{
		notify: make(chan struct{}),
		ready:  make(map[string]struct{}),
		files:  make(map[string]struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	node.markFileReady(ctx, job, "segment.ts")

	if node.jobFileReady(job, "segment.ts") {
		t.Fatal("file closed after cancellation was published as ready")
	}
	if _, ok := job.files["segment.ts"]; !ok {
		t.Fatal("canceled output was not retained for cleanup")
	}
}

func TestResponseProcessorWritesOnlyStdoutAndFailuresAreBodyless(t *testing.T) {
	stdout := types.NewMsgFromBytes(0, "", types.BINARY, types.NewMetadata(), []byte{0, 1, 2})
	stdout.Metadata.PutValue(channelKey, "stdout")
	out := newTestMessage(&stdout)
	exchange := &endpoint.Exchange{Out: out}
	if !projectResponse(nil, exchange) || !reflect.DeepEqual(out.body, []byte{0, 1, 2}) || out.flushes != 1 {
		t.Fatalf("stdout projection: body=%v flushes=%d", out.body, out.flushes)
	}

	stderr := types.NewMsgFromBytes(0, "", types.BINARY, types.NewMetadata(), []byte("diagnostic"))
	stderr.Metadata.PutValue(channelKey, "stderr")
	out = newTestMessage(&stderr)
	projectResponse(nil, &endpoint.Exchange{Out: out})
	if len(out.body) != 0 || out.flushes != 0 {
		t.Fatalf("stderr entered response: body=%q flushes=%d", out.body, out.flushes)
	}

	terminal := types.NewMsgWithJsonData(`{"program":"ffmpeg","exitCode":1}`)
	out = newTestMessage(&terminal)
	out.err = errors.New("remote process failed")
	projectResponse(nil, &endpoint.Exchange{Out: out})
	if len(out.body) != 0 || out.status != http.StatusBadGateway {
		t.Fatalf("failure projection: body=%q status=%d", out.body, out.status)
	}
}

func TestRuleGoStreamsBothChannelsBeforeOneTerminalResult(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		if _, err := readFixtureFrame(conn); err != nil {
			serverErr <- err
			return
		}
		if message, err := readFixtureFrame(conn); err != nil || message.typ != 0x11 {
			serverErr <- errors.New("stdin EOF not received")
			return
		}
		for _, message := range []fixtureFrame{
			{typ: 0x12, payload: []byte("out")},
			{typ: 0x13, payload: []byte("err")},
			{typ: 0x03, payload: make([]byte, 4)},
		} {
			if err := writeFixtureFrame(conn, message.typ, message.payload); err != nil {
				serverErr <- err
				return
			}
		}
		serverErr <- nil
	}()

	if err := rulego.Registry.Register(&ffmpegOverIPNode{}); err != nil {
		t.Fatal(err)
	}
	defer rulego.Registry.Unregister(componentType)
	dsl := fmt.Sprintf(`{
		"ruleChain":{"id":"ffoip-test","root":true},
		"metadata":{
			"firstNodeIndex":0,
			"nodes":[
				{"id":"client","type":"ffmpegOverIp","configuration":{"address":%q,"authSecret":"secret"}},
				{"id":"end","type":"end","configuration":{}}
			],
			"connections":[
				{"fromId":"client","toId":"end","type":"Stream"},
				{"fromId":"client","toId":"end","type":"Success"},
				{"fromId":"client","toId":"end","type":"Failure"}
			]
		}
	}`, listener.Addr().String())
	pool := rulego.NewRuleGo()
	engine, err := pool.New("ffoip-test", []byte(dsl), rulego.WithConfig(rulego.NewConfig()))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Stop(nil)

	var mu sync.Mutex
	var events []string
	msg := types.NewMsgWithJsonData(`{"program":"ffmpeg","args":["-i","a b!?","pipe:1"]}`)
	msg.Metadata.PutValue("trace-id", "preserved")
	msg.Metadata.PutValue(channelKey, "stdout")
	terminalMetadataOK := false
	engine.OnMsgAndWait(msg, types.WithOnEnd(func(_ types.RuleContext, output types.RuleMsg, callbackErr error, relation string) {
		mu.Lock()
		defer mu.Unlock()
		if callbackErr != nil {
			events = append(events, relation+":failure")
			return
		}
		if relation == types.Stream {
			events = append(events, output.Metadata.GetValue(channelKey)+":"+output.GetData())
			return
		}
		terminalMetadataOK = output.Metadata.GetValue("trace-id") == "preserved" && output.Metadata.GetValue(channelKey) == ""
		events = append(events, relation)
	}))
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"stdout:out", "stderr:err", types.Success}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
	if !terminalMetadataOK {
		t.Fatal("terminal output did not preserve caller metadata or retained stream channel metadata")
	}
}

func TestProducerSharesJobAndExpiresFiles(t *testing.T) {
	if err := os.MkdirAll("tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp("tmp", "session-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	outputPath := filepath.Join(directory, "segment.ts")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	started := make(chan struct{})
	writeFile := make(chan struct{})
	release := make(chan struct{})
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		if _, err := readFixtureFrame(conn); err != nil {
			serverErr <- err
			return
		}
		if message, err := readFixtureFrame(conn); err != nil || message.typ != 0x11 {
			serverErr <- errors.New("stdin EOF not received")
			return
		}
		close(started)
		<-writeFile
		if err := writeFixtureFile(conn, outputPath, []byte("segment")); err != nil {
			serverErr <- err
			return
		}
		<-release
		serverErr <- writeFixtureFrame(conn, 0x03, make([]byte, 4))
	}()

	if err := rulego.Registry.Register(&ffmpegOverIPProducerNode{}); err != nil {
		t.Fatal(err)
	}
	defer rulego.Registry.Unregister(producerComponentType)
	dsl := fmt.Sprintf(`{
		"ruleChain":{"id":"ffoip-producer","root":true},
		"metadata":{"firstNodeIndex":0,"nodes":[
			{"id":"client","type":"ffmpegOverIpProducer","configuration":{"address":%q,"authSecret":"secret"}},
			{"id":"end","type":"end","configuration":{}}
		],"connections":[
			{"fromId":"client","toId":"end","type":"Stream"},
			{"fromId":"client","toId":"end","type":"Success"},
			{"fromId":"client","toId":"end","type":"Failure"}
		]}
	}`, listener.Addr().String())
	pool := rulego.NewRuleGo()
	engine, err := pool.New("ffoip-producer", []byte(dsl), rulego.WithConfig(rulego.NewConfig()))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Stop(nil)

	request := fmt.Sprintf(`{"key":"video:profile","invocation":{"program":"ffmpeg","args":["-window","0"]},"awaitFile":%q,"runTimeoutMs":1000,"cacheTtlMs":50}`, outputPath)
	awaitOnlyRequest := fmt.Sprintf(`{"key":"video:profile","invocation":{"program":"ffmpeg","args":["-window","0"]},"awaitFile":%q,"awaitOnly":true,"runTimeoutMs":1000,"cacheTtlMs":50}`, outputPath)
	run := func(request string, result chan<- string) {
		var body strings.Builder
		engine.OnMsgAndWait(types.NewMsgWithJsonData(request), types.WithOnEnd(func(_ types.RuleContext, output types.RuleMsg, callbackErr error, relation string) {
			if callbackErr == nil && relation == types.Stream && output.Metadata.GetValue(channelKey) == "stdout" {
				body.Write(output.GetBytes())
			}
		}))
		result <- body.String()
	}
	results := make(chan string, 2)
	go run(request, results)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("session did not start")
	}
	go run(awaitOnlyRequest, results)
	close(writeFile)
	seen := map[string]int{}
	for range 2 {
		select {
		case got := <-results:
			seen[got]++
		case <-time.After(time.Second):
			t.Fatal("session waiter did not finish")
		}
	}
	if seen["segment"] != 1 || seen[""] != 1 {
		t.Fatalf("producer outputs = %#v", seen)
	}

	if tcp, ok := listener.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(50 * time.Millisecond))
		if duplicate, acceptErr := tcp.Accept(); acceptErr == nil {
			_ = duplicate.Close()
			t.Fatal("duplicate session opened a second connection")
		}
	}
	close(release)
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(outputPath); errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("session cache file did not expire")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProducerReplacesDifferentJob(t *testing.T) {
	canceled := make(chan struct{})
	node := &ffmpegOverIPProducerNode{
		Config: producerConfiguration{Address: "127.0.0.1:1", AuthSecret: "secret", DialTimeoutMs: 1},
		jobs: map[string]*producerJob{
			"video:profile": {
				fingerprint: sha256.Sum256([]byte("old")),
				cancel:      func() { close(canceled) },
			},
		},
	}
	request := producerRequest{
		Key:          "video:profile",
		Invocation:   invocationRequest{Program: "ffmpeg", Args: []string{"-window", "next"}},
		AwaitFile:    "unused",
		RunTimeoutMs: 100,
		CacheTTLms:   1,
	}
	if _, ok := node.ensureJob(request, nil, invocationFingerprint(request.Invocation)); !ok {
		t.Fatal("replacement job was rejected")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("previous window was not canceled")
	}
	node.Destroy()
}

func TestProducerCancelsWhenLastWaiterDisconnects(t *testing.T) {
	canceled := make(chan struct{})
	node := &ffmpegOverIPProducerNode{}
	job := &producerJob{
		cancel: func() { close(canceled) },
		done:   make(chan struct{}),
		notify: make(chan struct{}),
		ready:  make(map[string]struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := node.waitForFile(ctx, job, "segment.ts"); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("producer was not canceled after its last waiter disconnected")
	}
}

func TestRuleGoCancellationSourcesSendOneRemoteCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ready := make(chan struct{})
	firstCancel := make(chan struct{})
	releaseRemote := make(chan struct{})
	cancelCount := make(chan int, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			cancelCount <- -1
			return
		}
		defer conn.Close()
		_, _ = readFixtureFrame(conn)
		count := 0
		for {
			message, err := readFixtureFrame(conn)
			if err != nil {
				cancelCount <- count
				return
			}
			if message.typ == 0x11 {
				close(ready)
			}
			if message.typ == 0x02 {
				count++
				if count == 1 {
					close(firstCancel)
					<-releaseRemote
					_ = writeFixtureFrame(conn, 0x03, make([]byte, 4))
				}
			}
		}
	}()

	if err := rulego.Registry.Register(&ffmpegOverIPNode{}); err != nil {
		t.Fatal(err)
	}
	defer rulego.Registry.Unregister(componentType)
	dsl := fmt.Sprintf(`{
		"ruleChain":{"id":"ffoip-cancel","root":true},
		"metadata":{"firstNodeIndex":0,"nodes":[
			{"id":"client","type":"ffmpegOverIp","configuration":{"address":%q,"authSecret":"secret","sessionTimeoutMs":100}},
			{"id":"end","type":"end","configuration":{}}
		],"connections":[{"fromId":"client","toId":"end","type":"Failure"}]}
	}`, listener.Addr().String())
	pool := rulego.NewRuleGo()
	engine, err := pool.New("ffoip-cancel", []byte(dsl), rulego.WithConfig(rulego.NewConfig()))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Stop(nil)

	requestContext, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	var terminals int
	go func() {
		engine.OnMsgAndWait(types.NewMsgWithJsonData(`{"program":"ffmpeg","args":[]}`),
			types.WithContext(requestContext),
			types.WithOnEnd(func(_ types.RuleContext, _ types.RuleMsg, _ error, relation string) {
				if relation == types.Failure {
					terminals++
				}
			}),
		)
		close(finished)
	}()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("session did not start")
	}
	select {
	case <-firstCancel:
	case <-time.After(time.Second):
		t.Fatal("session timeout did not send remote cancel")
	}
	cancel()
	stopStarted := make(chan struct{})
	stopDone := make(chan struct{})
	go func() {
		close(stopStarted)
		engine.Stop(context.Background())
		close(stopDone)
	}()
	<-stopStarted
	close(releaseRemote)
	select {
	case count := <-cancelCount:
		if count != 1 {
			t.Fatalf("cancel count = %d", count)
		}
	case <-time.After(time.Second):
		t.Fatal("remote cancel was not observed")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("RuleGo invocation did not finish")
	}
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("node destruction did not finish")
	}
	if terminals != 1 {
		t.Fatalf("terminal outcomes = %d", terminals)
	}
}

func TestRESTDisconnectCancelsRemoteSession(t *testing.T) {
	remote, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()

	started := make(chan struct{})
	canceled := make(chan struct{})
	go func() {
		conn, err := remote.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = readFixtureFrame(conn)
		for {
			message, err := readFixtureFrame(conn)
			if err != nil {
				return
			}
			switch message.typ {
			case 0x11:
				_ = writeFixtureFrame(conn, 0x12, []byte("chunk"))
				close(started)
			case 0x02:
				close(canceled)
				_ = writeFixtureFrame(conn, 0x03, make([]byte, 4))
				return
			}
		}
	}()

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpAddress := probe.Addr().String()
	_ = probe.Close()

	if err := rulego.Registry.Register(&ffmpegOverIPNode{}); err != nil {
		t.Fatal(err)
	}
	defer rulego.Registry.Unregister(componentType)
	chainID := "ffoip-rest-cancel"
	dsl := fmt.Sprintf(`{
		"ruleChain":{"id":%q,"root":true},
		"metadata":{
			"firstNodeIndex":0,
			"endpoints":[{
				"id":"http","type":"endpoint/http",
				"configuration":{"server":%q,"writeTimeout":60},
				"routers":[{"id":"invoke","params":["POST"],"from":{"path":"/ffmpeg","processors":["setBinaryDataType"]},"to":{"path":%q,"wait":true,"processors":["ffmpegOverIpResponse"]}}]
			}],
			"nodes":[
				{"id":"client","type":"ffmpegOverIp","configuration":{"address":%q,"authSecret":"secret"}},
				{"id":"end","type":"end","configuration":{}}
			],
			"connections":[
				{"fromId":"client","toId":"end","type":"Stream"},
				{"fromId":"client","toId":"end","type":"Success"},
				{"fromId":"client","toId":"end","type":"Failure"}
			]
		}
	}`, chainID, httpAddress, chainID+":client", remote.Addr().String())
	_, err = rulego.New(chainID, []byte(dsl), types.WithConfig(rulego.NewConfig(types.WithDefaultPool(), types.WithEndpointEnabled(true))))
	if err != nil {
		t.Fatal(err)
	}
	defer rulego.Del(chainID)

	deadline := time.Now().Add(time.Second)
	for {
		conn, dialErr := net.DialTimeout("tcp", httpAddress, 10*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("REST endpoint did not start: %v", dialErr)
		}
		time.Sleep(10 * time.Millisecond)
	}

	requestContext, cancel := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, "http://"+httpAddress+"/ffmpeg", strings.NewReader(`{"program":"ffmpeg","args":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 2 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	chunk := make([]byte, len("chunk"))
	if _, err := io.ReadFull(response.Body, chunk); err != nil || string(chunk) != "chunk" {
		t.Fatalf("progressive response = %q, %v", chunk, err)
	}
	select {
	case <-started:
	default:
		t.Fatal("stdout was not flushed before remote completion")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("REST disconnect did not send remote cancel")
	}
}

type fixtureFrame struct {
	typ     byte
	payload []byte
}

func readFixtureFrame(reader io.Reader) (fixtureFrame, error) {
	var header [5]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return fixtureFrame{}, err
	}
	payload := make([]byte, binary.BigEndian.Uint32(header[1:]))
	if _, err := io.ReadFull(reader, payload); err != nil {
		return fixtureFrame{}, err
	}
	return fixtureFrame{typ: header[0], payload: payload}, nil
}

func writeFixtureFrame(writer io.Writer, typ byte, payload []byte) error {
	header := make([]byte, 5)
	header[0] = typ
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if _, err := writer.Write(header); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

func writeFixtureFile(conn net.Conn, path string, content []byte) error {
	open := make([]byte, 10+len(path))
	binary.BigEndian.PutUint16(open, 1)
	binary.BigEndian.PutUint16(open[2:], 7)
	binary.BigEndian.PutUint32(open[4:], 0x241)
	binary.BigEndian.PutUint16(open[8:], 0o600)
	copy(open[10:], path)
	if err := writeFixtureFrame(conn, 0x20, open); err != nil {
		return err
	}
	if response, err := readFixtureFrame(conn); err != nil || response.typ != 0x40 {
		return errors.New("file open failed")
	}
	write := append([]byte{0, 2, 0, 7}, content...)
	if err := writeFixtureFrame(conn, 0x22, write); err != nil {
		return err
	}
	if response, err := readFixtureFrame(conn); err != nil || response.typ != 0x42 {
		return errors.New("file write failed")
	}
	if err := writeFixtureFrame(conn, 0x24, []byte{0, 3, 0, 7}); err != nil {
		return err
	}
	if response, err := readFixtureFrame(conn); err != nil || response.typ != 0x44 {
		return errors.New("file close failed")
	}
	return nil
}

type testMessage struct {
	body    []byte
	headers textproto.MIMEHeader
	msg     *types.RuleMsg
	err     error
	status  int
	flushes int
}

func newTestMessage(msg *types.RuleMsg) *testMessage {
	return &testMessage{headers: make(textproto.MIMEHeader), msg: msg}
}

func (m *testMessage) Body() []byte                  { return m.body }
func (m *testMessage) Headers() textproto.MIMEHeader { return m.headers }
func (m *testMessage) From() string                  { return "" }
func (m *testMessage) GetParam(string) string        { return "" }
func (m *testMessage) SetMsg(msg *types.RuleMsg)     { m.msg = msg }
func (m *testMessage) GetMsg() *types.RuleMsg        { return m.msg }
func (m *testMessage) SetStatusCode(status int)      { m.status = status }
func (m *testMessage) SetBody(body []byte)           { m.body = append(m.body, body...) }
func (m *testMessage) SetError(err error)            { m.err = err }
func (m *testMessage) GetError() error               { return m.err }
func (m *testMessage) Flush()                        { m.flushes++ }
