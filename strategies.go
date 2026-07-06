// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

import "github.com/go-ruby-rack/rack"

// Result is the outcome kind a strategy produces, mirroring the result symbols
// of Warden::Strategies::Base (:success, :failure, :redirect, :custom, or none).
type Result string

const (
	// ResultNone is produced by pass — no decision, try the next strategy.
	ResultNone Result = ""
	// ResultSuccess is produced by success!(user) — halts with a user.
	ResultSuccess Result = "success"
	// ResultFailure is produced by fail!/fail — a failed attempt.
	ResultFailure Result = "failure"
	// ResultRedirect is produced by redirect! — halts with a redirect response.
	ResultRedirect Result = "redirect"
	// ResultCustom is produced by custom!(response) — halts with a raw response.
	ResultCustom Result = "custom"
)

// StrategyResult is the outcome of running one strategy — the observable state
// of a Warden::Strategies::Base after its valid? check and authenticate! body.
type StrategyResult struct {
	// Valid reports the strategy's valid? predicate. When false the strategy is
	// skipped and the rest of the fields are ignored.
	Valid bool
	// Result is the outcome kind. ResultNone (from pass) or a non-halting
	// ResultFailure (from fail, without a bang) let the runner continue.
	Result Result
	// User is the authenticated user, set on ResultSuccess.
	User any
	// Message is the failure / status message (fail!/fail/redirect!).
	Message string
	// Halted reports whether the strategy halted the chain. success!, fail!,
	// redirect! and custom! halt; pass and the non-bang fail do not.
	Halted bool
	// Response is the raw Rack response carried by redirect! / custom!.
	Response *rack.Response
}

// StrategyRun is the injectable seam that runs a single registered strategy by
// label against the Rack env and returns its [StrategyResult]. It stands in for
// Warden::Strategies[label].new(env,scope) followed by valid? and authenticate!,
// whose bodies are Ruby. A binding wires this to the host's strategy registry.
type StrategyRun func(name string, env rack.Env) StrategyResult
