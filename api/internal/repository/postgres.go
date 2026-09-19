// Package repository ouvre et expose les connexions aux dependances externes
// (Postgres, Redis). Les requetes typees vivent dans le sous-package db,
// genere par sqlc depuis queries/*.sql.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig regle le dimensionnement du pool Postgres.
type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// DefaultPoolConfig convient a une API servie par un seul conteneur.
//
// MaxConns est volontairement bas : Postgres degrade au-dela de quelques
// dizaines de connexions actives, et un pool Go partage entre goroutines n'a
// pas besoin d'une connexion par requete — contrairement a PHP-FPM ou chaque
// worker ouvre la sienne.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConns:          10,
		MinConns:          2,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: time.Minute,
	}
}

// NewPool ouvre le pool Postgres et verifie que la base repond avant de rendre
// la main, pour que le conteneur echoue au demarrage plutot qu'a la premiere
// requete.
func NewPool(ctx context.Context, databaseURL string, cfg PoolConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL invalide : %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("ouverture du pool : %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("Postgres injoignable : %w", err)
	}

	return pool, nil
}
