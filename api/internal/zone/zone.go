// Package zone porte le fuseau de l'agence : celui de « aujourd'hui », des
// semaines, des mois, des dates des e-mails et des exports. Il se regle par
// APP_TIMEZONE, Europe/Paris par defaut — Piilot est open source, une agence
// ailleurs regle le sien.
//
// Le fuseau du serveur n'y suffit pas : un conteneur tourne volontiers en
// UTC, et une tache due ce jour passerait en retard des une heure du matin.
package zone

import (
	"sync/atomic"
	"time"
)

// Default est le fuseau sans reglage.
const Default = "Europe/Paris"

var current atomic.Pointer[time.Location]

func init() {
	loc, err := time.LoadLocation(Default)
	if err != nil {
		loc = time.UTC
	}
	current.Store(loc)
}

// Set fixe le fuseau, au demarrage.
func Set(loc *time.Location) {
	if loc != nil {
		current.Store(loc)
	}
}

// Location rend le fuseau de l'agence.
func Location() *time.Location { return current.Load() }

// Name rend son nom IANA : « Europe/Paris ».
func Name() string { return current.Load().String() }
