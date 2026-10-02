package usecase

import "testing"

func TestBudgetStateOfClasseSelonLaPartConsommee(t *testing.T) {
	cas := []struct {
		nom      string
		vendues  float64
		saisies  float64
		interne  bool
		attendus string
	}{
		{"projet interne", 100, 150, true, BudgetNone},
		{"sans heures vendues", 0, 12, false, BudgetNone},
		{"rien de consomme", 40, 0, false, BudgetOK},
		{"juste sous le seuil", 100, 79.75, false, BudgetOK},
		{"pile au seuil", 100, 80, false, BudgetWarning},
		{"budget tout juste epuise", 100, 100, false, BudgetWarning},
		{"une demi-heure de trop", 100, 100.5, false, BudgetOver},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := budgetStateOf(c.vendues, c.saisies, c.interne); got != c.attendus {
				t.Errorf("budgetStateOf(%v, %v, %v) = %q, attendu %q", c.vendues, c.saisies, c.interne, got, c.attendus)
			}
		})
	}
}

func TestValidBudgetFilterNAccepteQueLesDerives(t *testing.T) {
	for _, valeur := range []string{BudgetWarning, BudgetOver} {
		if !ValidBudgetFilter(valeur) {
			t.Errorf("%q devrait etre accepte", valeur)
		}
	}
	for _, valeur := range []string{BudgetOK, BudgetNone, "", "OVER"} {
		if ValidBudgetFilter(valeur) {
			t.Errorf("%q devrait etre refuse", valeur)
		}
	}
}

func TestWeekStartOfRendLeLundi(t *testing.T) {
	cas := map[string]string{
		"2026-10-05": "2026-10-05", // lundi
		"2026-10-07": "2026-10-05", // mercredi
		"2026-10-11": "2026-10-05", // dimanche : fin de semaine, pas debut
		"2026-11-01": "2026-10-26", // a cheval sur deux mois
	}

	for jour, lundi := range cas {
		if got := weekStartOf(day(jour)).Format(dateLayout); got != lundi {
			t.Errorf("weekStartOf(%s) = %s, attendu %s", jour, got, lundi)
		}
	}
}
