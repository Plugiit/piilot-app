package usecase

import (
	"reflect"
	"testing"
)

func TestHoldReasonsOf(t *testing.T) {
	up := 1.0
	day := func(v *float64) []UptimeDay { return []UptimeDay{{Day: "2026-10-01", Uptime: v}} }
	complete := &ReportAvailability{Sites: []ReportSite{{Environment: "prod", Uptime: &up, Days: day(&up)}}}
	withIncident := &ReportAvailability{Sites: complete.Sites, Incidents: []ReportIncident{{Site: "Vitrine"}}}
	hole := &ReportAvailability{Sites: []ReportSite{{Environment: "prod", Uptime: &up, Days: day(nil)}}}
	preprodOnly := &ReportAvailability{Sites: []ReportSite{{Environment: "preprod", Uptime: &up, Days: day(&up)}}}
	preprodHole := &ReportAvailability{Sites: []ReportSite{complete.Sites[0], {Environment: "preprod", Days: day(nil)}}}

	cases := []struct {
		name       string
		data       ReportData
		sections   []string
		recipients int
		want       []string
	}{
		{"tout va bien", ReportData{Availability: complete}, []string{"availability"}, 1, []string{}},
		{"sans destinataire", ReportData{Availability: complete}, []string{"availability"}, 0, []string{holdNoRecipients}},
		{"site non surveille", ReportData{}, []string{"availability"}, 1, []string{holdAvailabilityMissing}},
		{"jour sans mesure", ReportData{Availability: hole}, []string{"availability"}, 1, []string{holdAvailabilityMissing}},
		{"seulement une preprod", ReportData{Availability: preprodOnly}, []string{"availability"}, 1, []string{holdAvailabilityMissing}},
		{"un trou en preprod ne compte pas", ReportData{Availability: preprodHole}, []string{"availability"}, 1, []string{}},
		{"un incident se relit", ReportData{Availability: withIncident}, []string{"availability"}, 1, []string{holdIncident}},
		{"section hors formule", ReportData{}, []string{"interventions"}, 1, []string{}},
		{"l'audience indisponible ne retient pas", ReportData{Audience: &ReportAudience{Unavailable: true}}, []string{"audience"}, 1, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := holdReasonsOf(c.data, c.sections, c.recipients); !reflect.DeepEqual(got, c.want) {
				t.Errorf("motifs : %v, attendu %v", got, c.want)
			}
		})
	}
}

func TestLAlerteNeSeRepeteQueSiLaSituationSAggrave(t *testing.T) {
	base := ReportAttentionPayload{Month: "2026-10", Unsent: 3, Retained: 1}
	cases := []struct {
		name string
		next ReportAttentionPayload
		want bool
	}{
		{"identique", base, false},
		{"un rapport est parti", ReportAttentionPayload{Month: "2026-10", Unsent: 2, Retained: 1}, false},
		{"un e-mail abandonne de plus", ReportAttentionPayload{Month: "2026-10", Unsent: 3, Retained: 1, Undelivered: 1}, true},
		{"nouveau mois", ReportAttentionPayload{Month: "2026-11", Unsent: 1}, true},
		{"SMTP retire", ReportAttentionPayload{Month: "2026-10", Unsent: 3, Retained: 1, SMTPMissing: true}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.next.worseThan(base); got != c.want {
				t.Errorf("worseThan = %v, attendu %v", got, c.want)
			}
		})
	}
}
