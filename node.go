package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/killbus/rulego-ffmpeg-over-ip/client"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
)

const (
	componentType = "ffmpegOverIp"
	channelKey    = "ffmpegOverIp.channel"
	exitCodeKey   = "ffmpegOverIp.exitCode"
	maxTimeoutMs  = int64(1<<63-1) / int64(time.Millisecond)
)

type NodeConfiguration struct {
	Address          string `json:"address" label:"Address" desc:"TCP host:port or unix:/path/to/socket" required:"true"`
	AuthSecret       string `json:"authSecret" label:"Authentication secret" required:"true"`
	DialTimeoutMs    int64  `json:"dialTimeoutMs" label:"Dial timeout (ms)"`
	SessionTimeoutMs int64  `json:"sessionTimeoutMs" label:"Session timeout (ms)"`
}

type invocationRequest struct {
	Program     string   `json:"program"`
	Args        []string `json:"args"`
	StdinBase64 string   `json:"stdinBase64,omitempty"`
}

type terminalResult struct {
	Program  string `json:"program"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Message  string `json:"message,omitempty"`
}

type ffmpegOverIPNode struct {
	Config NodeConfiguration

	mu        sync.Mutex
	sessions  map[uint64]context.CancelFunc
	nextID    uint64
	destroyed bool
	wg        sync.WaitGroup
}

func (n *ffmpegOverIPNode) Type() string { return componentType }

func (n *ffmpegOverIPNode) Def() types.ComponentForm {
	relations := []string{types.Stream, types.Success, types.Failure}
	return types.ComponentForm{
		Type:          componentType,
		Category:      "external",
		Label:         "ffmpeg-over-ip",
		Desc:          "Run one authenticated remote ffmpeg or ffprobe invocation",
		Version:       "0.3.0",
		ComponentKind: types.ComponentKindNative,
		RelationTypes: &relations,
	}
}

func (n *ffmpegOverIPNode) New() types.Node {
	return &ffmpegOverIPNode{sessions: make(map[uint64]context.CancelFunc)}
}

func (n *ffmpegOverIPNode) Init(_ types.Config, configuration types.Configuration) error {
	var config NodeConfiguration
	if err := maps.Map2Struct(configuration, &config); err != nil {
		return fmt.Errorf("ffmpegOverIp: invalid configuration")
	}
	if config.Address == "" || config.AuthSecret == "" {
		return fmt.Errorf("ffmpegOverIp: address and authSecret are required")
	}
	if config.DialTimeoutMs < 0 || config.SessionTimeoutMs < 0 ||
		config.DialTimeoutMs > maxTimeoutMs || config.SessionTimeoutMs > maxTimeoutMs {
		return fmt.Errorf("ffmpegOverIp: timeouts are out of range")
	}
	if config.DialTimeoutMs == 0 {
		config.DialTimeoutMs = 5000
	}
	n.Config = config
	if n.sessions == nil {
		n.sessions = make(map[uint64]context.CancelFunc)
	}
	return nil
}

func (n *ffmpegOverIPNode) OnMsg(ruleContext types.RuleContext, msg types.RuleMsg) {
	request, stdin, err := decodeInvocation(msg.GetData())
	if err != nil {
		tellFailure(ruleContext, msg, "", nil, "invalid_input", "invalid invocation", err)
		return
	}
	parent := ruleContext.GetContext()
	if parent == nil {
		parent = context.Background()
	}
	sessionContext, cancel := context.WithCancel(parent)
	if n.Config.SessionTimeoutMs > 0 {
		var timeoutCancel context.CancelFunc
		sessionContext, timeoutCancel = context.WithTimeout(sessionContext, time.Duration(n.Config.SessionTimeoutMs)*time.Millisecond)
		previousCancel := cancel
		cancel = func() {
			timeoutCancel()
			previousCancel()
		}
	}

	id, ok := n.addSession(cancel)
	if !ok {
		cancel()
		tellFailure(ruleContext, msg, request.Program, nil, "canceled", "node is shutting down", errors.New("node is shutting down"))
		return
	}
	defer func() {
		cancel()
		n.removeSession(id)
	}()

	exitCode, runErr := client.Run(sessionContext, client.Config{
		Address:     n.Config.Address,
		AuthSecret:  n.Config.AuthSecret,
		DialTimeout: time.Duration(n.Config.DialTimeoutMs) * time.Millisecond,
	}, client.Invocation{
		Program: request.Program,
		Args:    request.Args,
		Stdin:   stdin,
	}, func(channel string, data []byte) {
		stream := msg.Copy()
		stream.DataType = types.BINARY
		stream.SetBytes(data)
		stream.Metadata.PutValue(channelKey, channel)
		ruleContext.TellNext(stream, types.Stream)
	})

	if runErr == nil {
		tellSuccess(ruleContext, msg, request.Program, exitCode)
		return
	}
	var sessionErr *client.Error
	if errors.As(runErr, &sessionErr) {
		tellFailure(ruleContext, msg, request.Program, sessionErr.ExitCode, sessionErr.Kind, sessionErr.Message, runErr)
		return
	}
	tellFailure(ruleContext, msg, request.Program, nil, "internal", "remote session failed", errors.New("remote session failed"))
}

func (n *ffmpegOverIPNode) Destroy() {
	n.mu.Lock()
	if n.destroyed {
		n.mu.Unlock()
		return
	}
	n.destroyed = true
	cancels := make([]context.CancelFunc, 0, len(n.sessions))
	for _, cancel := range n.sessions {
		cancels = append(cancels, cancel)
	}
	n.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

func (n *ffmpegOverIPNode) addSession(cancel context.CancelFunc) (uint64, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.destroyed {
		return 0, false
	}
	n.nextID++
	id := n.nextID
	n.sessions[id] = cancel
	n.wg.Add(1)
	return id, true
}

func (n *ffmpegOverIPNode) removeSession(id uint64) {
	n.mu.Lock()
	delete(n.sessions, id)
	n.mu.Unlock()
	n.wg.Done()
}

func decodeInvocation(data string) (invocationRequest, io.Reader, error) {
	var request invocationRequest
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return request, nil, err
	}
	stdin, err := decodeStdin(request)
	return request, stdin, err
}

func decodeStdin(request invocationRequest) (io.Reader, error) {
	if request.Args == nil {
		return nil, errors.New("args is required")
	}
	if err := client.ValidateInvocation(request.Program, request.Args); err != nil {
		return nil, err
	}
	if request.StdinBase64 == "" {
		return nil, nil
	}
	encoding := base64.StdEncoding.Strict()
	check := base64.NewDecoder(encoding, strings.NewReader(request.StdinBase64))
	buffer := make([]byte, 32*1024)
	if _, err := io.CopyBuffer(io.Discard, check, buffer); err != nil {
		return nil, errors.New("stdinBase64 is invalid")
	}
	return base64.NewDecoder(encoding, strings.NewReader(request.StdinBase64)), nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func tellSuccess(ctx types.RuleContext, input types.RuleMsg, program string, exitCode int) {
	payload, _ := json.Marshal(terminalResult{Program: program, ExitCode: &exitCode})
	output := input.Copy()
	output.DataType = types.JSON
	output.SetBytes(payload)
	output.Metadata.Delete(channelKey)
	output.Metadata.PutValue(exitCodeKey, strconv.Itoa(exitCode))
	ctx.TellSuccess(output)
}

func tellReady(ctx types.RuleContext, input types.RuleMsg, program string) {
	payload, _ := json.Marshal(terminalResult{Program: program, Kind: "ready"})
	output := input.Copy()
	output.DataType = types.JSON
	output.SetBytes(payload)
	output.Metadata.Delete(channelKey)
	output.Metadata.Delete(exitCodeKey)
	ctx.TellSuccess(output)
}

func tellFailure(ctx types.RuleContext, input types.RuleMsg, program string, exitCode *int, kind, message string, err error) {
	if program != "ffmpeg" && program != "ffprobe" {
		program = ""
	}
	payload, _ := json.Marshal(terminalResult{Program: program, ExitCode: exitCode, Kind: kind, Message: message})
	output := input.Copy()
	output.DataType = types.JSON
	output.SetBytes(payload)
	output.Metadata.Delete(channelKey)
	if exitCode != nil {
		output.Metadata.PutValue(exitCodeKey, strconv.Itoa(*exitCode))
	} else {
		output.Metadata.Delete(exitCodeKey)
	}
	ctx.TellFailure(output, err)
}
