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
