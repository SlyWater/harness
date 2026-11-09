package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Defaults — параметры по умолчанию.
type Defaults struct {
	Thinker     string  `json:"thinker"`
	Worker      string  `json:"worker"`
	MaxSteps    int     `json:"max_steps"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

// ProviderConfig — настройки одного провайдера.
//
// APIKeyEnv — основная переменная окружения с ключом (или списком ключей через
// запятую). APIKeyEnvs — дополнительные переменные для фермы ключей.
type ProviderConfig struct {
	Type       string   `json:"type"`
	BaseURL    string   `json:"base_url"`
	APIKeyEnv  string   `json:"api_key_env"`
	APIKeyEnvs []string `json:"api_key_envs,omitempty"`
}

// Router — настройки маршрутизации и фоллбэка.
type Router struct {
	Fallback []string `json:"fallback"`
}

// Config — корневой конфиг harness.
type Config struct {
	Defaults  Defaults                  `json:"defaults"`
	Providers map[string]ProviderConfig `json:"providers"`
	Router    Router                    `json:"router"`
}

// Load читает и валидирует конфиг из JSON-файла.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Defaults.MaxSteps == 0 {
		c.Defaults.MaxSteps = 25
	}
	if c.Defaults.MaxTokens == 0 {
		c.Defaults.MaxTokens = 8192
	}
	if c.Defaults.Temperature == 0 {
		c.Defaults.Temperature = 0.2
	}
}

// Validate проверяет обязательные поля.
func (c *Config) Validate() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("config: no providers configured")
	}
	for name, p := range c.Providers {
		if p.Type == "" {
			return fmt.Errorf("config: provider %q: type is required", name)
		}
		if p.BaseURL == "" {
			return fmt.Errorf("config: provider %q: base_url is required", name)
		}
	}
	return nil
}

// ParseModelRef разбирает строку вида "provider:model".
func ParseModelRef(ref string) (provider, model string, err error) {
	i := strings.Index(ref, ":")
	if i <= 0 || i == len(ref)-1 {
		return "", "", fmt.Errorf("config: invalid model ref %q (want provider:model)", ref)
	}
	return ref[:i], ref[i+1:], nil
}

// ResolveKeys собирает ключи из окружения: сначала APIKeyEnvs (в порядке
// объявления), затем APIKeyEnv. Значение каждой переменной может содержать
// несколько ключей через запятую. Пустые значения пропускаются.
func (p ProviderConfig) ResolveKeys() []string {
	var keys []string

	add := func(env string) {
		v := os.Getenv(env)
		if v == "" {
			return
		}
		for _, part := range strings.Split(v, ",") {
			if k := strings.TrimSpace(part); k != "" {
				keys = append(keys, k)
			}
		}
	}

	for _, env := range p.APIKeyEnvs {
		add(env)
	}
	add(p.APIKeyEnv)
	return keys
}

// APIKey возвращает первый доступный ключ или ошибку, если ключей нет.
// Если ключ не требуется (APIKeyEnv пуст) — возвращает пустую строку.
func (p ProviderConfig) APIKey() (string, error) {
	keys := p.ResolveKeys()
	if len(keys) > 0 {
		return keys[0], nil
	}
	if p.APIKeyEnv == "" && len(p.APIKeyEnvs) == 0 {
		return "", nil
	}
	return "", fmt.Errorf("config: no API key found in env %v", append(p.APIKeyEnvs, p.APIKeyEnv))
}
