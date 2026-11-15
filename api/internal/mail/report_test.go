package mail

import (
	"strings"
	"testing"
)

func TestLeRapportMensuelSeComposeSansSVG(t *testing.T) {
	msg, err := MonthlyReport("client@example.fr", "Inès", ReportMail{
		Month: "octobre 2026", ProjectName: "Vitrine Atelier", Editorial: []string{"Un mois calme."},
		Figures:  []ReportFigure{{Label: "Disponibilité", Value: "99,95 %"}, {Label: "Visiteurs", Value: "3 263", Delta: "+9,9 %", DeltaGood: true}},
		Consumed: "2h35 sur 2h", ConsumedRatio: 100, Overrun: "+35 min",
		Interventions:   []ReportItem{{Date: "7 oct.", Label: "Correctif", Detail: "Formulaire de contact"}},
		HasAvailability: true,
		Pages:           []ReportRow{{Label: "/", Value: "1 840", Ratio: 100}},
	}, "https://piilot.test/client/rapports/42")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Vitrine Atelier", "Un mois calme.", "99,95 %", "9,9 %", "35 min", "Aucun incident ce mois-ci.", "/client/rapports/42", "Formulaire de contact"} {
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("le HTML ne contient pas %q", want)
		}
	}
	if strings.Contains(msg.HTML, "<svg") {
		t.Error("le HTML porte du SVG")
	}
	if !strings.Contains(msg.Text, "https://piilot.test/client/rapports/42") || !strings.Contains(msg.Subject, "octobre 2026") {
		t.Errorf("texte ou sujet : %q / %q", msg.Subject, msg.Text)
	}
}

func TestLeRapportDitDOuViennentLesMisesEnLigne(t *testing.T) {
	base := ReportMail{
		Month: "octobre 2026", ProjectName: "Vitrine Atelier",
		Figures: []ReportFigure{
			{Label: "Mises à jour de dépendances", Value: "4"},
			{Label: "Correctifs de sécurité", Value: "1"},
		},
		Deployments: []ReportItem{{Date: "7 oct.", Label: "Corrige le panier", Detail: "production"}, {Date: "9 oct.", Label: "Ajoute la page tarifs", Detail: "production"}},
	}
	for name, tc := range map[string]struct {
		source string
		want   string
		absent []string
	}{
		"coolify":  {"d’après Coolify", "d’après Coolify", []string{"d’après le dépôt"}},
		"depot":    {"d’après le dépôt", "d’après le dépôt", []string{"d’après Coolify"}},
		"inconnue": {"", "Mises en ligne", []string{"d’après"}},
	} {
		t.Run(name, func(t *testing.T) {
			r := base
			r.DeploymentsSource = tc.source
			msg, err := MonthlyReport("client@example.fr", "Inès", r, "https://piilot.test/client/rapports/42")
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{tc.want, "Corrige le panier", "Ajoute la page tarifs", "Mises à jour de dépendances", "Correctifs de sécurité"} {
				if !strings.Contains(msg.HTML, want) {
					t.Errorf("le HTML ne contient pas %q", want)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(msg.HTML, absent) || strings.Contains(msg.Text, absent) {
					t.Errorf("%q ne devrait pas figurer", absent)
				}
			}
			if !strings.Contains(msg.Text, "Mises en ligne") || !strings.Contains(msg.Text, ": 2") || !strings.Contains(msg.Text, "Correctifs de sécurité : 1") {
				t.Errorf("texte : %q", msg.Text)
			}
			if tc.source != "" && !strings.Contains(msg.Text, tc.source) {
				t.Errorf("le texte ne dit pas la source : %q", msg.Text)
			}
		})
	}
}

func TestLeRapportSansMiseEnLigneNeParleNiDeSourceNiDeSection(t *testing.T) {
	msg, err := MonthlyReport("client@example.fr", "", ReportMail{Month: "octobre 2026", ProjectName: "Vitrine"}, "https://piilot.test/x")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.HTML, "Mises en ligne") || strings.Contains(msg.Text, "Mises en ligne") {
		t.Error("une section vide s'affiche")
	}
}
