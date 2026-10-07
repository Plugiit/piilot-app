// Package backup sauvegarde la base et les fichiers, et sait les restaurer.
//
// Une sauvegarde est une archive tar qui contient le dump Postgres (format
// custom de pg_dump) et les pieces jointes (tar.gz). Les deux vont ensemble :
// une base restauree sans ses fichiers pointe vers des fichiers absents.
package backup

import (
	"fmt"
	"sort"
	"time"
)

// Retention dit combien de sauvegardes garder, par horizon : toutes celles
// des derniers jours, puis une par semaine, puis une par mois.
type Retention struct {
	Daily   int
	Weekly  int
	Monthly int
}

// DefaultRetention : sept jours, quatre semaines, trois mois.
func DefaultRetention() Retention {
	return Retention{Daily: 7, Weekly: 4, Monthly: 3}
}

// Keep choisit, parmi des sauvegardes datees, celles a garder. La plus recente
// de chaque semaine et de chaque mois represente son horizon ; les autres
// partent une fois sorties de la fenetre quotidienne.
//
// Les cles de `dates` sont les noms des sauvegardes ; rend l'ensemble des
// noms a garder.
func (r Retention) Keep(dates map[string]time.Time, now time.Time) map[string]bool {
	keep := make(map[string]bool, len(dates))

	names := make([]string, 0, len(dates))
	for name := range dates {
		names = append(names, name)
	}
	// Du plus recent au plus ancien : le premier vu dans un creneau est le
	// plus recent du creneau.
	sort.Slice(names, func(i, j int) bool { return dates[names[i]].After(dates[names[j]]) })

	dailyLimit := now.AddDate(0, 0, -r.Daily)
	weeklyLimit := now.AddDate(0, 0, -7*r.Weekly)
	monthlyLimit := now.AddDate(0, -r.Monthly, 0)

	seenWeek := map[string]bool{}
	seenMonth := map[string]bool{}
	for _, name := range names {
		at := dates[name]
		if r.Daily > 0 && !at.Before(dailyLimit) {
			keep[name] = true
		}
		if r.Weekly > 0 && !at.Before(weeklyLimit) {
			year, week := at.ISOWeek()
			key := fmt.Sprintf("%d-w%02d", year, week)
			if !seenWeek[key] {
				seenWeek[key] = true
				keep[name] = true
			}
		}
		if r.Monthly > 0 && !at.Before(monthlyLimit) {
			key := at.Format("2006-01")
			if !seenMonth[key] {
				seenMonth[key] = true
				keep[name] = true
			}
		}
	}

	// La plus recente reste quoi qu'il arrive : une retention a zero ne doit
	// pas effacer la seule sauvegarde qui existe.
	if len(names) > 0 {
		keep[names[0]] = true
	}

	return keep
}
