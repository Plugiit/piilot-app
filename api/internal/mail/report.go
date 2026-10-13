package mail

import (
	"fmt"
	"strings"
)

// ReportFigure est un chiffre cle du rapport : sa valeur, et sa variation
// quand il y en a une.
type ReportFigure struct {
	Label string
	Value string
	// Vide sans comparaison.
	Delta string
	// La variation va dans le bon sens : verte, sinon rouge.
	DeltaGood bool
}

// ReportRow est une ligne de classement, avec sa barre (0 a 100).
type ReportRow struct {
	Label string
	Value string
	Ratio int
}

// ReportItem est une ligne datee : une intervention, un incident, une mise en
// ligne.
type ReportItem struct {
	Date   string
	Label  string
	Detail string
}

// ReportMail est le resume du rapport mensuel que porte l'e-mail. Le rapport
// complet, graphiques compris, est dans l'espace client : les messageries
// n'affichent pas le SVG.
type ReportMail struct {
	Month       string
	ProjectName string
	Editorial   []string
	Figures     []ReportFigure

	// Le temps consomme sur le temps inclus, et sa barre (0 a 100).
	Consumed      string
	ConsumedRatio int
	Overrun       string
	Interventions []ReportItem

	HasAvailability bool
	Incidents       []ReportItem

	Pages   []ReportRow
	Sources []ReportRow

	Deployments []ReportItem
}

// MonthlyReport compose l'e-mail du rapport mensuel.
func MonthlyReport(to, firstname string, r ReportMail, url string) (Message, error) {
	v := view{
		Subject:     fmt.Sprintf("Rapport de maintenance · %s · %s", r.Month, r.ProjectName),
		Title:       fmt.Sprintf("Votre site en %s", r.Month),
		Action:      "Voir le rapport complet",
		URL:         url,
		Firstname:   firstname,
		ProjectName: r.ProjectName,
		Report:      &r,
	}

	var text strings.Builder
	fmt.Fprintf(&text, "Bonjour%s,\n\nVoici le rapport de maintenance de %s pour %s.\n\n", prefixed(firstname), r.Month, r.ProjectName)
	for _, p := range r.Editorial {
		fmt.Fprintf(&text, "%s\n\n", p)
	}
	for _, f := range r.Figures {
		fmt.Fprintf(&text, "- %s : %s", f.Label, f.Value)
		if f.Delta != "" {
			fmt.Fprintf(&text, " (%s)", f.Delta)
		}
		text.WriteString("\n")
	}
	if r.Consumed != "" {
		fmt.Fprintf(&text, "- Temps de maintenance : %s%s\n", r.Consumed, ifElse(r.Overrun != "", " ("+r.Overrun+")", ""))
	}
	fmt.Fprintf(&text, "\nLe rapport complet, avec ses graphiques, est dans votre espace :\n\n%s\n", url)

	return compose("monthly_report", to, v, "report.html", text.String())
}
