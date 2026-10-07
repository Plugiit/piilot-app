package usecase

import (
	"reflect"
	"testing"
)

func TestParseGitRefs(t *testing.T) {
	cases := []struct {
		name    string
		texts   []string
		tickets []int64
		tasks   []int64
	}{
		{"titre avec un ticket", []string{"Corrige le panier #47"}, []int64{47}, nil},
		{"branche ticket", []string{"fix/ticket-47-panier"}, []int64{47}, nil},
		{"branche tache", []string{"feat/T-123-header"}, nil, []int64{123}},
		{"tache courte", []string{"T123 header"}, nil, []int64{123}},
		{"mot tache", []string{"task-9", "tache_10"}, nil, []int64{9, 10}},
		{"plusieurs, sans doublon", []string{"#47 et #48 (#47)", "ticket/48"}, []int64{47, 48}, nil},
		{"rien", []string{"Refonte du header", "feature/header"}, nil, nil},
		// « t » en minuscule sans separateur n'est pas une reference : « t3 »
		// est un mot comme un autre, et « 2t5 » un numero de piece.
		{"faux positifs", []string{"t3 et 2t5 et v12 et #", "item-7"}, nil, nil},
		{"titre et branche ensemble", []string{"Ajout de la newsletter", "feature/T-12-newsletter-#33"}, []int64{33}, []int64{12}},
	}

	for _, c := range cases {
		got := ParseGitRefs(c.texts...)
		if !reflect.DeepEqual(got.Tickets, c.tickets) || !reflect.DeepEqual(got.Tasks, c.tasks) {
			t.Errorf("%s : %+v, attendu tickets %v taches %v", c.name, got, c.tickets, c.tasks)
		}
	}
}

func TestNormalizeRepoURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/Plugiit/piilot-app":       "https://github.com/plugiit/piilot-app",
		"https://github.com/Plugiit/piilot-app.git/":  "https://github.com/plugiit/piilot-app",
		"git@github.com:Plugiit/piilot-app.git":       "https://github.com/plugiit/piilot-app",
		"gitlab.agence.fr/clients/brasserie/site":     "https://gitlab.agence.fr/clients/brasserie/site",
		"https://gitlab.com/groupe/sous-groupe/depot": "https://gitlab.com/groupe/sous-groupe/depot",
		"github.com":      "",
		"pas une adresse": "",
		"":                "",
	}
	for in, want := range cases {
		if got := NormalizeRepoURL(in); got != want {
			t.Errorf("%q : %q, attendu %q", in, got, want)
		}
	}

	if GitProviderOf("https://github.com/x/y") != "github" || GitProviderOf("https://gitlab.agence.fr/x/y") != "gitlab" {
		t.Error("fournisseur mal devine")
	}
}
