// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

// DefaultScope is Warden's default authentication scope (:default).
const DefaultScope = "default"

// EnvKey is the Rack env key under which the [Proxy] is injected, matching the
// gem's env['warden'].
const EnvKey = "warden"

// Option configures a [Manager] at construction, mirroring the settings a
// Warden::Config exposes (default scope, per-scope strategies, failure app).
type Option func(*Manager)

// WithFailureApp sets the failure application invoked when authentication is
// thrown and no strategy produced a redirect/custom response.
func WithFailureApp(app App) Option {
	return func(m *Manager) { m.failureApp = app }
}

// WithStrategyRun sets the [StrategyRun] seam used to run strategies.
func WithStrategyRun(run StrategyRun) Option {
	return func(m *Manager) { m.run = run }
}

// WithSessionStore sets the [SessionStore] seam (defaults to a
// [NewSerializerStore]).
func WithSessionStore(store SessionStore) Option {
	return func(m *Manager) { m.store = store }
}

// WithDefaultStrategies sets the strategy labels tried for scopes without an
// explicit list, in order.
func WithDefaultStrategies(names ...string) Option {
	return func(m *Manager) { m.defaultStrategies = names }
}

// WithScopeStrategies sets the strategy labels tried for a specific scope.
func WithScopeStrategies(scope string, names ...string) Option {
	return func(m *Manager) { m.scopeStrategies[scope] = names }
}

// WithDefaultScope overrides the default scope (default: [DefaultScope]).
func WithDefaultScope(scope string) Option {
	return func(m *Manager) { m.defaultScope = scope }
}

// WithIntercept401 makes the manager treat a downstream 401 response as an
// authentication failure and run the failure handling, like Warden's
// intercept_401.
func WithIntercept401() Option {
	return func(m *Manager) { m.intercept401 = true }
}
