package accounts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return store
}

func TestAddGetListAndEncryptionAtRest(t *testing.T) {
	store := newTestStore(t)

	added, err := store.Add(Account{
		Provider: "deepseek",
		Label:    "work",
		Email:    "work@example.com",
		APIKey:   "sk-super-secret",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if added.ID == "" {
		t.Fatal("expected generated id")
	}

	got, err := store.Get(added.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.APIKey != "sk-super-secret" {
		t.Fatalf("APIKey = %q", got.APIKey)
	}
	if got.Provider != "deepseek" || got.Label != "work" {
		t.Fatalf("unexpected account: %+v", got)
	}

	list, err := store.List("deepseek")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List len = %d", len(list))
	}
	if other, _ := store.List("openai"); len(other) != 0 {
		t.Fatalf("expected no openai accounts, got %d", len(other))
	}

	detail, err := os.ReadFile(filepath.Join(store.Dir(), accountsDir, added.ID+".json"))
	if err != nil {
		t.Fatalf("read detail: %v", err)
	}
	if strings.Contains(string(detail), "sk-super-secret") {
		t.Fatal("API key must not be stored in plaintext")
	}
	index, err := os.ReadFile(filepath.Join(store.Dir(), indexFile))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if strings.Contains(string(index), "sk-super-secret") {
		t.Fatal("API key must not appear in the index")
	}
}

func TestNextRotationAndDisabled(t *testing.T) {
	store := newTestStore(t)

	a, err := store.Add(Account{Provider: "deepseek", Label: "a", APIKey: "key-a"})
	if err != nil {
		t.Fatalf("Add a: %v", err)
	}
	b, err := store.Add(Account{Provider: "deepseek", Label: "b", APIKey: "key-b"})
	if err != nil {
		t.Fatalf("Add b: %v", err)
	}

	first, err := store.Next("deepseek")
	if err != nil {
		t.Fatalf("Next first: %v", err)
	}
	second, err := store.Next("deepseek")
	if err != nil {
		t.Fatalf("Next second: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected rotation, got same account %q twice", first.ID)
	}

	if err := store.SetDisabled(a.ID, true, "test"); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}
	third, err := store.Next("deepseek")
	if err != nil {
		t.Fatalf("Next third: %v", err)
	}
	if third.ID != b.ID {
		t.Fatalf("expected account b (%s), got %s", b.ID, third.ID)
	}

	if err := store.SetDisabled(b.ID, true, "test"); err != nil {
		t.Fatalf("SetDisabled b: %v", err)
	}
	if _, err := store.Next("deepseek"); !errors.Is(err, ErrNoAccounts) {
		t.Fatalf("expected ErrNoAccounts, got %v", err)
	}
}

func TestReportErrorDisablesAndCoolsDown(t *testing.T) {
	store := newTestStore(t)

	bad, err := store.Add(Account{Provider: "deepseek", Label: "bad", APIKey: "k1"})
	if err != nil {
		t.Fatalf("Add bad: %v", err)
	}
	if err := store.ReportError(bad.ID, 401, "invalid key"); err != nil {
		t.Fatalf("ReportError 401: %v", err)
	}
	got, err := store.Get(bad.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Disabled {
		t.Fatal("expected account to be disabled after 401")
	}
	if got.DisabledReason != "invalid key" {
		t.Fatalf("reason = %q", got.DisabledReason)
	}

	limited, err := store.Add(Account{Provider: "deepseek", Label: "limited", APIKey: "k2"})
	if err != nil {
		t.Fatalf("Add limited: %v", err)
	}
	if err := store.ReportError(limited.ID, 429, "rate limited"); err != nil {
		t.Fatalf("ReportError 429: %v", err)
	}
	got, err = store.Get(limited.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CooldownUntil == nil || !got.CooldownUntil.After(time.Now()) {
		t.Fatal("expected cooldown in the future")
	}
	if _, err := store.Next("deepseek"); !errors.Is(err, ErrNoAccounts) {
		t.Fatalf("expected no accounts (disabled + cooldown), got %v", err)
	}
}

func TestRemoveAndCurrent(t *testing.T) {
	store := newTestStore(t)

	a, err := store.Add(Account{Provider: "openai", Label: "main", APIKey: "k"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.SetCurrent("openai", a.ID); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	cur, err := store.Current("openai")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if cur != a.ID {
		t.Fatalf("current = %q, want %q", cur, a.ID)
	}

	if err := store.Remove(a.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := store.Get(a.ID); err == nil {
		t.Fatal("expected Get to fail after Remove")
	}
	cur, err = store.Current("openai")
	if err != nil {
		t.Fatalf("Current after remove: %v", err)
	}
	if cur != "" {
		t.Fatalf("expected current cleared, got %q", cur)
	}
}
