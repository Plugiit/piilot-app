// Package coolify lit une instance Coolify : ses applications et l'historique
// de leurs deploiements. Lecture seule : Piilot ne declenche rien chez Coolify.
//
// Le schema de reponse de l'API est mal documente : la lecture est tolerante,
// un champ absent ou d'un autre type reste vide au lieu de faire echouer la
// releve.
package coolify

import (
	"context"
	"errors"
	"time"
)

// Application est une application Coolify, telle que la releve la voit.
type Application struct {
	UUID            string
	Name            string
	FQDN            string
	EnvironmentName string
	ProjectName     string
}

// Deployment est un deploiement d'une application.
type Deployment struct {
	// UUID du deploiement chez Coolify, vide s'il n'est pas fourni.
	UUID string
	// Status tel que Coolify le donne (« finished », « failed »…), sans
	// interpretation : les valeurs ne sont pas documentees.
	Status        string
	CommitSHA     string
	CommitMessage string
	// Dates a zero quand Coolify ne les donne pas ou dans un format inconnu.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Provider est un service de deploiement. Coolify en est le premier.
type Provider interface {
	// ListApplications lit toutes les applications de l'instance.
	ListApplications(ctx context.Context) ([]Application, error)
	// ListDeployments lit les deploiements d'une application crees a partir
	// de since (zero : tout l'historique accessible).
	ListDeployments(ctx context.Context, appUUID string, since time.Time) ([]Deployment, error)
}

var (
	// ErrUnauthorized : Coolify refuse le jeton (absent, mal forme, expire).
	ErrUnauthorized = errors.New("jeton refusé par Coolify")
	// ErrForbidden : Coolify reconnait le jeton mais refuse l'acces : API non
	// activee dans les reglages de l'instance, adresse IP non autorisee, ou
	// jeton sans la permission de lecture.
	ErrForbidden = errors.New("accès refusé par Coolify : API désactivée, adresse IP non autorisée ou jeton sans permission de lecture")
)
