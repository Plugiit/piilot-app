// Package handler expose les endpoints HTTP. Un handler ne fait que trois
// choses : valider l'entree, appeler un usecase, serialiser la sortie.
package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Health expose l'etat de l'API et de ses dependances. C'est cet endpoint que
// le healthcheck Docker et Coolify interrogent.
type Health struct {
	db    *pgxpool.Pool
	redis *redis.Client
	build BuildInfo
}

// BuildInfo identifie la version deployee, injectee au build via -ldflags.
type BuildInfo struct {
	Version string
	Commit  string
}

// NewHealth construit le handler de sante.
func NewHealth(db *pgxpool.Pool, rdb *redis.Client, build BuildInfo) *Health {
	return &Health{db: db, redis: rdb, build: build}
}

// Live repond 200 des que le process ecoute. Ne touche aucune dependance :
// c'est la sonde de liveness, elle ne doit pas faire redemarrer le conteneur
// parce que Postgres est momentanement indisponible.
func (h *Health) Live(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"version": h.build.Version,
		"commit":  h.build.Commit,
	})
}

// Ready repond 200 seulement si l'API peut reellement servir du trafic :
// Postgres et Redis repondent. C'est la sonde de readiness.
func (h *Health) Ready(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	checks := fiber.Map{
		"database": "ok",
		"redis":    "ok",
	}
	healthy := true

	if err := h.db.Ping(ctx); err != nil {
		checks["database"] = "unavailable"
		healthy = false
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unavailable"
		healthy = false
	}

	status := fiber.StatusOK
	if !healthy {
		status = fiber.StatusServiceUnavailable
	}

	return c.Status(status).JSON(fiber.Map{
		"status": map[bool]string{true: "ok", false: "degraded"}[healthy],
		"checks": checks,
	})
}
