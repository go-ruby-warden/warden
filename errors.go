// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

// ThrowOptions carries the payload of Warden's throw(:warden, opts) — the
// control-flow signal raised when AuthenticateBang (authenticate!) fails. A
// binding maps a Ruby throw :warden to a panic of *[Throw]; [Manager.Call]
// recovers it and turns it into a failure response.
type ThrowOptions struct {
	// Scope is the authentication scope that failed ("default" or a custom one).
	Scope string
	// Action names the failure action; it becomes PATH_INFO ("/unauthenticated"
	// by default) when the failure app runs.
	Action string
	// Message is the winning strategy's failure message, if any.
	Message string
	// Result is the winning strategy's result (failure / redirect / custom), or
	// empty when no strategy halted.
	Result Result
	// Strategy is the label of the winning (last-run) strategy, if any.
	Strategy string
}

// Throw is the Go analogue of Ruby's throw :warden. [Proxy.AuthenticateBang]
// panics with a *Throw when authentication fails; [Manager.Call] recovers it.
type Throw struct {
	Options ThrowOptions
}

// Error implements error so a *Throw can also be surfaced as one by a binding.
func (t *Throw) Error() string { return "warden: throw :warden (" + t.Options.Action + ")" }

// NotAuthenticated models Warden::NotAuthenticated — the error a binding raises
// when authentication was required (a throw :warden reached the failure stage)
// but no failure app is configured. It mirrors the gem's "No Failure App"
// raise. [Manager.Call] panics with a *NotAuthenticated in that case.
type NotAuthenticated struct {
	Scope   string
	Message string
}

// Error implements error.
func (e *NotAuthenticated) Error() string {
	if e.Message != "" {
		return "warden: not authenticated: " + e.Message
	}
	return "warden: not authenticated"
}
