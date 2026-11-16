package config

import "testing"

// Coolify injects COOLIFY_URL (the application's own address) into every
// container it deploys: it must never be read as the Coolify instance address.
func TestLoadIgnoresCoolifyInjectedURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("COOLIFY_URL", "https://piilot.example.com")
	t.Setenv("COOLIFY_API_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Coolify.URL != "" {
		t.Fatalf("Coolify.URL = %q, want empty", cfg.Coolify.URL)
	}
}
