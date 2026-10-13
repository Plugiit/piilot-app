// Package uptime lit un service de surveillance de disponibilite (Uptime
// Kuma pour commencer) : la liste des sites surveilles et leur etat courant,
// et les changements d'etat qu'il pousse par webhook.
//
// Le paquet ne tient aucun historique : /metrics ne donne que l'instant
// present. La disponibilite dans le temps et les incidents sont calcules par
// Piilot, a partir de ce que le job releve et de ce que le webhook recoit,
// dans ses propres tables. Un fournisseur qui exposerait son historique
// pourrait le reprendre, sans que le reste change.
package uptime

import (
	"context"
	"errors"
	"time"
)

// Status est l'etat d'un site, dans le decoupage de Kuma.
type Status int

const (
	Down        Status = 0
	Up          Status = 1
	Pending     Status = 2
	Maintenance Status = 3
)

// Valid dit si la valeur est un etat connu.
func (s Status) Valid() bool { return s >= Down && s <= Maintenance }

// Monitor est un site surveille, tel que le fournisseur le decrit a
// l'instant de la releve.
type Monitor struct {
	// Key identifie le site chez le fournisseur, de facon stable d'une releve
	// a l'autre : « id:12 » quand le fournisseur donne un identifiant, sinon
	// « name:Site vitrine ».
	Key      string
	Name     string
	Type     string
	URL      string
	Hostname string
	Status   Status
	// Temps de reponse du dernier controle, nul s'il n'est pas connu.
	ResponseMS *int
	// Jours avant l'expiration du certificat, et sa validite ; nuls pour un
	// site sans TLS.
	CertDays  *int
	CertValid *bool
}

// Event est un changement d'etat pousse par le fournisseur.
type Event struct {
	// Les cles possibles du site, de la plus sure a la moins sure : un
	// webhook porte l'identifiant et le nom, la releve n'a parfois que le nom.
	Keys    []string
	Status  Status
	At      time.Time
	Message string
}

// Provider est un service de surveillance. Kuma en est la premiere
// implementation.
type Provider interface {
	// ListMonitors lit l'etat courant de tous les sites surveilles.
	ListMonitors(ctx context.Context) ([]Monitor, error)
	// ParseEvent lit une notification poussee. ok est faux pour une
	// notification sans changement d'etat — l'essai d'un webhook, par
	// exemple — qui n'est pas une erreur.
	ParseEvent(body []byte) (event Event, ok bool, err error)
}

// ErrUnauthorized : le service refuse les identifiants.
var ErrUnauthorized = errors.New("identifiants refusés par le service")
