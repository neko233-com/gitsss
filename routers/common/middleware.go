// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/neko233-com/gitsss/modules/cache"
	"github.com/neko233-com/gitsss/modules/gtprof"
	"github.com/neko233-com/gitsss/modules/httplib"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/public"
	"github.com/neko233-com/gitsss/modules/reqctx"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/web/routing"
	"github.com/neko233-com/gitsss/services/context"

	"gitea.com/go-chi/session"
	"github.com/go-chi/chi/v5"
)

// ProtocolMiddlewares returns HTTP protocol related middlewares, and it provides a global panic recovery
func ProtocolMiddlewares() (handlers []any) {
	// the order is important
	handlers = append(handlers, ChiRoutePathHandler())   // make sure chi has correct paths
	handlers = append(handlers, RequestContextHandler()) //	prepare the context and panic recovery
	handlers = append(handlers, SecurityHeadersHandler())

	handlers = append(handlers, ForwardedHeadersHandler(setting.ReverseProxyLimit, setting.ReverseProxyTrustedProxies))

	handlers = append(handlers, routing.NewRequestInfoHandler())

	if setting.IsAccessLogEnabled() {
		handlers = append(handlers, context.AccessLogger())
	}

	if !setting.IsProd {
		handlers = append(handlers, public.ViteDevMiddleware)
	}

	return handlers
}

// SecurityHeadersHandler sets headers globally for every response that leaves Gitea.
func SecurityHeadersHandler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if setting.Security.XContentTypeOptions != "unset" {
				resp.Header().Set("X-Content-Type-Options", setting.Security.XContentTypeOptions)
			}
			if setting.Security.XFrameOptions != "unset" {
				resp.Header().Set("X-Frame-Options", setting.Security.XFrameOptions)
			}
			next.ServeHTTP(resp, req)
		})
	}
}

func RequestContextHandler() func(h http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(respOrig http.ResponseWriter, req *http.Request) {
			// this response writer might not be the same as the one in context.Base.Resp
			// because there might be a "gzip writer" in the middle, so the "written size" here is the compressed size
			respWriter := context.WrapResponseWriter(respOrig)

			profDesc := fmt.Sprintf("HTTP: %s %s", req.Method, req.RequestURI)
			ctx, finished := reqctx.NewRequestContext(req.Context(), profDesc)
			defer finished()

			ctx, span := gtprof.GetTracer().Start(ctx, gtprof.TraceSpanHTTP)
			req = req.WithContext(ctx)
			defer func() {
				chiCtx := chi.RouteContext(req.Context())
				span.SetAttributeString(gtprof.TraceAttrHTTPRoute, chiCtx.RoutePattern())
				span.End()
			}()

			defer func() {
				if recovered := recover(); recovered != nil {
					renderPanicErrorPage(respWriter, req, recovered) // it should never panic, and it handles the stack trace internally
				}
			}()

			ds := reqctx.GetRequestDataStore(ctx)
			req = req.WithContext(cache.WithCacheContext(ctx))
			ds.SetContextValue(httplib.RequestContextKey, req)
			ds.AddCleanUp(func() {
				// TODO: GOLANG-HTTP-TMPDIR: Golang saves the uploaded files to temp directory (TMPDIR) when parsing multipart-form.
				// The "req" might have changed due to the new "req.WithContext" calls
				// For example: in NewBaseContext, a new "req" with context is created, and the multipart-form is parsed there.
				// So we always use the latest "req" from the data store.
				ctxReq := ds.GetContextValue(httplib.RequestContextKey).(*http.Request)
				if ctxReq.MultipartForm != nil {
					_ = ctxReq.MultipartForm.RemoveAll() // remove the temp files buffered to tmp directory
				}
			})
			next.ServeHTTP(respWriter, req)
		})
	}
}

func ChiRoutePathHandler() func(h http.Handler) http.Handler {
	// make sure chi uses EscapedPath(RawPath) as RoutePath, then "%2f" could be handled correctly
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			chiCtx := chi.RouteContext(req.Context())
			if req.URL.RawPath == "" {
				chiCtx.RoutePath = req.URL.EscapedPath()
			} else {
				chiCtx.RoutePath = req.URL.RawPath
			}
			next.ServeHTTP(resp, req)
		})
	}
}

func ForwardedHeadersHandler(limit int, trustedProxies []string) func(h http.Handler) http.Handler {
	protoHeader := setting.ReverseProxyProtoHeader
	trusted := make([]netip.Prefix, 0, len(trustedProxies))
	trustAll := false
	for _, value := range trustedProxies {
		if value == "*" {
			trustAll = true
			continue
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			trusted = append(trusted, prefix)
			continue
		}
		if addr, err := netip.ParseAddr(value); err == nil {
			trusted = append(trusted, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	isTrusted := func(addr netip.Addr) bool {
		if trustAll {
			return true
		}
		for _, prefix := range trusted {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			host, _, _ := net.SplitHostPort(req.RemoteAddr)
			if req.RemoteAddr == "@" {
				host = "127.0.0.1"
			}
			peer, err := netip.ParseAddr(host)
			if err == nil {
				peer = peer.Unmap()
			}
			if err != nil || limit <= 0 || !isTrusted(peer) {
				if protoHeader != "" {
					req.Header.Del(protoHeader)
				}
				// URL generation must not trust a spoofed scheme from a direct client either.
				for _, header := range []string{"Forwarded", "X-Forwarded-For", "X-Real-IP", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-Protocol", "X-Url-Scheme", "Front-End-Https", "X-Forwarded-Ssl"} {
					req.Header.Del(header)
				}
				next.ServeHTTP(w, req)
				return
			}
			if protoHeader != "" {
				proto := req.Header.Get(protoHeader)
				for _, header := range []string{"X-Forwarded-Proto", "X-Forwarded-Protocol", "X-Url-Scheme", "Front-End-Https", "X-Forwarded-Ssl"} {
					req.Header.Del(header)
				}
				if proto == "http" || proto == "https" {
					req.Header.Set("X-Forwarded-Proto", proto)
				}
			}
			// Walk from the socket toward the client, and stop at the first untrusted hop.
			chain := strings.Join(req.Header.Values("X-Forwarded-For"), ",")
			if chain == "" {
				chain = req.Header.Get("X-Real-IP")
			}
			if len(chain) <= 4096 {
				parts := strings.Split(chain, ",")
				for i, hops := len(parts)-1, 0; i >= 0 && hops < limit && isTrusted(peer); i, hops = i-1, hops+1 {
					addr, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
					if err != nil {
						break
					}
					peer = addr.Unmap()
				}
			}
			req.RemoteAddr = net.JoinHostPort(peer.String(), "0")
			next.ServeHTTP(w, req)
		})
	}
}

func MustInitSessioner() func(next http.Handler) http.Handler {
	// TODO: CHI-SESSION-GOB-REGISTER: chi-session has a design problem: it calls gob.Register for "Set"
	// But if the server restarts, then the first "Get" will fail to decode the previously stored session data because the structs are not registered yet.
	// So each package should make sure their structs are registered correctly during startup for session storage.

	middleware, err := session.Sessioner(session.Options{
		Provider:       setting.SessionConfig.Provider,
		ProviderConfig: setting.SessionConfig.ProviderConfig,
		CookieName:     setting.SessionConfig.CookieName,
		CookiePath:     setting.SessionConfig.CookiePath,
		Gclifetime:     setting.SessionConfig.Gclifetime,
		Maxlifetime:    setting.SessionConfig.Maxlifetime,
		Secure:         setting.SessionConfig.Secure,
		SameSite:       setting.SessionConfig.SameSite,
		Domain:         setting.SessionConfig.Domain,

		// in the future, if websocket is used, the websocket handler should manage its own session sync (release)
		IgnoreReleaseForWebSocket: true,
	})
	if err != nil {
		log.Fatal("common.Sessioner failed: %v", err)
	}
	return middleware
}
