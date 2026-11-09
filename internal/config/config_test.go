package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseModelRef(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
		wantErr      bool
	}{
		{"openai:gpt-4o", "openai", "gpt-4o", false},
		{"deepseek:deepseek-chat", "deepseek", "deepseek-chat", false},
		{"openai:gpt-4o:mini", "openai", "gpt-4o:mini", false},
		{"invalid", "", "", true},
		{":model", "", "", true},
		{"provider:", "", "", true},
	}

	for _, tt := range tests {
		p, m, err := ParseModelRef(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseModelRef(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if p != tt.wantProvider || m != tt.wantModel {
			t.Errorf("ParseModelRef(%q) = (%q, %q), want (%q, %q)", tt.input, p, m, tt.wantProvider, tt.wantModel)
		}
	}
}

func TestResolveKeys(t *testing.T) {
	os.Setenv("TEST_KEY_1", "key-a,key-b")
	os.Setenv("TEST_KEY_2", "key-c")
	defer func() {
		os.Unsetenv("TEST_KEY_1")
		os.Unsetenv("TEST_KEY_2")
	}()

	p := ProviderConfig{
		APIKeyEnv:  "TEST_KEY_2",
		APIKeyEnvs: []string{"TEST_KEY_1"},
	}

	keys := p.ResolveKeys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(keys), keys)
	}
	expected := []string{"key-a", "key-b", "key-c"}
	for i, exp := range expected {
		if keys[i] != exp {
			t.Errorf("key[%d] = %q, want %q", i, keys[i], exp)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	content := `{
  "defaults": {
    "thinker": "openai:gpt-4o",
    "worker": "openai:gpt-4o-mini"
  },
  "providers": {
    "openai": {
      "type": "openai_compatible",
      "base_url": "https://api.openai.com/v1",
      "api_key_env": "OPENAI_API_KEY"
    }
  },
  "router": {
    "fallback": ["openai"]
  }
}`

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected Load error: %v", err)
	}

	if cfg.Defaults.MaxSteps != 25 {
		t.Errorf("expected default MaxSteps 25, got %d", cfg.Defaults.MaxSteps)
	}
	if cfg.Defaults.MaxTokens != 8192 {
		t.Errorf("expected default MaxTokens 8192, got %d", cfg.Defaults.MaxTokens)
	}
	if cfg.Defaults.Temperature != 0.2 {
		t.Errorf("expected default Temperature 0.2, got %f", cfg.Defaults.Temperature)
	}
	if _, ok := cfg.Providers["openai"]; !ok {
		t.Fatalf("expected provider openai to exist")
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	content := `{"defaults": {"thinker": "openai:gpt-4o"}, "providers": {}, "surprise": true}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected error for unknown field")
	}
}
