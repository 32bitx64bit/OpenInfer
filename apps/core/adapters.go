package main

import (
	"github.com/openinfer/openinfer-studio/internal/chat"
	"github.com/openinfer/openinfer-studio/internal/instances"
	"github.com/openinfer/openinfer-studio/internal/proxy"
)

// chatEndpointAdapter adapts the instance manager to chat.EndpointProvider
// without introducing an import cycle between the two packages.
type chatEndpointAdapter struct{ im *instances.Manager }

func (a *chatEndpointAdapter) EndpointFor(modelID string) (chat.Endpoint, error) {
	ep, err := a.im.EndpointFor(modelID)
	if err != nil {
		return chat.Endpoint{}, err
	}
	return chat.Endpoint{URL: ep.URL, APIKey: ep.APIKey, Alias: ep.Alias}, nil
}

func (a *chatEndpointAdapter) Touch(modelID string) { a.im.Touch(modelID) }

// proxyEndpointAdapter adapts the instance manager to proxy.EndpointProvider.
type proxyEndpointAdapter struct{ im *instances.Manager }

func (a *proxyEndpointAdapter) EndpointFor(modelID string) (proxy.Endpoint, error) {
	ep, err := a.im.EndpointFor(modelID)
	if err != nil {
		return proxy.Endpoint{}, err
	}
	return proxy.Endpoint{URL: ep.URL, APIKey: ep.APIKey, Alias: ep.Alias}, nil
}

func (a *proxyEndpointAdapter) Touch(modelID string) { a.im.Touch(modelID) }

func (a *proxyEndpointAdapter) ResolveModelID(name string) (string, error) {
	return a.im.ResolveModelID(name)
}

func (a *proxyEndpointAdapter) LoadedModels() []string { return a.im.LoadedModels() }
