// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

import "github.com/go-ruby-rack/rack"

// DefaultSessionEnvKey is the Rack env key under which the session hash lives,
// matching Rack's "rack.session".
const DefaultSessionEnvKey = "rack.session"

// SessionKeyFor returns the session key under which Warden stores the
// serialized user for a scope, matching the gem's
// "warden.user.#{scope}.key".
func SessionKeyFor(scope string) string { return "warden.user." + scope + ".key" }

// SessionStore is the seam through which the Proxy persists the authenticated
// user across requests — the serialize_into_session / serialize_from_session
// pair. It is a seam because the session and the (de)serialization of a user to
// a storable key are host concerns.
type SessionStore interface {
	// SerializeIntoSession stores the serialized key for user under scope
	// (serialize_into_session).
	SerializeIntoSession(env rack.Env, scope string, user any)
	// SerializeFromSession returns the user stored for scope and whether one is
	// present (serialize_from_session).
	SerializeFromSession(env rack.Env, scope string) (user any, present bool)
	// Delete removes the stored user for scope (logout of a single scope).
	Delete(env rack.Env, scope string)
	// Reset clears the whole session (logout of every scope / reset_session!).
	Reset(env rack.Env)
}

// SerializerStore is the faithful default [SessionStore]. It stores the
// serialized user key under [SessionKeyFor] in the Rack session hash held at
// SessionEnvKey in the env. Serialize maps a user to a storable key and
// Deserialize maps a stored key back to a user (returning present=false when the
// key no longer resolves) — exactly the SessionSerializer#serialize /
// #deserialize seam of the gem. [NewSerializerStore] wires identity functions,
// suitable when the user object is itself directly storable.
type SerializerStore struct {
	// Serialize maps a user to the value stored in the session.
	Serialize func(user any) any
	// Deserialize maps a stored value back to a user; present reports whether it
	// still resolves.
	Deserialize func(key any) (user any, present bool)
	// SessionEnvKey overrides [DefaultSessionEnvKey] when non-empty.
	SessionEnvKey string
}

// NewSerializerStore returns a SerializerStore with identity serialize /
// deserialize functions, storing the user object itself in the session.
func NewSerializerStore() *SerializerStore {
	return &SerializerStore{
		Serialize:   func(u any) any { return u },
		Deserialize: func(k any) (any, bool) { return k, true },
	}
}

func (s *SerializerStore) envKey() string {
	if s.SessionEnvKey != "" {
		return s.SessionEnvKey
	}
	return DefaultSessionEnvKey
}

// session returns the session hash, creating it when create is true.
func (s *SerializerStore) session(env rack.Env, create bool) map[string]any {
	if m, ok := env[s.envKey()].(map[string]any); ok {
		return m
	}
	if !create {
		return nil
	}
	m := map[string]any{}
	env[s.envKey()] = m
	return m
}

// SerializeIntoSession implements [SessionStore].
func (s *SerializerStore) SerializeIntoSession(env rack.Env, scope string, user any) {
	s.session(env, true)[SessionKeyFor(scope)] = s.Serialize(user)
}

// SerializeFromSession implements [SessionStore].
func (s *SerializerStore) SerializeFromSession(env rack.Env, scope string) (any, bool) {
	sess := s.session(env, false)
	if sess == nil {
		return nil, false
	}
	key, ok := sess[SessionKeyFor(scope)]
	if !ok {
		return nil, false
	}
	return s.Deserialize(key)
}

// Delete implements [SessionStore].
func (s *SerializerStore) Delete(env rack.Env, scope string) {
	sess := s.session(env, false)
	if sess == nil {
		return
	}
	delete(sess, SessionKeyFor(scope))
}

// Reset implements [SessionStore].
func (s *SerializerStore) Reset(env rack.Env) { delete(env, s.envKey()) }
