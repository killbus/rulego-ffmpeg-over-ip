package main

import (
	"net/http"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
)

const responseProcessor = "ffmpegOverIpResponse"

// Plugins is the symbol loaded by RuleGo's Go plugin registry.
var Plugins pluginRegistry

type pluginRegistry struct{}

func (*pluginRegistry) Init() error { return nil }

func (*pluginRegistry) Components() []types.Node {
	return []types.Node{&ffmpegOverIPNode{}, &ffmpegOverIPProducerNode{}}
}

func init() {
	// RuleGo's pinned plugin loader does not call PluginRegistry.Init, so the
	// companion REST projection must be registered during package loading.
	processor.OutBuiltins.Register(responseProcessor, projectResponse)
}

func projectResponse(_ endpoint.Router, exchange *endpoint.Exchange) bool {
	exchange.Lock()
	defer exchange.Unlock()
	if exchange.Out.GetError() != nil {
		exchange.Out.SetStatusCode(http.StatusBadGateway)
		return true
	}
	msg := exchange.Out.GetMsg()
	if msg == nil || msg.Metadata == nil || msg.Metadata.GetValue(channelKey) != "stdout" {
		return true
	}
	exchange.Out.SetBody(msg.GetBytes())
	if flusher, ok := exchange.Out.(endpoint.Flusher); ok {
		flusher.Flush()
	}
	return true
}
