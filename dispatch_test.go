// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

// bangApp is a downstream app that calls authenticate! and never returns.
func bangApp() App {
	return func(env rack.Env) *rack.Response {
		FromEnv(env).AuthenticateBang()
		return rack.NewResponseString("unreached", 200, nil)
	}
}

func TestRedirectResultReturnsResponse(t *testing.T) {
	redirect := rack.NewResponseString("go there", 302, nil)
	run := func(string, rack.Env) StrategyResult {
		return StrategyResult{Valid: true, Result: ResultRedirect, Halted: true, Response: redirect}
	}
	m := New(bangApp(), WithStrategyRun(run), WithDefaultStrategies("s"))
	if got := okBody(m.Call(rack.Env{})); got != "go there" {
		t.Fatalf("body = %q", got)
	}
}

func TestCustomResultReturnsResponse(t *testing.T) {
	custom := rack.NewResponseString("teapot", 418, nil)
	run := func(string, rack.Env) StrategyResult {
		return StrategyResult{Valid: true, Result: ResultCustom, Halted: true, Response: custom}
	}
	m := New(bangApp(), WithStrategyRun(run), WithDefaultStrategies("s"))
	if got := okBody(m.Call(rack.Env{})); got != "teapot" {
		t.Fatalf("body = %q", got)
	}
}

func TestRedirectWithNilResponseFallsToFailureApp(t *testing.T) {
	// Halted redirect but no response carried -> failure app runs.
	run := func(string, rack.Env) StrategyResult {
		return StrategyResult{Valid: true, Result: ResultRedirect, Halted: true}
	}
	m := New(bangApp(),
		WithStrategyRun(run),
		WithDefaultStrategies("s"),
		WithFailureApp(func(rack.Env) *rack.Response { return rack.NewResponseString("fallback", 401, nil) }),
	)
	if got := okBody(m.Call(rack.Env{})); got != "fallback" {
		t.Fatalf("body = %q", got)
	}
}

func TestIntercept401RunsFailureApp(t *testing.T) {
	// App returns 401 without throwing; intercept_401 routes to the failure app.
	m := New(
		func(rack.Env) *rack.Response { return rack.NewResponseString("raw401", 401, nil) },
		WithIntercept401(),
		WithFailureApp(func(env rack.Env) *rack.Response {
			// intercept passes empty options; action defaults to unauthenticated.
			if env["PATH_INFO"] != "/unauthenticated" {
				t.Fatalf("PATH_INFO = %v", env["PATH_INFO"])
			}
			return rack.NewResponseString("intercepted", 401, nil)
		}),
	)
	if got := okBody(m.Call(rack.Env{})); got != "intercepted" {
		t.Fatalf("body = %q", got)
	}
}

func TestIntercept401IgnoresNon401(t *testing.T) {
	m := New(appReturning("fine"), WithIntercept401(),
		WithFailureApp(func(rack.Env) *rack.Response { return rack.NewResponseString("no", 401, nil) }))
	if got := okBody(m.Call(rack.Env{})); got != "fine" {
		t.Fatalf("body = %q", got)
	}
}

func TestFailureAppReceivesCustomAction(t *testing.T) {
	// A strategy that fails without halting leaves result empty; the failure app
	// still runs and PATH_INFO uses the thrown action.
	run := func(string, rack.Env) StrategyResult {
		return StrategyResult{Valid: true, Result: ResultFailure, Halted: true, Message: "bad creds"}
	}
	m := New(bangApp(),
		WithStrategyRun(run),
		WithDefaultStrategies("s"),
		WithFailureApp(func(env rack.Env) *rack.Response {
			opts := env["warden.options"].(ThrowOptions)
			if opts.Message != "bad creds" {
				t.Fatalf("opts.Message = %q", opts.Message)
			}
			if opts.Result != ResultFailure {
				t.Fatalf("opts.Result = %q", opts.Result)
			}
			return rack.NewResponseString("failed", 401, nil)
		}),
	)
	if got := okBody(m.Call(rack.Env{})); got != "failed" {
		t.Fatalf("body = %q", got)
	}
}
