<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-warden/brand/main/social/go-ruby-warden-warden.png" alt="go-ruby-warden/warden" width="720"></p>

# warden — go-ruby-warden

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-warden.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) model of the engine of Ruby's [Warden](https://github.com/wardencommunity/warden)** —
the Rack authentication middleware. It mirrors Warden's observable control flow —
the `env['warden']` proxy, scope-aware authentication, session-cached users,
pluggable strategies, and the `throw :warden` → failure-app dispatch — **without
any Ruby runtime**.

Warden is not an authentication *system*; it is the plumbing that lets a Rack app
run pluggable strategies, cache the authenticated user per scope, and hand control
to a failure application when authentication is required but unmet. This package
models exactly that plumbing — the deterministic, interpreter-independent parts —
and leaves the Ruby-defined pieces (the strategy bodies and the session) behind
**seams**.

It is the Warden backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module — a sibling of
[go-ruby-rack](https://github.com/go-ruby-rack/rack) (whose `Env` and `Response`
it reuses), [go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) and
[go-ruby-erb](https://github.com/go-ruby-erb/erb).

## Fidelity basis

Modeled on the observable behaviour of the `warden` gem (the `wardencommunity/warden`
Rack middleware, as bundled with MRI-4.0.5-era Rack apps):

- **`Warden::Manager`** — the Rack middleware. `call(env)` injects the proxy as
  `env['warden']`, runs the downstream app inside `catch(:warden)`, and on a throw
  dispatches to a redirect / custom response or the `failure_app`.
- **`Warden::Proxy`** — the `env['warden']` object: `authenticate` / `authenticate!`,
  `authenticated?`, `user` / `set_user`, `logout`, `winning_strategy`, `message`,
  scope-aware over `:default` and custom scopes.
- **`Warden::Strategies::Base`** — each strategy's `valid?` predicate and
  `authenticate!` body, producing `success!` / `fail!` / `fail` / `redirect!` /
  `custom!` / `pass`.
- **`Warden::SessionSerializer`** — `serialize_into_session` /
  `serialize_from_session`, keying the user under `warden.user.<scope>.key` in the
  Rack session.
- **`throw :warden`** and **`Warden::NotAuthenticated`** control flow.

The strategy bodies and the (de)serialization of a user are Ruby, so they are
**injectable seams**, not baked in — see below.

## The two seams

Everything interpreter-specific is injected, so the package stays free of any Ruby
runtime:

```go
// StrategyRun runs one registered strategy by label (valid? + authenticate!)
// and returns its outcome. A binding wires this to the host's strategy registry.
type StrategyRun func(name string, env rack.Env) StrategyResult

// SessionStore is serialize_into_session / serialize_from_session. The default
// SerializerStore stores the user key under "warden.user.<scope>.key" in
// env["rack.session"]; Serialize / Deserialize map a user to/from a storable key.
type SessionStore interface {
	SerializeIntoSession(env rack.Env, scope string, user any)
	SerializeFromSession(env rack.Env, scope string) (user any, present bool)
	Delete(env rack.Env, scope string)
	Reset(env rack.Env)
}
```

## Install

```sh
go get github.com/go-ruby-warden/warden
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-rack/rack"
	"github.com/go-ruby-warden/warden"
)

type user struct{ id int; name string }

func main() {
	// A strategy that authenticates when the env carries the right token.
	run := func(name string, env rack.Env) warden.StrategyResult {
		if name == "token" && env["HTTP_AUTHORIZATION"] == "secret" {
			return warden.StrategyResult{Valid: true, Result: warden.ResultSuccess,
				User: &user{id: 1, name: "ada"}}
		}
		return warden.StrategyResult{Valid: true, Result: warden.ResultFailure,
			Halted: true, Message: "bad token"}
	}

	// The protected app calls authenticate! (which throws :warden on failure).
	app := func(env rack.Env) *rack.Response {
		u := warden.FromEnv(env).AuthenticateBang().(*user)
		return rack.NewResponseString("hello "+u.name, 200, nil)
	}

	// The failure app runs when authentication is thrown.
	failure := func(env rack.Env) *rack.Response {
		opts := env["warden.options"].(warden.ThrowOptions)
		return rack.NewResponseString("denied: "+opts.Message, 401, nil)
	}

	m := warden.New(app,
		warden.WithStrategyRun(run),
		warden.WithDefaultStrategies("token"),
		warden.WithFailureApp(failure),
	)

	ok := m.Call(rack.Env{"HTTP_AUTHORIZATION": "secret"})
	_, _, body := ok.Finish()
	fmt.Println(body[0]) // hello ada

	no := m.Call(rack.Env{})
	_, _, body = no.Finish()
	fmt.Println(body[0]) // denied: bad token
}
```

## API

```go
// middleware
type App func(env rack.Env) *rack.Response
func New(app App, opts ...Option) *Manager
func (m *Manager) Call(env rack.Env) *rack.Response

// options (Warden::Config)
func WithFailureApp(app App) Option
func WithStrategyRun(run StrategyRun) Option
func WithSessionStore(store SessionStore) Option
func WithDefaultStrategies(names ...string) Option
func WithScopeStrategies(scope string, names ...string) Option
func WithDefaultScope(scope string) Option
func WithIntercept401() Option

// the env['warden'] object
func FromEnv(env rack.Env) *Proxy
func (p *Proxy) Authenticate(opts ...AuthOptions) any      // authenticate
func (p *Proxy) AuthenticateBang(opts ...AuthOptions) any  // authenticate! (throws)
func (p *Proxy) Authenticated(scope ...string) bool        // authenticated?
func (p *Proxy) Unauthenticated(scope ...string) bool      // unauthenticated?
func (p *Proxy) User(scope ...string) any                  // user
func (p *Proxy) SetUser(user any, scope string, store bool) any // set_user
func (p *Proxy) Logout(scopes ...string)                   // logout
func (p *Proxy) WinningStrategy() string                   // winning_strategy
func (p *Proxy) Message() string                           // message
func (p *Proxy) Result(scope ...string) Result

// strategies
type Result string // ResultNone/Success/Failure/Redirect/Custom
type StrategyResult struct { Valid bool; Result Result; User any; Message string; Halted bool; Response *rack.Response }
type StrategyRun func(name string, env rack.Env) StrategyResult

// session
type SessionStore interface { /* serialize_into_session / serialize_from_session */ }
type SerializerStore struct { Serialize func(any) any; Deserialize func(any) (any, bool); SessionEnvKey string }
func NewSerializerStore() *SerializerStore
func SessionKeyFor(scope string) string // "warden.user.<scope>.key"

// control flow
type Throw struct { Options ThrowOptions }           // throw :warden
type NotAuthenticated struct { Scope, Message string } // Warden::NotAuthenticated
```

`AuthenticateBang` panics with a `*Throw` (Ruby's `throw :warden`), which
`Manager.Call` recovers and turns into a failure response — a redirect / custom
response when the winning strategy produced one, otherwise the failure app. With
no failure app configured it panics with a `*NotAuthenticated` (the gem's
"No Failure App" raise). A binding maps these to Ruby's `throw`/`catch` and
`Warden::NotAuthenticated`.

## Tests & coverage

Deterministic, Ruby-free tests drive every engine path — auth success / failure /
`throw`, multi-scope isolation, `set_user` / `logout`, the serialize round-trip,
`redirect!` / `custom!` dispatch, `intercept_401`, and the missing-failure-app
raise — through fake strategy and session seams, holding coverage at **100%**
(so the qemu cross-arch and Windows lanes pass the gate).

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

CGO-free, `gofmt` + `go vet` clean, and green across the six 64-bit Go targets
(amd64, arm64, riscv64, loong64, ppc64le, s390x) and three OSes (Linux, macOS,
Windows). Its one dependency is the pure-Go
[go-ruby-rack](https://github.com/go-ruby-rack/rack), for the `Env` and `Response`
value types.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-warden/warden authors.
