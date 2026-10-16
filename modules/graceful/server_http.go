// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/proxyprotocol"
	"github.com/neko233-com/gitsss/modules/setting"
)

var webListeners = struct {
	sync.Mutex
	handler  http.Handler
	useProxy bool
	servers  map[string]*Server
}{servers: map[string]*Server{}}

func newHTTPServer(network, address, name string, handler http.Handler) (*Server, ServeFunction) {
	server := NewServer(network, address, name)
	protocols := http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)            // HTTP/2 can only be used when Gitea is configured to use TLS
	protocols.SetUnencryptedHTTP2(true) // Allow HTTP/2 without TLS, in case Gitea is behind a reverse proxy
	httpServer := http.Server{
		Protocols:   &protocols,
		Handler:     handler,
		BaseContext: func(net.Listener) context.Context { return GetManager().HammerContext() },
	}
	server.OnShutdown = func() {
		httpServer.SetKeepAlivesEnabled(false)
	}
	return server, httpServer.Serve
}

// HTTPListenAndServe listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServe(network, address, name string, handler http.Handler, useProxyProtocol bool) error {
	setWebHandler(handler, useProxyProtocol)
	return listenHTTP(network, address, name, handler, useProxyProtocol)
}

// HTTPListenAndServeWithAdditionalPorts starts the primary HTTP listener and
// any configured aliases with the same handler.
func HTTPListenAndServeWithAdditionalPorts(network, address, name string, handler http.Handler, useProxyProtocol bool, additionalAddresses []string) error {
	setWebHandler(handler, useProxyProtocol)
	started := make([]string, 0, len(additionalAddresses))
	for _, additionalAddress := range additionalAddresses {
		if err := AddWebListener(network, additionalAddress); err != nil {
			for _, startedAddress := range started {
				_ = RemoveWebListener(startedAddress)
			}
			return fmt.Errorf("listen on additional HTTP address %s: %w", additionalAddress, err)
		}
		started = append(started, additionalAddress)
	}
	err := listenHTTP(network, address, name, handler, useProxyProtocol)
	for _, startedAddress := range started {
		_ = RemoveWebListener(startedAddress)
	}
	return err
}

func setWebHandler(handler http.Handler, useProxyProtocol bool) {
	webListeners.Lock()
	webListeners.handler = handler
	webListeners.useProxy = useProxyProtocol
	webListeners.Unlock()
}

func listenHTTP(network, address, name string, handler http.Handler, useProxyProtocol bool) error {
	server, lHandler := newHTTPServer(network, address, name, handler)
	server.rebindable = network == "tcp"
	return server.ListenAndServe(lHandler, useProxyProtocol)
}

// AddWebListener starts another plain HTTP listener while preserving the current listener.
func AddWebListener(network, address string) error {
	webListeners.Lock()
	defer webListeners.Unlock()
	if webListeners.handler == nil {
		return errors.New("web handler is not initialized")
	}
	if _, exists := webListeners.servers[address]; exists {
		return fmt.Errorf("HTTP address %s is already listening", address)
	}
	select {
	case <-GetManager().IsShutdown():
		return net.ErrClosed
	default:
	}
	server, serve := newHTTPServer(network, address, "Web alias", webListeners.handler)
	listener, err := GetListener(network, address)
	if err != nil {
		return err
	}
	listener = newWrappedListener(listener, server)
	if webListeners.useProxy {
		listener = &proxyprotocol.Listener{
			Listener:           listener,
			ProxyHeaderTimeout: setting.ProxyProtocolHeaderTimeout,
			AcceptUnknown:      setting.ProxyProtocolAcceptUnknown,
		}
	}
	server.listener = listener
	server.BeforeBegin(network, address)
	webListeners.servers[address] = server
	go server.awaitShutdown()
	go func() {
		if err := server.Serve(serve); err != nil {
			log.Error("Additional HTTP listener %s stopped: %v", address, err)
		}
	}()
	return nil
}

// RemoveWebListener closes a secondary HTTP listener after a failed config update.
func RemoveWebListener(address string) error {
	webListeners.Lock()
	server, exists := webListeners.servers[address]
	if exists {
		delete(webListeners.servers, address)
	}
	webListeners.Unlock()
	if !exists {
		return nil
	}
	return server.listener.Close()
}

// HTTPListenAndServeTLSConfig listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServeTLSConfig(network, address, name string, tlsConfig *tls.Config, handler http.Handler, useProxyProtocol, proxyProtocolTLSBridging bool) error {
	server, lHandler := newHTTPServer(network, address, name, handler)
	return server.ListenAndServeTLSConfig(tlsConfig, lHandler, useProxyProtocol, proxyProtocolTLSBridging)
}
