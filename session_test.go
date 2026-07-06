// Copyright (c) the go-ruby-warden/warden authors
//
// SPDX-License-Identifier: BSD-3-Clause

package warden

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

func TestSessionKeyFor(t *testing.T) {
	if got := SessionKeyFor("admin"); got != "warden.user.admin.key" {
		t.Fatalf("key = %q", got)
	}
}

func TestSerializerStoreRoundTrip(t *testing.T) {
	// A store that serializes to the user id and deserializes via a registry.
	registry := map[int]*user{1: {id: 1, name: "ada"}}
	store := &SerializerStore{
		Serialize: func(u any) any { return u.(*user).id },
		Deserialize: func(k any) (any, bool) {
			u, ok := registry[k.(int)]
			return u, ok
		},
	}
	env := rack.Env{}
	store.SerializeIntoSession(env, "default", registry[1])

	sess := env["rack.session"].(map[string]any)
	if sess[SessionKeyFor("default")] != 1 {
		t.Fatalf("stored key = %v, want id 1", sess[SessionKeyFor("default")])
	}

	got, present := store.SerializeFromSession(env, "default")
	if !present || got.(*user).name != "ada" {
		t.Fatalf("round-trip = %v present=%v", got, present)
	}
}

func TestSerializerStoreDeserializeMiss(t *testing.T) {
	store := &SerializerStore{
		Serialize:   func(u any) any { return u },
		Deserialize: func(any) (any, bool) { return nil, false }, // key no longer resolves
	}
	env := rack.Env{}
	store.SerializeIntoSession(env, "default", &user{id: 1})
	if _, present := store.SerializeFromSession(env, "default"); present {
		t.Fatal("expected present=false when deserialize misses")
	}
}

func TestSerializeFromSessionNoSessionAndNoKey(t *testing.T) {
	store := NewSerializerStore()
	// No session at all.
	if _, present := store.SerializeFromSession(rack.Env{}, "default"); present {
		t.Fatal("expected no user with no session")
	}
	// Session present but missing this scope's key.
	env := rack.Env{"rack.session": map[string]any{}}
	if _, present := store.SerializeFromSession(env, "default"); present {
		t.Fatal("expected no user when key absent")
	}
}

func TestSerializerStoreDeleteAndReset(t *testing.T) {
	store := NewSerializerStore()
	// Delete with no session is a no-op.
	store.Delete(rack.Env{}, "default")

	env := rack.Env{}
	store.SerializeIntoSession(env, "default", &user{id: 1})
	store.SerializeIntoSession(env, "other", &user{id: 2})
	store.Delete(env, "default")
	sess := env["rack.session"].(map[string]any)
	if _, ok := sess[SessionKeyFor("default")]; ok {
		t.Fatal("expected default key deleted")
	}
	if _, ok := sess[SessionKeyFor("other")]; !ok {
		t.Fatal("expected other key intact")
	}
	store.Reset(env)
	if _, ok := env["rack.session"]; ok {
		t.Fatal("expected session reset")
	}
}

func TestSerializerStoreCustomEnvKey(t *testing.T) {
	store := &SerializerStore{
		Serialize:     func(u any) any { return u },
		Deserialize:   func(k any) (any, bool) { return k, true },
		SessionEnvKey: "my.session",
	}
	u := &user{id: 1}
	env := rack.Env{}
	store.SerializeIntoSession(env, "default", u)
	if _, ok := env["my.session"]; !ok {
		t.Fatal("expected custom session env key used")
	}
	got, present := store.SerializeFromSession(env, "default")
	if !present || got != u {
		t.Fatalf("round-trip on custom key: %v %v", got, present)
	}
}

// TestSessionStoreSeamInjected verifies a wholly custom SessionStore drives the
// proxy's persistence.
func TestSessionStoreSeamInjected(t *testing.T) {
	fs := &fakeStore{data: map[string]any{}}
	u := &user{id: 42}
	p, _ := newProxyForTest(t, WithSessionStore(fs))
	p.SetUser(u, "default", true)
	if fs.data["default"] != u {
		t.Fatal("expected custom store to receive the user")
	}
	p.Logout("default")
	if _, ok := fs.data["default"]; ok {
		t.Fatal("expected custom store delete")
	}
}

// fakeStore is a minimal SessionStore used to prove the seam.
type fakeStore struct {
	data     map[string]any
	resetHit bool
}

func (f *fakeStore) SerializeIntoSession(_ rack.Env, scope string, user any) {
	f.data[scope] = user
}
func (f *fakeStore) SerializeFromSession(_ rack.Env, scope string) (any, bool) {
	u, ok := f.data[scope]
	return u, ok
}
func (f *fakeStore) Delete(_ rack.Env, scope string) { delete(f.data, scope) }
func (f *fakeStore) Reset(rack.Env)                  { f.resetHit = true }
