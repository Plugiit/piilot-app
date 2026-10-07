package backup

import (
	"testing"
	"time"
)

func TestRetentionGardeJoursSemainesMois(t *testing.T) {
	now := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	dates := map[string]time.Time{}
	// Une sauvegarde par jour depuis six mois.
	for d := 0; d < 180; d++ {
		at := now.AddDate(0, 0, -d)
		dates[at.Format("2006-01-02")] = at
	}

	keep := DefaultRetention().Keep(dates, now)

	// Les sept derniers jours, tous.
	for d := 0; d < 7; d++ {
		if !keep[now.AddDate(0, 0, -d).Format("2006-01-02")] {
			t.Errorf("jour -%d perdu", d)
		}
	}
	// Au-dela, plus toutes : le samedi 26 septembre n'est ni le dernier de sa
	// semaine (c'est le dimanche 27) ni de son mois.
	if keep["2026-09-26"] {
		t.Error("jour -11 garde alors qu'il ne represente rien")
	}
	// Une par semaine sur quatre semaines : le dimanche 2026-09-13 ferme sa
	// semaine ISO.
	if !keep["2026-09-13"] {
		t.Error("derniere sauvegarde de la semaine du 7 septembre perdue")
	}
	// Une par mois sur trois mois : le 31 aout et le 31 juillet.
	if !keep["2026-08-31"] || !keep["2026-07-31"] {
		t.Error("derniere sauvegarde du mois perdue")
	}
	// Six mois en arriere : plus rien.
	if keep["2026-04-30"] {
		t.Error("sauvegarde hors retention gardee")
	}

	total := 0
	for _, v := range keep {
		if v {
			total++
		}
	}
	// 7 jours + 4 semaines + 3 mois, moins les recouvrements : entre 10 et 14.
	if total < 10 || total > 14 {
		t.Errorf("%d sauvegardes gardees", total)
	}
}

func TestRetentionGardeToujoursLaDerniere(t *testing.T) {
	now := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	old := now.AddDate(-1, 0, 0)
	keep := Retention{}.Keep(map[string]time.Time{"vieille": old}, now)
	if !keep["vieille"] {
		t.Error("la seule sauvegarde a ete effacee")
	}
}
