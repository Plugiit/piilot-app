// Package config charge et valide la configuration depuis l'environnement.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env identifie l'environnement d'execution.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvStaging     Env = "staging"
	EnvProduction  Env = "production"
)

// Config regroupe toute la configuration de l'API. Aucun autre package ne lit
// os.Getenv : la configuration est chargee une fois au demarrage et injectee,
// ce qui rend les handlers testables sans variables d'environnement.
type Config struct {
	Env  Env
	Port int

	DatabaseURL string
	RedisURL    string

	// Origines autorisees pour l'admin, servi depuis un autre domaine et
	// authentifie par cookie httpOnly (donc CORS avec credentials).
	AdminOrigins []string

	JWTSecret     []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	CookieDomain  string
	PublicBaseURL string

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	// RunMigrations applique les migrations embarquees au demarrage. Actif par
	// defaut : le binaire porte son schema. A couper si les migrations sont
	// jouees par une etape de deploiement dediee.
	RunMigrations bool
}

// Load lit la configuration depuis l'environnement et echoue si une valeur
// obligatoire manque — un demarrage bruyant vaut mieux qu'une API qui repond
// 500 sur la premiere requete.
func Load() (Config, error) {
	cfg := Config{
		Env:             Env(env("APP_ENV", string(EnvDevelopment))),
		Port:            envInt("PORT", 8080),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        env("REDIS_URL", "redis://localhost:6379/0"),
		AdminOrigins:    envList("ADMIN_ORIGINS", "http://localhost:5173"),
		JWTSecret:       []byte(os.Getenv("JWT_SECRET")),
		AccessTTL:       envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL:      envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		CookieDomain:    os.Getenv("COOKIE_DOMAIN"),
		PublicBaseURL:   env("PUBLIC_BASE_URL", "http://localhost:8080"),
		ReadTimeout:     envDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:    envDuration("WRITE_TIMEOUT", 30*time.Second),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		RunMigrations:   envBool("RUN_MIGRATIONS", true),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL est obligatoire")
	}

	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET doit faire au moins 32 octets (actuel : %d)", len(cfg.JWTSecret))
	}

	switch cfg.Env {
	case EnvDevelopment, EnvStaging, EnvProduction:
	default:
		return Config{}, fmt.Errorf("APP_ENV invalide : %q", cfg.Env)
	}

	return cfg, nil
}

// IsDevelopment indique si l'API tourne en local (logs verbeux, CORS permissif).
func (c Config) IsDevelopment() bool { return c.Env == EnvDevelopment }

// IsProduction indique si l'API tourne en production (cookies Secure, logs JSON).
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// Addr retourne l'adresse d'ecoute du serveur HTTP.
func (c Config) Addr() string { return fmt.Sprintf(":%d", c.Port) }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envList(key, fallback string) []string {
	raw := env(key, fallback)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
