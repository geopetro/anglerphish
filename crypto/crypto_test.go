package crypto

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func newKey(t *testing.T) string {
	t.Helper()
	k, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestEncryptPassthroughWhenOff(t *testing.T) {
	t.Cleanup(Unload)
	Unload()
	got, err := Encrypt("x")
	if err != nil || got != "x" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRoundTripWhenOn(t *testing.T) {
	t.Cleanup(Unload)
	if err := LoadKey(newKey(t), SourceSession); err != nil {
		t.Fatal(err)
	}
	c, err := Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c, "ENC:v1:") {
		t.Fatalf("missing prefix: %q", c)
	}
	p, err := Decrypt(c)
	if err != nil || p != "secret" {
		t.Fatalf("got %q, %v", p, err)
	}
	if CurrentSource() != SourceSession {
		t.Fatalf("source = %q", CurrentSource())
	}
	if CurrentMode() != ModeOn || !IsEncryptionEnabled() {
		t.Fatal("expected ModeOn")
	}
}

func TestDecryptOnlyMode(t *testing.T) {
	t.Cleanup(Unload)
	if err := LoadKey(newKey(t), SourceSession); err != nil {
		t.Fatal(err)
	}
	c, _ := Encrypt("secret")
	SetDecryptOnly()
	if got, _ := Encrypt("new"); got != "new" {
		t.Fatalf("Encrypt in decrypt-only = %q", got)
	}
	if p, err := Decrypt(c); err != nil || p != "secret" {
		t.Fatalf("got %q, %v", p, err)
	}
	if IsEncryptionEnabled() {
		t.Fatal("IsEncryptionEnabled should be false")
	}
	if !HasKey() {
		t.Fatal("HasKey should be true")
	}
	if CurrentMode() != ModeDecryptOnly {
		t.Fatal("expected ModeDecryptOnly")
	}
}

func TestUnloadDropsKey(t *testing.T) {
	t.Cleanup(Unload)
	if err := LoadKey(newKey(t), SourceEnv); err != nil {
		t.Fatal(err)
	}
	c, _ := Encrypt("secret")
	Unload()
	if _, err := Decrypt(c); !errors.Is(err, ErrNoKeyConfigured) {
		t.Fatalf("err = %v", err)
	}
	if CurrentMode() != ModeOff || CurrentSource() != SourceNone || HasKey() {
		t.Fatal("expected cleared state")
	}
}

func TestLoadKeyRejectsBadFormat(t *testing.T) {
	t.Cleanup(Unload)
	Unload()
	for _, k := range []string{"short", strings.Repeat("!", 44)} {
		if err := LoadKey(k, SourceSession); !errors.Is(err, ErrInvalidKeyLength) {
			t.Fatalf("key %q: err = %v", k, err)
		}
		if CurrentMode() != ModeOff {
			t.Fatal("mode changed on bad key")
		}
	}
}

func TestDecryptWithKey(t *testing.T) {
	t.Cleanup(Unload)
	a, b := newKey(t), newKey(t)
	if err := LoadKey(a, SourceSession); err != nil {
		t.Fatal(err)
	}
	c, _ := Encrypt("secret")
	Unload()
	if p, err := DecryptWithKey(c, a); err != nil || p != "secret" {
		t.Fatalf("got %q, %v", p, err)
	}
	if _, err := DecryptWithKey(c, b); !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("err = %v", err)
	}
	if HasKey() {
		t.Fatal("DecryptWithKey must not load the key")
	}
}

func TestConcurrentLoadAndEncrypt(t *testing.T) {
	t.Cleanup(Unload)
	k := newKey(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c, _ := Encrypt("v")
				_, _ = Decrypt(c)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			_ = LoadKey(k, SourceSession)
			SetDecryptOnly()
			Unload()
		}
	}()
	wg.Wait()
}
