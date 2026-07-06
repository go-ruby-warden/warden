// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package warden is a pure-Go (no cgo) model of the engine of Ruby's
// [Warden] Rack authentication middleware, faithful to the observable behaviour
// of the `warden` gem (as bundled with MRI-4.0.5-era Rack apps).
//
// Warden is not an authentication *system*; it is the plumbing that lets a Rack
// app run pluggable authentication strategies, cache the authenticated user per
// scope in the session, and hand control to a failure application when
// authentication is required but not satisfied. This package models that
// plumbing — the deterministic, interpreter-independent control flow — while
// leaving the Ruby-defined pieces behind seams:
//
//   - [Manager] is the Rack middleware. It wraps a downstream [App], injects a
//     [Proxy] as env["warden"], runs the app inside a catch for the throw
//     :warden control-flow signal, and dispatches to the failure app / redirect
//     / custom response when authentication was thrown.
//   - [Proxy] is the env["warden"] object: Authenticate / AuthenticateBang,
//     Authenticated, User / SetUser, Logout, WinningStrategy, Message. It is
//     scope-aware ("default" plus any custom scope).
//   - The strategy bodies (valid? + authenticate!) are Ruby, so they live
//     behind the injectable [StrategyRun] seam, which returns a [StrategyResult]
//     (success / failure / redirect / custom / pass, and whether it halted the
//     chain).
//   - The session is a Rack concern, so serialize_into_session /
//     serialize_from_session go through the [SessionStore] seam; a faithful
//     default, [SerializerStore], stores the user key under
//     "warden.user.<scope>.key" in env["rack.session"].
//
// The Rack environment and response tuple are reused from the sibling module
// [github.com/go-ruby-rack/rack]: an environment is a [rack.Env]
// (a string-keyed map) and an app returns a *[rack.Response] — the SPEC
// [status, headers, body] tuple.
//
// throw :warden is modeled as a panic of *[Throw] recovered by [Manager.Call];
// [NotAuthenticated] models the error a binding raises when authentication is
// required but no failure app is configured (the gem's "No Failure App" raise).
//
// The package is the Warden backend for go-embedded-ruby, but is a standalone,
// reusable module — a sibling of go-ruby-rack, go-ruby-regexp and go-ruby-erb.
//
// [Warden]: https://github.com/wardencommunity/warden
package warden
