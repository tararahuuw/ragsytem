package ratelimit

import "testing"

func TestAllow_BlocksAfterLimit(t *testing.T) {
	l := New(Config{Enabled: true, PerMinute: map[string]int{"chat": 3}})
	// burst = limit (3): first 3 allowed, 4th blocked (refill is slow).
	for i := 0; i < 3; i++ {
		if !l.Allow("chat", "user:1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.Allow("chat", "user:1") {
		t.Fatal("4th request should be blocked")
	}
}

func TestAllow_KeysAreIndependent(t *testing.T) {
	l := New(Config{Enabled: true, PerMinute: map[string]int{"chat": 1}})
	if !l.Allow("chat", "user:1") {
		t.Fatal("user:1 first request should pass")
	}
	if !l.Allow("chat", "user:2") {
		t.Fatal("user:2 must have its own bucket")
	}
	if l.Allow("chat", "user:1") {
		t.Fatal("user:1 second request should be blocked")
	}
}

func TestAllow_CategoriesAreIndependent(t *testing.T) {
	l := New(Config{Enabled: true, PerMinute: map[string]int{"auth": 1, "chat": 1}})
	if !l.Allow("auth", "ip:x") || !l.Allow("chat", "ip:x") {
		t.Fatal("same key, different categories must not share a bucket")
	}
}

func TestAllow_DisabledOrUnlimited(t *testing.T) {
	off := New(Config{Enabled: false, PerMinute: map[string]int{"chat": 1}})
	for i := 0; i < 100; i++ {
		if !off.Allow("chat", "k") {
			t.Fatal("disabled limiter must always allow")
		}
	}
	// category with no limit and no default => unlimited
	nolimit := New(Config{Enabled: true, PerMinute: map[string]int{}})
	for i := 0; i < 100; i++ {
		if !nolimit.Allow("unknown", "k") {
			t.Fatal("uncategorized (no default) must be unlimited")
		}
	}
}
