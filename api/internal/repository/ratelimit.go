package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter compte les tentatives par cle sur une fenetre fixe.
//
// Fenetre fixe et non glissante : la seconde demande un tri par horodatage a
// chaque appel, la premiere tient en un INCR. Le defaut connu de la fenetre
// fixe — jusqu'a deux fois la limite a cheval sur deux fenetres — est sans
// portee ici, ou l'objectif est de rendre le test exhaustif de mots de passe
// impraticable, pas de compter au jeton pres.
type RateLimiter struct {
	rdb *redis.Client
}

// NewRateLimiter construit le compteur sur le client Redis de l'application.
func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	return &RateLimiter{rdb: rdb}
}

// Allow incremente le compteur et indique si la tentative passe. Le second
// retour est le delai avant reouverture, a servir dans l'en-tete Retry-After.
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	fullKey := "ratelimit:" + key

	// ExpireNX ne pose le TTL qu'a la premiere incrementation. Un Expire nu le
	// repousserait a chaque tentative, et la fenetre ne se refermerait jamais
	// tant que l'attaquant continue de frapper.
	pipe := r.rdb.Pipeline()
	incr := pipe.Incr(ctx, fullKey)
	pipe.ExpireNX(ctx, fullKey, window)
	ttl := pipe.TTL(ctx, fullKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, fmt.Errorf("compteur de tentatives : %w", err)
	}

	count := incr.Val()
	retryAfter := ttl.Val()

	// TTL negatif : la cle existe sans expiration (course perdue avec un autre
	// process). On la borne pour qu'elle ne bloque pas indefiniment.
	if retryAfter < 0 {
		retryAfter = window
		r.rdb.Expire(ctx, fullKey, window)
	}

	return count <= int64(limit), retryAfter, nil
}

// Reset efface le compteur. Appele apres une authentification reussie : les
// tentatives ratees d'un utilisateur qui finit par se rappeler son mot de passe
// ne doivent pas le penaliser ensuite.
func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	if err := r.rdb.Del(ctx, "ratelimit:"+key).Err(); err != nil {
		return fmt.Errorf("remise a zero du compteur : %w", err)
	}

	return nil
}
