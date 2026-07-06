// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

// user is a trivial host user object.
type user struct {
	id   int
	name string
}

// okBody returns the single-string body of a response, for assertions.
func okBody(r *rack.Response) string {
	_, _, body := r.Finish()
	if len(body) == 0 {
		return ""
	}
	return body[0]
}

// appReturning is a downstream app that just returns a 200 with a fixed body.
func appReturning(text string) App {
	return func(rack.Env) *rack.Response { return rack.NewResponseString(text, 200, nil) }
}

func TestManagerPassesThroughWhenNoThrow(t *testing.T) {
	m := New(appReturning("ok"))
	resp := m.Call(rack.Env{})
	if got := okBody(resp); got != "ok" {
		t.Fatalf("body = %q, want ok", got)
	}
}

func TestFromEnvInjectedAndAbsent(t *testing.T) {
	var seen *Proxy
	m := New(func(env rack.Env) *rack.Response {
		seen = FromEnv(env)
		return rack.NewResponseString("x", 200, nil)
	})
	m.Call(rack.Env{})
	if seen == nil {
		t.Fatal("expected proxy injected into env")
	}
	if FromEnv(rack.Env{}) != nil {
		t.Fatal("expected nil proxy for bare env")
	}
}

// success is a StrategyRun seam that authenticates u for the named "good"
// strategy and fails everything else.
func successRun(u any) StrategyRun {
	return func(name string, _ rack.Env) StrategyResult {
		if name == "good" {
			return StrategyResult{Valid: true, Result: ResultSuccess, User: u}
		}
		return StrategyResult{Valid: true, Result: ResultFailure, Halted: true, Message: "nope"}
	}
}

func TestAuthenticateSuccessAndFailure(t *testing.T) {
	u := &user{id: 1, name: "ada"}
	m := New(appReturning("ok"),
		WithStrategyRun(successRun(u)),
		WithDefaultStrategies("good"))
	env := rack.Env{}
	m.Call(env) // inject proxy
	p := FromEnv(env)

	if got := p.Authenticate(); got != u {
		t.Fatalf("Authenticate = %v, want %v", got, u)
	}
	if !p.Authenticated() {
		t.Fatal("expected authenticated")
	}
	if p.WinningStrategy() != "good" {
		t.Fatalf("winning = %q", p.WinningStrategy())
	}

	// A fresh proxy with a failing-only strategy list.
	m2 := New(appReturning("ok"),
		WithStrategyRun(successRun(u)),
		WithDefaultStrategies("bad"))
	env2 := rack.Env{}
	m2.Call(env2)
	p2 := FromEnv(env2)
	if p2.Authenticate() != nil {
		t.Fatal("expected failed authenticate to return nil")
	}
	if p2.Unauthenticated() != true {
		t.Fatal("expected unauthenticated")
	}
	if p2.Message() != "nope" {
		t.Fatalf("message = %q", p2.Message())
	}
	if p2.Result() != ResultFailure {
		t.Fatalf("result = %q", p2.Result())
	}
}

func TestAuthenticateBangThrowsRunsFailureApp(t *testing.T) {
	u := &user{id: 1}
	m := New(
		func(env rack.Env) *rack.Response {
			FromEnv(env).AuthenticateBang()
			return rack.NewResponseString("unreached", 200, nil)
		},
		WithStrategyRun(successRun(u)),
		WithDefaultStrategies("bad"),
		WithFailureApp(func(env rack.Env) *rack.Response {
			if env["PATH_INFO"] != "/unauthenticated" {
				t.Fatalf("PATH_INFO = %v", env["PATH_INFO"])
			}
			if _, ok := env["warden.options"].(ThrowOptions); !ok {
				t.Fatal("expected warden.options")
			}
			return rack.NewResponseString("denied", 401, nil)
		}),
	)
	resp := m.Call(rack.Env{})
	if got := okBody(resp); got != "denied" {
		t.Fatalf("body = %q, want denied", got)
	}
}

func TestAuthenticateBangSucceeds(t *testing.T) {
	u := &user{id: 7}
	m := New(
		func(env rack.Env) *rack.Response {
			got := FromEnv(env).AuthenticateBang()
			if got != u {
				t.Fatalf("bang = %v", got)
			}
			return rack.NewResponseString("in", 200, nil)
		},
		WithStrategyRun(successRun(u)),
		WithDefaultStrategies("good"),
	)
	if got := okBody(m.Call(rack.Env{})); got != "in" {
		t.Fatalf("body = %q", got)
	}
}

func TestNoFailureAppPanicsNotAuthenticated(t *testing.T) {
	m := New(
		func(env rack.Env) *rack.Response {
			FromEnv(env).AuthenticateBang()
			return nil
		},
		WithStrategyRun(successRun(&user{})),
		WithDefaultStrategies("bad"),
	)
	defer func() {
		r := recover()
		na, ok := r.(*NotAuthenticated)
		if !ok {
			t.Fatalf("recovered %T, want *NotAuthenticated", r)
		}
		if na.Error() != "warden: not authenticated: nope" {
			t.Fatalf("error = %q", na.Error())
		}
	}()
	m.Call(rack.Env{})
}

func TestNotAuthenticatedErrorNoMessage(t *testing.T) {
	e := &NotAuthenticated{}
	if e.Error() != "warden: not authenticated" {
		t.Fatalf("error = %q", e.Error())
	}
}

func TestThrowError(t *testing.T) {
	tr := &Throw{Options: ThrowOptions{Action: "unauthenticated"}}
	if tr.Error() != "warden: throw :warden (unauthenticated)" {
		t.Fatalf("error = %q", tr.Error())
	}
}

func TestCatchRepanicsNonThrow(t *testing.T) {
	m := New(func(rack.Env) *rack.Response { panic("boom") })
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("recovered %v, want boom", r)
		}
	}()
	m.Call(rack.Env{})
}
