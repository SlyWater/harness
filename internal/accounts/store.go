package accounts

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrNoAccounts — нет доступных аккаунтов для провайдера.
var ErrNoAccounts = errors.New("accounts: no available accounts")

const (
	keyFileName = "secure-account-storage.key"
	indexFile   = "accounts.json"
	accountsDir = "accounts"
	lockFile    = "accounts.lock"
)

// Store — хранилище аккаунтов: plaintext-индекс + зашифрованные детали.
type Store struct {
	dir string
	mu  sync.Mutex
}

// DefaultDir возвращает ~/.harness (или HARNESS_HOME).
func DefaultDir() string {
	if v := os.Getenv("HARNESS_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".harness"
	}
	return filepath.Join(home, ".harness")
}

// Open открывает (и при необходимости создаёт) хранилище.
func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	if err := os.MkdirAll(filepath.Join(dir, accountsDir), 0o700); err != nil {
		return nil, fmt.Errorf("accounts: create dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Dir возвращает каталог хранилища.
func (s *Store) Dir() string { return s.dir }

func (s *Store) indexPath() string { return filepath.Join(s.dir, indexFile) }
func (s *Store) keyPath() string   { return filepath.Join(s.dir, keyFileName) }
func (s *Store) lockPath() string  { return filepath.Join(s.dir, lockFile) }
func (s *Store) accountPath(id string) string {
	return filepath.Join(s.dir, accountsDir, id+".json")
}

// Add сохраняет новый аккаунт.
func (s *Store) Add(a Account) (Account, error) {
	err := s.withLock(func() error {
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		if a.Provider == "" {
			return errors.New("accounts: provider is required")
		}
		if a.ID == "" {
			a.ID = newID()
		}
		for _, sum := range idx.Accounts {
			if sum.ID == a.ID {
				return fmt.Errorf("accounts: id %q already exists", a.ID)
			}
		}
		if a.Label == "" {
			a.Label = a.Email
		}
		if a.Label == "" {
			a.Label = a.ID
		}
		if a.CreatedAt.IsZero() {
			a.CreatedAt = time.Now().UTC()
		}
		if err := s.writeAccount(a); err != nil {
			return err
		}
		idx.Accounts = append(idx.Accounts, a.Summary())
		return s.saveIndex(idx)
	})
	return a, err
}

// Get возвращает полный аккаунт по id.
func (s *Store) Get(id string) (*Account, error) {
	var out *Account
	err := s.withLock(func() error {
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		for _, sum := range idx.Accounts {
			if sum.ID != id {
				continue
			}
			a, err := s.readAccount(id)
			if err != nil {
				return err
			}
			out = &a
			return nil
		}
		return fmt.Errorf("accounts: %q not found", id)
	})
	return out, err
}

// List возвращает аккаунты (все или только указанного провайдера).
func (s *Store) List(provider string) ([]Account, error) {
	var out []Account
	err := s.withLock(func() error {
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		for _, sum := range idx.Accounts {
			if provider != "" && sum.Provider != provider {
				continue
			}
			a, err := s.readAccount(sum.ID)
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

// Remove удаляет аккаунт и его файл.
func (s *Store) Remove(id string) error {
	return s.withLock(func() error {
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		kept := idx.Accounts[:0]
		found := false
		for _, sum := range idx.Accounts {
			if sum.ID == id {
				found = true
				continue
			}
			kept = append(kept, sum)
		}
		if !found {
			return fmt.Errorf("accounts: %q not found", id)
		}
		idx.Accounts = kept
		for p, cid := range idx.Current {
			if cid == id {
				delete(idx.Current, p)
			}
		}
		if err := os.Remove(s.accountPath(id)); err != nil && !os.IsNotExist(err) {
			return err
		}
		return s.saveIndex(idx)
	})
}

// SetDisabled включает/выключает аккаунт.
func (s *Store) SetDisabled(id string, disabled bool, reason string) error {
	return s.withLock(func() error {
		a, err := s.readAccount(id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		a.Disabled = disabled
		if disabled {
			a.DisabledReason = reason
			a.DisabledAt = &now
			a.CooldownUntil = nil
		} else {
			a.DisabledReason = ""
			a.DisabledAt = nil
		}
		if err := s.writeAccount(a); err != nil {
			return err
		}
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		for i := range idx.Accounts {
			if idx.Accounts[i].ID == id {
				idx.Accounts[i].Disabled = disabled
			}
		}
		return s.saveIndex(idx)
	})
}

// SetCurrent запоминает текущий аккаунт провайдера.
func (s *Store) SetCurrent(provider, id string) error {
	return s.withLock(func() error {
		a, err := s.readAccount(id)
		if err != nil {
			return err
		}
		if a.Provider != provider {
			return fmt.Errorf("accounts: %q belongs to provider %q, not %q", id, a.Provider, provider)
		}
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		if idx.Current == nil {
			idx.Current = make(map[string]string)
		}
		idx.Current[provider] = id
		return s.saveIndex(idx)
	})
}

// Current возвращает id текущего аккаунта провайдера (или "").
func (s *Store) Current(provider string) (string, error) {
	var id string
	err := s.withLock(func() error {
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		id = idx.Current[provider]
		return nil
	})
	return id, err
}

// Next выбирает аккаунт для запроса: активный, не на кулдауне, наименее
// использованный. Помечает его как использованный.
func (s *Store) Next(provider string) (*Account, error) {
	var chosen *Account
	err := s.withLock(func() error {
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, sum := range idx.Accounts {
			if sum.Provider != provider || sum.Disabled {
				continue
			}
			a, err := s.readAccount(sum.ID)
			if err != nil {
				return err
			}
			if a.Disabled {
				continue
			}
			if a.CooldownUntil != nil && a.CooldownUntil.After(now) {
				continue
			}
			if chosen == nil || lessRecentlyUsed(a, *chosen) {
				cp := a
				chosen = &cp
			}
		}
		if chosen == nil {
			return ErrNoAccounts
		}
		chosen.LastUsed = &now
		if err := s.writeAccount(*chosen); err != nil {
			return err
		}
		for i := range idx.Accounts {
			if idx.Accounts[i].ID == chosen.ID {
				idx.Accounts[i].LastUsed = &now
			}
		}
		return s.saveIndex(idx)
	})
	return chosen, err
}

// ReportError обновляет состояние аккаунта по HTTP-ошибке.
// 401/403 — disable, 429 — кулдаун 60s, 5xx — кулдаун 10s.
func (s *Store) ReportError(id string, status int, message string) error {
	return s.withLock(func() error {
		a, err := s.readAccount(id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		switch {
		case status == 401 || status == 403:
			a.Disabled = true
			a.DisabledReason = message
			a.DisabledAt = &now
			a.CooldownUntil = nil
		case status == 429:
			until := now.Add(60 * time.Second)
			a.CooldownUntil = &until
		case status >= 500:
			until := now.Add(10 * time.Second)
			a.CooldownUntil = &until
		default:
			return nil
		}
		if err := s.writeAccount(a); err != nil {
			return err
		}
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		for i := range idx.Accounts {
			if idx.Accounts[i].ID == id {
				idx.Accounts[i].Disabled = a.Disabled
			}
		}
		return s.saveIndex(idx)
	})
}

func lessRecentlyUsed(a, b Account) bool {
	if a.LastUsed == nil {
		return true
	}
	if b.LastUsed == nil {
		return false
	}
	return a.LastUsed.Before(*b.LastUsed)
}

func (s *Store) loadIndex() (Index, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return newIndex(), nil
		}
		return Index{}, fmt.Errorf("accounts: read index: %w", err)
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return Index{}, fmt.Errorf("accounts: parse index: %w", err)
	}
	if idx.Version == "" {
		idx.Version = IndexVersion
	}
	if idx.Current == nil {
		idx.Current = make(map[string]string)
	}
	return idx, nil
}

func (s *Store) saveIndex(idx Index) error {
	idx.Version = IndexVersion
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("accounts: marshal index: %w", err)
	}
	return atomicWrite(s.indexPath(), data)
}

func (s *Store) writeAccount(a Account) error {
	plaintext, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("accounts: marshal account: %w", err)
	}

	key, err := s.readOrCreateKey()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("accounts: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("accounts: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("accounts: nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	env := envelope{
		Version:     1,
		Algorithm:   "AES-256-GCM",
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
		EncryptedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("accounts: marshal envelope: %w", err)
	}
	return atomicWrite(s.accountPath(a.ID), data)
}

type envelope struct {
	Version     int       `json:"version"`
	Algorithm   string    `json:"algorithm"`
	Nonce       string    `json:"nonce"`
	Ciphertext  string    `json:"ciphertext"`
	EncryptedAt time.Time `json:"encrypted_at"`
}

func (s *Store) readAccount(id string) (Account, error) {
	data, err := os.ReadFile(s.accountPath(id))
	if err != nil {
		return Account{}, fmt.Errorf("accounts: read %q: %w", id, err)
	}

	// Обратная совместимость с plaintext-файлами.
	var probe struct {
		Ciphertext string `json:"ciphertext"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return Account{}, fmt.Errorf("accounts: parse %q: %w", id, err)
	}
	if probe.Ciphertext == "" {
		var a Account
		if err := json.Unmarshal(data, &a); err != nil {
			return Account{}, fmt.Errorf("accounts: parse plaintext %q: %w", id, err)
		}
		return a, nil
	}

	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Account{}, fmt.Errorf("accounts: parse envelope %q: %w", id, err)
	}
	if env.Version != 1 || env.Algorithm != "AES-256-GCM" {
		return Account{}, fmt.Errorf("accounts: unsupported envelope in %q", id)
	}

	key, err := s.readOrCreateKey()
	if err != nil {
		return Account{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Account{}, fmt.Errorf("accounts: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Account{}, fmt.Errorf("accounts: gcm: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return Account{}, fmt.Errorf("accounts: nonce %q: %w", id, err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return Account{}, fmt.Errorf("accounts: ciphertext %q: %w", id, err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return Account{}, fmt.Errorf("accounts: decrypt %q: %w", id, err)
	}
	var a Account
	if err := json.Unmarshal(plaintext, &a); err != nil {
		return Account{}, fmt.Errorf("accounts: parse decrypted %q: %w", id, err)
	}
	return a, nil
}

func (s *Store) readOrCreateKey() ([]byte, error) {
	path := s.keyPath()
	if data, err := os.ReadFile(path); err == nil {
		return decodeKey(data)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("accounts: read key: %w", err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("accounts: generate key: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil, fmt.Errorf("accounts: read key: %w", rerr)
			}
			return decodeKey(data)
		}
		return nil, fmt.Errorf("accounts: create key: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("accounts: write key: %w", err)
	}
	return key, nil
}

func decodeKey(data []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(string(bytesTrimSpace(data)))
	if err != nil {
		return nil, fmt.Errorf("accounts: decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("accounts: invalid key length %d", len(key))
	}
	return key, nil
}

func bytesTrimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}

func (s *Store) withLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := acquireFileLock(s.lockPath())
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func acquireFileLock(path string) (func(), error) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("accounts: acquire lock: %w", err)
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, errors.New("accounts: lock timeout")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("accounts: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("accounts: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("accounts: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("accounts: close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("accounts: rename: %w", err)
	}
	return nil
}

func newID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("acc-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
