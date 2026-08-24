package client

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

func TestPinnedUpstreamServer(t *testing.T) {
	address := os.Getenv("FFOIP_INTEGRATION_ADDRESS")
	if address == "" {
		t.Skip("FFOIP_INTEGRATION_ADDRESS is set by the pinned-upstream CI job")
	}
	input := []byte{0x00, 0x41, 0x80, 0xff}
	var stdout bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	code, err := Run(ctx, Config{Address: address, AuthSecret: "integration-secret", DialTimeout: 2 * time.Second}, Invocation{
		Program: "ffmpeg",
		Args: []string{
			"-hide_banner", "-loglevel", "error",
			"-f", "rawvideo", "-pixel_format", "gray", "-video_size", "2x2", "-framerate", "1",
			"-i", "pipe:0", "-frames:v", "1", "-f", "rawvideo", "pipe:1",
		},
		Stdin: bytes.NewReader(input),
	}, func(channel string, data []byte) {
		if channel == "stdout" {
			stdout.Write(data)
		}
	})
	if err != nil || code != 0 {
		t.Fatalf("pinned server invocation = %d, %v", code, err)
	}
	if !bytes.Equal(stdout.Bytes(), input) {
		t.Fatalf("binary output = %x, want %x", stdout.Bytes(), input)
	}

	_, err = Run(ctx, Config{Address: address, AuthSecret: "wrong-secret", DialTimeout: 2 * time.Second}, Invocation{
		Program: "ffprobe",
		Args:    []string{"-version"},
	}, nil)
	if sessionErr, ok := err.(*Error); !ok || sessionErr.Kind != "server" {
		t.Fatalf("incorrect authentication error = %#v", err)
	}
}
