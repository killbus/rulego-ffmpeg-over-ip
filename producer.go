package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/killbus/rulego-ffmpeg-over-ip/client"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
)

const producerComponentType = "ffmpegOverIpProducer"

type producerConfiguration struct {
	Address       string `json:"address" label:"Address" desc:"TCP host:port or unix:/path/to/socket" required:"true"`
	AuthSecret    string `json:"authSecret" label:"Authentication secret" required:"true"`
	DialTimeoutMs int64  `json:"dialTimeoutMs" label:"Dial timeout (ms)"`
}

type producerRequest struct {
	Key          string            `json:"key"`
	Invocation   invocationRequest `json:"invocation"`
	AwaitFile    string            `json:"awaitFile"`
	AwaitOnly    bool              `json:"awaitOnly"`
	RunTimeoutMs int64             `json:"runTimeoutMs"`
	CacheTTLms   int64             `json:"cacheTtlMs"`
}

type ffmpegOverIPProducerNode struct {
	Config producerConfiguration

	mu        sync.Mutex
	jobs      map[string]*producerJob
	destroyed bool
	wg        sync.WaitGroup
}

type producerJob struct {
	fingerprint [sha256.Size]byte
	cancel      context.CancelFunc
	done        chan struct{}
	notify      chan struct{}
	ready       map[string]struct{}
	files       map[string]struct{}
	waiters     int
	err         error
	ttl         time.Duration
}

type cachedFile struct {
	path    string
	size    int64
	modTime time.Time
}

func (*ffmpegOverIPProducerNode) Type() string { return producerComponentType }

func (*ffmpegOverIPProducerNode) Def() types.ComponentForm {
	relations := []string{types.Stream, types.Success, types.Failure}
	return types.ComponentForm{
		Type:          producerComponentType,
		Category:      "external",
		Label:         "ffmpeg-over-ip producer",
		Desc:          "Share and bound a remote invocation that produces files",
		Version:       "0.3.1",
		ComponentKind: types.ComponentKindNative,
		RelationTypes: &relations,
	}
}

func (*ffmpegOverIPProducerNode) New() types.Node {
	return &ffmpegOverIPProducerNode{jobs: make(map[string]*producerJob)}
}

func (n *ffmpegOverIPProducerNode) Init(_ types.Config, configuration types.Configuration) error {
	var config producerConfiguration
	if err := maps.Map2Struct(configuration, &config); err != nil {
		return errors.New("ffmpegOverIpProducer: invalid configuration")
	}
	if config.Address == "" || config.AuthSecret == "" {
		return errors.New("ffmpegOverIpProducer: address and authSecret are required")
	}
	if config.DialTimeoutMs < 0 || config.DialTimeoutMs > maxTimeoutMs {
		return errors.New("ffmpegOverIpProducer: timeout is out of range")
	}
	if config.DialTimeoutMs == 0 {
		config.DialTimeoutMs = 5000
	}
	n.Config = config
	if n.jobs == nil {
		n.jobs = make(map[string]*producerJob)
	}
	return nil
}

func (n *ffmpegOverIPProducerNode) OnMsg(ruleContext types.RuleContext, msg types.RuleMsg) {
	request, stdin, err := decodeProducerRequest(msg.GetData())
	if err != nil {
		tellFailure(ruleContext, msg, "", nil, "invalid_input", "invalid producer request", err)
		return
	}
	fingerprint := invocationFingerprint(request.Invocation)
	job, same := n.currentJob(request.Key, fingerprint)
	if job != nil && !same && n.jobFileReady(job, request.AwaitFile) {
		if !request.AwaitOnly {
			if err := streamReadyFile(ruleContext, msg, request.AwaitFile); err != nil {
				tellFailure(ruleContext, msg, request.Invocation.Program, nil, "file", "ready file could not be streamed", err)
				return
			}
		}
		_, _ = n.ensureJob(request, stdin, fingerprint)
		tellReady(ruleContext, msg, request.Invocation.Program)
		return
	}
	if job == nil || !same {
		var ok bool
		job, ok = n.ensureJob(request, stdin, fingerprint)
		if !ok {
			tellFailure(ruleContext, msg, request.Invocation.Program, nil, "canceled", "node is shutting down", errors.New("node is shutting down"))
			return
		}
	}
	if err := n.waitForFile(ruleContext.GetContext(), job, request.AwaitFile); err != nil {
		tellJobFailure(ruleContext, msg, request.Invocation.Program, err)
		return
	}
	if !request.AwaitOnly {
		if err := streamReadyFile(ruleContext, msg, request.AwaitFile); err != nil {
			tellFailure(ruleContext, msg, request.Invocation.Program, nil, "file", "ready file could not be streamed", err)
			return
		}
	}
	tellReady(ruleContext, msg, request.Invocation.Program)
}

func (n *ffmpegOverIPProducerNode) Destroy() {
	n.mu.Lock()
	if n.destroyed {
		n.mu.Unlock()
		return
	}
	n.destroyed = true
	cancels := make([]context.CancelFunc, 0, len(n.jobs))
	for _, job := range n.jobs {
		cancels = append(cancels, job.cancel)
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

func decodeProducerRequest(data string) (producerRequest, io.Reader, error) {
	var request producerRequest
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return request, nil, err
	}
	if strings.TrimSpace(request.Key) == "" || len(request.Key) > 512 {
		return request, nil, errors.New("key is invalid")
	}
	if request.AwaitFile == "" || len(request.AwaitFile) > 4096 {
		return request, nil, errors.New("awaitFile is invalid")
	}
	request.AwaitFile = filepath.Clean(request.AwaitFile)
	if request.RunTimeoutMs <= 0 || request.CacheTTLms <= 0 ||
		request.RunTimeoutMs > maxTimeoutMs || request.CacheTTLms > maxTimeoutMs {
		return request, nil, errors.New("producer timeouts are out of range")
	}
	stdin, err := decodeStdin(request.Invocation)
	return request, stdin, err
}

func invocationFingerprint(request invocationRequest) [sha256.Size]byte {
	payload, _ := json.Marshal(request)
	return sha256.Sum256(payload)
}

func (n *ffmpegOverIPProducerNode) currentJob(key string, fingerprint [sha256.Size]byte) (*producerJob, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	job := n.jobs[key]
	return job, job != nil && job.fingerprint == fingerprint
}

func (n *ffmpegOverIPProducerNode) ensureJob(request producerRequest, stdin io.Reader, fingerprint [sha256.Size]byte) (*producerJob, bool) {
	n.mu.Lock()
	if n.destroyed {
		n.mu.Unlock()
		return nil, false
	}
	if current := n.jobs[request.Key]; current != nil && current.fingerprint == fingerprint {
		n.mu.Unlock()
		return current, true
	} else if current != nil {
		current.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(request.RunTimeoutMs)*time.Millisecond)
	job := &producerJob{
		fingerprint: fingerprint,
		cancel:      cancel,
		done:        make(chan struct{}),
		notify:      make(chan struct{}),
		ready:       make(map[string]struct{}),
		files:       make(map[string]struct{}),
		ttl:         time.Duration(request.CacheTTLms) * time.Millisecond,
	}
	n.jobs[request.Key] = job
	n.wg.Add(1)
	n.mu.Unlock()

	go n.runJob(ctx, request.Invocation, stdin, job)
	return job, true
}

func (n *ffmpegOverIPProducerNode) runJob(ctx context.Context, request invocationRequest, stdin io.Reader, job *producerJob) {
	defer n.wg.Done()
	_, runErr := client.Run(ctx, client.Config{
		Address:     n.Config.Address,
		AuthSecret:  n.Config.AuthSecret,
		DialTimeout: time.Duration(n.Config.DialTimeoutMs) * time.Millisecond,
	}, client.Invocation{
		Program: request.Program,
		Args:    request.Args,
		Stdin:   stdin,
		OnFileReady: func(path string) {
			path = filepath.Clean(path)
			n.mu.Lock()
			job.ready[path] = struct{}{}
			job.files[path] = struct{}{}
			close(job.notify)
			job.notify = make(chan struct{})
			n.mu.Unlock()
		},
	}, nil)

	n.mu.Lock()
	job.err = runErr
	close(job.done)
	files := make([]cachedFile, 0, len(job.files))
	for path := range job.files {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files = append(files, cachedFile{path: path, size: info.Size(), modTime: info.ModTime()})
		}
	}
	if runErr != nil {
		for key, current := range n.jobs {
			if current == job {
				delete(n.jobs, key)
			}
		}
	}
	n.mu.Unlock()
	job.cancel()
	time.AfterFunc(job.ttl, func() { n.expireJob(job, files) })
}

func (n *ffmpegOverIPProducerNode) expireJob(job *producerJob, files []cachedFile) {
	n.mu.Lock()
	for key, current := range n.jobs {
		if current == job {
			delete(n.jobs, key)
		}
	}
	n.mu.Unlock()
	for _, file := range files {
		if info, err := os.Stat(file.path); err == nil && info.Size() == file.size && info.ModTime().Equal(file.modTime) {
			_ = os.Remove(file.path)
		}
	}
}

func (n *ffmpegOverIPProducerNode) jobFileReady(job *producerJob, path string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ok := job.ready[path]
	return ok
}

func (n *ffmpegOverIPProducerNode) jobFileState(job *producerJob, path string) (bool, <-chan struct{}) {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ready := job.ready[path]
	return ready, job.notify
}

func (n *ffmpegOverIPProducerNode) waitForFile(ctx context.Context, job *producerJob, path string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	n.mu.Lock()
	job.waiters++
	n.mu.Unlock()
	aborted := false
	defer func() {
		n.mu.Lock()
		job.waiters--
		cancel := aborted && job.waiters == 0
		n.mu.Unlock()
		if cancel {
			job.cancel()
		}
	}()
	for {
		ready, notify := n.jobFileState(job, path)
		if ready {
			return nil
		}
		select {
		case <-notify:
		case <-job.done:
			if n.jobFileReady(job, path) {
				return nil
			}
			if job.err != nil {
				return job.err
			}
			return errors.New("remote process exited before the requested file was ready")
		case <-ctx.Done():
			aborted = true
			return ctx.Err()
		}
	}
}

func streamReadyFile(ruleContext types.RuleContext, msg types.RuleMsg, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	buffer := make([]byte, 256*1024)
	for {
		read, readErr := file.Read(buffer)
		if read > 0 {
			stream := msg.Copy()
			stream.DataType = types.BINARY
			stream.SetBytes(buffer[:read])
			stream.Metadata.PutValue(channelKey, "stdout")
			ruleContext.TellNext(stream, types.Stream)
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func tellJobFailure(ruleContext types.RuleContext, msg types.RuleMsg, program string, err error) {
	var sessionErr *client.Error
	if errors.As(err, &sessionErr) {
		tellFailure(ruleContext, msg, program, sessionErr.ExitCode, sessionErr.Kind, sessionErr.Message, err)
		return
	}
	kind := "file"
	message := "requested file was not produced"
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		kind = "canceled"
		message = "request canceled while waiting for file"
	}
	tellFailure(ruleContext, msg, program, nil, kind, message, err)
}
