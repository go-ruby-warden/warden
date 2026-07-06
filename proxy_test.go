// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

// newProxyForTest builds a manager and returns a freshly-injected proxy plus env.
func newProxyForTest(t *testing.T, opts ...Option) (*Proxy, rack.Env) {
	t.Helper()
	m := New(appReturning("ok"), opts...)
	env := rack.Env{}
	m.Call(env)
	return FromEnv(env), env
}

func TestSetUserStoreTrueAndFalse(t *testing.T) {
	p, env := newProxyForTest(t)
	u := &user{id: 1}
	p.SetUser(u, "default", true)
	sess, _ := env["rack.session"].(map[string]any)
	if sess[SessionKeyFor("default")] != u {
		t.Fatal("expected user serialized into session")
	}

	p2, env2 := newProxyForTest(t)
	p2.SetUser(&user{id: 2}, "default", false)
	if _, ok := env2["rack.session"]; ok {
		t.Fatal("store:false should not touch the session")
	}
	if !p2.Authenticated() {
		t.Fatal("cached user should read back as authenticated")
	}
}

func TestUserLoadsFromSessionAndCaches(t *testing.T) {
	u := &user{id: 3, name: "grace"}
	p, env := newProxyForTest(t)
	// Seed the session directly.
	env["rack.session"] = map[string]any{SessionKeyFor("default"): u}
	if p.User() != u {
		t.Fatal("expected user loaded from session")
	}
	// Now it is cached: mutate the session and confirm the cache wins.
	env["rack.session"].(map[string]any)[SessionKeyFor("default")] = &user{id: 99}
	if p.User().(*user).id != 3 {
		t.Fatal("expected cached user")
	}
}

func TestUserAbsentReturnsNil(t *testing.T) {
	p, _ := newProxyForTest(t)
	if p.User("nobody") != nil {
		t.Fatal("expected nil user")
	}
	if p.Authenticated("nobody") {
		t.Fatal("expected not authenticated")
	}
}

func TestAuthenticateShortCircuitsCachedUser(t *testing.T) {
	u := &user{id: 5}
	run := func(string, rack.Env) StrategyResult {
		t.Fatal("strategies must not run when a user is cached")
		return StrategyResult{}
	}
	p, _ := newProxyForTest(t, WithStrategyRun(run), WithDefaultStrategies("x"))
	p.SetUser(u, "default", false)
	if p.Authenticate() != u {
		t.Fatal("expected cached user from Authenticate")
	}
}

func TestAuthenticateShortCircuitsSessionUser(t *testing.T) {
	u := &user{id: 6}
	run := func(string, rack.Env) StrategyResult {
		t.Fatal("strategies must not run when a session user exists")
		return StrategyResult{}
	}
	p, env := newProxyForTest(t, WithStrategyRun(run), WithDefaultStrategies("x"))
	env["rack.session"] = map[string]any{SessionKeyFor("default"): u}
	if p.Authenticate() != u {
		t.Fatal("expected session user from Authenticate")
	}
}

func TestMultiScopeIndependent(t *testing.T) {
	admin := &user{id: 1, name: "admin"}
	member := &user{id: 2, name: "member"}
	run := func(name string, _ rack.Env) StrategyResult {
		switch name {
		case "adminStrat":
			return StrategyResult{Valid: true, Result: ResultSuccess, User: admin}
		case "memberStrat":
			return StrategyResult{Valid: true, Result: ResultSuccess, User: member}
		}
		return StrategyResult{Valid: false}
	}
	p, _ := newProxyForTest(t,
		WithStrategyRun(run),
		WithScopeStrategies("admin", "adminStrat"),
		WithScopeStrategies("member", "memberStrat"),
	)
	if p.Authenticate(AuthOptions{Scope: "admin"}) != admin {
		t.Fatal("admin scope")
	}
	if p.Authenticate(AuthOptions{Scope: "member"}) != member {
		t.Fatal("member scope")
	}
	if p.User("admin") != admin || p.User("member") != member {
		t.Fatal("scopes must not bleed")
	}
	if p.Result("admin") != ResultSuccess {
		t.Fatalf("admin result = %q", p.Result("admin"))
	}
}

func TestAuthOptionsStrategyOverride(t *testing.T) {
	u := &user{id: 8}
	run := func(name string, _ rack.Env) StrategyResult {
		if name == "override" {
			return StrategyResult{Valid: true, Result: ResultSuccess, User: u}
		}
		return StrategyResult{Valid: true, Result: ResultFailure, Halted: true}
	}
	// No default strategies configured; the call supplies them.
	p, _ := newProxyForTest(t, WithStrategyRun(run))
	if p.Authenticate(AuthOptions{Strategies: []string{"override"}}) != u {
		t.Fatal("expected override strategy to authenticate")
	}
}

func TestRunStrategiesValidSkipPassAndNonBangFail(t *testing.T) {
	u := &user{id: 9}
	// invalidStrat is skipped, passStrat passes, softFail fails without halting,
	// winner succeeds.
	run := func(name string, _ rack.Env) StrategyResult {
		switch name {
		case "invalid":
			return StrategyResult{Valid: false}
		case "pass":
			return StrategyResult{Valid: true, Result: ResultNone}
		case "softfail":
			return StrategyResult{Valid: true, Result: ResultFailure, Message: "soft", Halted: false}
		case "winner":
			return StrategyResult{Valid: true, Result: ResultSuccess, User: u}
		}
		return StrategyResult{Valid: false}
	}
	p, _ := newProxyForTest(t, WithStrategyRun(run),
		WithDefaultStrategies("invalid", "pass", "softfail", "winner"))
	if p.Authenticate() != u {
		t.Fatal("expected winner")
	}
	if p.WinningStrategy() != "winner" {
		t.Fatalf("winning = %q", p.WinningStrategy())
	}
}

func TestRunStrategiesNilRun(t *testing.T) {
	// No StrategyRun wired: authentication fails cleanly.
	p, _ := newProxyForTest(t, WithDefaultStrategies("x"))
	if p.Authenticate() != nil {
		t.Fatal("expected nil user with no strategy runner")
	}
	if p.Message() != "" {
		t.Fatalf("message = %q", p.Message())
	}
}

func TestLogoutSingleScope(t *testing.T) {
	u := &user{id: 1}
	p, env := newProxyForTest(t)
	p.SetUser(u, "default", true)
	if !p.Authenticated() {
		t.Fatal("precondition")
	}
	p.Logout("default")
	if p.Authenticated() {
		t.Fatal("expected logged out")
	}
	sess := env["rack.session"].(map[string]any)
	if _, ok := sess[SessionKeyFor("default")]; ok {
		t.Fatal("expected session key deleted")
	}
	// Session map still present (single-scope logout does not reset).
	if _, ok := env["rack.session"]; !ok {
		t.Fatal("single-scope logout should not reset the session")
	}
}

func TestLogoutAllResets(t *testing.T) {
	p, env := newProxyForTest(t)
	p.SetUser(&user{id: 1}, "admin", true)
	p.SetUser(&user{id: 2}, "member", true)
	p.Logout()
	if p.Authenticated("admin") || p.Authenticated("member") {
		t.Fatal("expected all scopes logged out")
	}
	if _, ok := env["rack.session"]; ok {
		t.Fatal("logout-all should reset the session")
	}
}

func TestScopeDefaulting(t *testing.T) {
	p, _ := newProxyForTest(t, WithDefaultScope("customdefault"))
	if p.scope(nil) != "customdefault" {
		t.Fatalf("scope = %q", p.scope(nil))
	}
	if p.scope([]string{""}) != "customdefault" {
		t.Fatalf("empty scope = %q", p.scope([]string{""}))
	}
	if p.scope([]string{"named"}) != "named" {
		t.Fatalf("named scope = %q", p.scope([]string{"named"}))
	}
}
