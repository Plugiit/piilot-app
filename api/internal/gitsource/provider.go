// Package gitsource lit un service Git pour rattraper ce que le webhook a
// manque : les pull requests fusionnees d'une periode. Lecture seule.
//
// Le webhook reste la voie normale. Ici, on ne fait que combler une coupure
// (Piilot arrete, webhook mal configure, evenement perdu) avant qu'un rapport
// ne fige un mois incomplet.
package gitsource

import (
	"context"
	"errors"
	"time"
)

// PullRequest est une pull request fusionnee, telle que le service la decrit.
type PullRequest struct {
	Number   int64
	Title    string
	URL      string
	Branch   string
	Author   string
	Labels   []string
	OpenedAt *time.Time
	MergedAt time.Time
}

// Provider est un service Git. GitHub en est la premiere implementation.
type Provider interface {
	// MergedPullRequests rend les pull requests du depot fusionnees entre since
	// (inclus) et until (exclu). repoURL est l'adresse normalisee du depot :
	// https://github.com/proprietaire/depot.
	MergedPullRequests(ctx context.Context, repoURL string, since, until time.Time) ([]PullRequest, error)
}

var (
	// ErrNotImplemented : le service Git n'est pas encore pris en charge.
	ErrNotImplemented = errors.New("ce service Git n’est pas encore pris en charge pour le rattrapage")
	// ErrUnsupportedRepo : l'adresse n'est celle d'aucun depot connu.
	ErrUnsupportedRepo = errors.New("adresse de dépôt non reconnue : le rattrapage ne lit que les dépôts GitHub")
	// ErrUnauthorized : le service refuse le jeton.
	ErrUnauthorized = errors.New("jeton GitHub refusé : il est invalide, expiré ou révoqué")
	// ErrForbidden : le jeton est reconnu mais n'a pas le droit de lire.
	ErrForbidden = errors.New("accès refusé par GitHub : le jeton doit avoir la permission « Pull requests : lecture » sur ce dépôt")
	// ErrRepoNotFound : le depot n'existe pas, ou le jeton n'y a pas acces
	// (GitHub ne distingue pas les deux pour un depot prive).
	ErrRepoNotFound = errors.New("dépôt introuvable sur GitHub, ou jeton sans accès à ce dépôt")
	// ErrTooMany : la lecture a atteint son plafond sans finir.
	ErrTooMany = errors.New("trop de pull requests à lire : synchronisation abandonnée")
)
