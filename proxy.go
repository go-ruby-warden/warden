// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

import "github.com/go-ruby-rack/rack"

// AuthOptions tunes a single Authenticate / AuthenticateBang call, mirroring the
// scope: and strategy-list arguments the gem accepts.
type AuthOptions struct {
	// Scope is the authentication scope; empty means the manager's default scope.
	Scope string
	// Strategies overrides the strategy labels tried for this call; empty means
	// the scope's configured strategies.
	Strategies []string
}

// Proxy is the env["warden"] object: the per-request, scope-aware
// authentication handle. It caches the user per scope, runs strategies, and
// records the winning strategy, result and message.
type Proxy struct {
	env         rack.Env
	manager     *Manager
	users       map[string]any            // scope -> user (key present = loaded, even if nil)
	results     map[string]Result         // scope -> winning result
	responses   map[string]*rack.Response // scope -> winning redirect/custom response
	messages    map[string]string         // scope -> failure message
	winning     *StrategyResult           // last-run (winning) strategy
	winningName string
}

func (m *Manager) newProxy(env rack.Env) *Proxy {
	return &Proxy{
		env:       env,
		manager:   m,
		users:     map[string]any{},
		results:   map[string]Result{},
		responses: map[string]*rack.Response{},
		messages:  map[string]string{},
	}
}

// FromEnv returns the [Proxy] injected into a Rack env by a [Manager], or nil
// when none is present. Downstream apps use it to reach env["warden"].
func FromEnv(env rack.Env) *Proxy {
	if p, ok := env[EnvKey].(*Proxy); ok {
		return p
	}
	return nil
}

// scope resolves an optional scope argument to a concrete scope, defaulting to
// the manager's default scope.
func (p *Proxy) scope(scopes []string) string {
	if len(scopes) > 0 && scopes[0] != "" {
		return scopes[0]
	}
	return p.manager.defaultScope
}

// Authenticate runs the strategies for a scope and returns the authenticated
// user, or nil on failure. It never throws.
func (p *Proxy) Authenticate(opts ...AuthOptions) any {
	user, _ := p.performAuthentication(firstOpt(opts))
	return user
}

// AuthenticateBang runs the strategies for a scope and returns the user, or
// panics with a *[Throw] (the throw :warden signal) on failure — the
// authenticate! method.
func (p *Proxy) AuthenticateBang(opts ...AuthOptions) any {
	user, throwOpts := p.performAuthentication(firstOpt(opts))
	if user == nil {
		panic(&Throw{Options: throwOpts})
	}
	return user
}

func firstOpt(opts []AuthOptions) AuthOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return AuthOptions{}
}

// performAuthentication returns the authenticated user for a scope (nil on
// failure) and the throw options describing the outcome. A user already cached
// or loadable from the session short-circuits the strategy run.
func (p *Proxy) performAuthentication(opts AuthOptions) (any, ThrowOptions) {
	scope := opts.Scope
	if scope == "" {
		scope = p.manager.defaultScope
	}
	throwOpts := ThrowOptions{Scope: scope, Action: "unauthenticated"}

	if u, ok := p.users[scope]; ok && u != nil {
		return u, throwOpts
	}
	if u := p.User(scope); u != nil {
		return u, throwOpts
	}

	names := opts.Strategies
	if len(names) == 0 {
		names = p.manager.strategiesFor(scope)
	}
	p.runStrategies(scope, names)

	if u, ok := p.users[scope]; ok && u != nil {
		return u, throwOpts
	}
	throwOpts.Result = p.results[scope]
	throwOpts.Message = p.messages[scope]
	throwOpts.Strategy = p.winningName
	return nil, throwOpts
}

// runStrategies runs the named strategies in order, stopping at the first that
// halts. success! sets the user; a halting failure/redirect/custom records the
// result and response; pass and the non-bang fail continue.
func (p *Proxy) runStrategies(scope string, names []string) {
	if p.manager.run == nil {
		return
	}
	for _, name := range names {
		res := p.manager.run(name, p.env)
		if !res.Valid {
			continue
		}
		p.winning = &res
		p.winningName = name
		if res.Message != "" {
			p.messages[scope] = res.Message
		}
		if res.Result == ResultSuccess {
			p.results[scope] = ResultSuccess
			p.SetUser(res.User, scope, true)
			return
		}
		if res.Halted {
			p.results[scope] = res.Result
			p.responses[scope] = res.Response
			return
		}
	}
}

// User returns the user for a scope, loading it from the session on first
// access, or nil when none is set.
func (p *Proxy) User(scope ...string) any {
	s := p.scope(scope)
	if u, ok := p.users[s]; ok {
		return u
	}
	if u, present := p.manager.store.SerializeFromSession(p.env, s); present {
		p.users[s] = u
		return u
	}
	return nil
}

// SetUser sets the user for a scope. When store is true it is also serialized
// into the session (set_user with store: true); pass false to skip persistence
// (store: false).
func (p *Proxy) SetUser(user any, scope string, store bool) any {
	s := p.scope([]string{scope})
	p.users[s] = user
	if store {
		p.manager.store.SerializeIntoSession(p.env, s, user)
	}
	return user
}

// Authenticated reports whether a user is set for a scope (authenticated?),
// loading it from the session if needed.
func (p *Proxy) Authenticated(scope ...string) bool { return p.User(scope...) != nil }

// Unauthenticated is the negation of [Proxy.Authenticated].
func (p *Proxy) Unauthenticated(scope ...string) bool { return !p.Authenticated(scope...) }

// Logout logs out the given scopes, deleting each from the session. With no
// scopes it logs out every loaded scope and resets the session.
func (p *Proxy) Logout(scopes ...string) {
	reset := false
	if len(scopes) == 0 {
		for s := range p.users {
			scopes = append(scopes, s)
		}
		reset = true
	}
	for _, s := range scopes {
		delete(p.users, s)
		p.manager.store.Delete(p.env, s)
	}
	if reset {
		p.manager.store.Reset(p.env)
	}
}

// WinningStrategy returns the label of the last strategy that ran, or empty
// when none ran.
func (p *Proxy) WinningStrategy() string { return p.winningName }

// Message returns the winning strategy's message, or empty when none ran.
func (p *Proxy) Message() string {
	if p.winning != nil {
		return p.winning.Message
	}
	return ""
}

// Result returns the winning result for a scope (empty when none halted).
func (p *Proxy) Result(scope ...string) Result { return p.results[p.scope(scope)] }
