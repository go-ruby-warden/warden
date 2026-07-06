// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

import "github.com/go-ruby-rack/rack"

// App is a Rack application: it maps a [rack.Env] to the SPEC
// [status, headers, body] tuple, returned as a *[rack.Response]. Both the
// downstream app wrapped by a [Manager] and the failure app are Apps.
type App func(env rack.Env) *rack.Response

// Manager is the Warden Rack middleware. It wraps a downstream [App], injects a
// [Proxy] as env["warden"], runs the app inside a catch for the throw :warden
// signal, and on a throw dispatches to the failure app / redirect / custom
// response.
type Manager struct {
	app               App
	failureApp        App
	run               StrategyRun
	store             SessionStore
	defaultScope      string
	defaultStrategies []string
	scopeStrategies   map[string][]string
	intercept401      bool
}

// New builds a Manager wrapping app, applying the given options. Without a
// [SessionStore] option it defaults to a [NewSerializerStore]; without a
// [WithDefaultScope] the scope is [DefaultScope].
func New(app App, opts ...Option) *Manager {
	m := &Manager{
		app:             app,
		store:           NewSerializerStore(),
		defaultScope:    DefaultScope,
		scopeStrategies: map[string][]string{},
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// strategiesFor returns the strategy labels for a scope: its explicit list if
// configured, else the default strategies.
func (m *Manager) strategiesFor(scope string) []string {
	if names, ok := m.scopeStrategies[scope]; ok {
		return names
	}
	return m.defaultStrategies
}

// Call runs the middleware for a single request. It injects the proxy, runs the
// downstream app under a throw :warden catch, and returns the app's response —
// or, when authentication was thrown (or a 401 was intercepted), the failure
// response.
func (m *Manager) Call(env rack.Env) *rack.Response {
	proxy := m.newProxy(env)
	env[EnvKey] = proxy

	thrown, resp := m.catch(func() *rack.Response { return m.app(env) })
	if thrown != nil {
		return m.processUnauthenticated(proxy, env, thrown.Options)
	}
	if m.intercept401 && resp != nil && resp.Status() == 401 {
		return m.processUnauthenticated(proxy, env, ThrowOptions{})
	}
	return resp
}

// catch runs f, recovering a *[Throw] panic (the throw :warden signal) and
// returning it; any other panic propagates.
func (m *Manager) catch(f func() *rack.Response) (thrown *Throw, resp *rack.Response) {
	defer func() {
		if r := recover(); r != nil {
			if t, ok := r.(*Throw); ok {
				thrown = t
				return
			}
			panic(r)
		}
	}()
	resp = f()
	return
}

// processUnauthenticated dispatches a thrown authentication. A winning redirect
// or custom strategy result returns its response; otherwise the failure app runs.
func (m *Manager) processUnauthenticated(p *Proxy, env rack.Env, opts ThrowOptions) *rack.Response {
	scope := opts.Scope
	if scope == "" {
		scope = m.defaultScope
	}
	result := opts.Result
	if result == "" {
		result = p.results[scope]
	}
	switch result {
	case ResultRedirect, ResultCustom:
		if resp := p.responses[scope]; resp != nil {
			return resp
		}
	}
	return m.callFailureApp(env, opts)
}

// callFailureApp invokes the failure app after setting PATH_INFO and
// env["warden.options"], matching the gem's call_failure_app. With no failure
// app configured it panics with a *[NotAuthenticated] (the "No Failure App"
// raise).
func (m *Manager) callFailureApp(env rack.Env, opts ThrowOptions) *rack.Response {
	if m.failureApp == nil {
		panic(&NotAuthenticated{Scope: opts.Scope, Message: opts.Message})
	}
	action := opts.Action
	if action == "" {
		action = "unauthenticated"
	}
	env["PATH_INFO"] = "/" + action
	env["warden.options"] = opts
	return m.failureApp(env)
}
