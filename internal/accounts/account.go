package accounts

import "time"

// IndexVersion — версия формата индекса аккаунтов.
const IndexVersion = "1.0"

// Account — полная запись аккаунта (хранится зашифрованной).
type Account struct {
	ID             string     `json:"id"`
	Provider       string     `json:"provider"`
	Label          string     `json:"label"`
	Email          string     `json:"email,omitempty"`
	APIKey         string     `json:"api_key,omitempty"`
	BaseURL        string     `json:"base_url,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	Disabled       bool       `json:"disabled"`
	DisabledReason string     `json:"disabled_reason,omitempty"`
	DisabledAt     *time.Time `json:"disabled_at,omitempty"`
	CooldownUntil  *time.Time `json:"cooldown_until,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsed       *time.Time `json:"last_used,omitempty"`
	Quota          *Quota     `json:"quota,omitempty"`
}

// Summary — публичная часть аккаунта для индекса (без секретов).
type Summary struct {
	ID        string     `json:"id"`
	Provider  string     `json:"provider"`
	Label     string     `json:"label"`
	Email     string     `json:"email,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	Disabled  bool       `json:"disabled"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

// Summary возвращает безопасную для индекса копию.
func (a Account) Summary() Summary {
	return Summary{
		ID:        a.ID,
		Provider:  a.Provider,
		Label:     a.Label,
		Email:     a.Email,
		Tags:      a.Tags,
		Disabled:  a.Disabled,
		CreatedAt: a.CreatedAt,
		LastUsed:  a.LastUsed,
	}
}

// Quota — снимок квот аккаунта (аналог QuotaData в cockpit-tools).
type Quota struct {
	Models       []ModelQuota `json:"models,omitempty"`
	LastUpdated  time.Time    `json:"last_updated,omitempty"`
	Subscription string       `json:"subscription,omitempty"`
}

// ModelQuota — квота по одной модели.
type ModelQuota struct {
	Name       string `json:"name"`
	Percentage int    `json:"percentage"` // остаток 0..100
	ResetTime  string `json:"reset_time,omitempty"`
}

// Index — accounts.json: метаданные без секретов.
type Index struct {
	Version  string            `json:"version"`
	Current  map[string]string `json:"current,omitempty"` // provider -> account id
	Accounts []Summary         `json:"accounts"`
}

func newIndex() Index {
	return Index{
		Version:  IndexVersion,
		Current:  make(map[string]string),
		Accounts: []Summary{},
	}
}
