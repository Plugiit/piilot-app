package spaces

import "testing"

func multi(t *testing.T) Spaces {
	t.Helper()
	s, err := Parse("https://auth.agence.fr", "https://team.agence.fr", "https://admin.agence.fr/", "https://client.agence.fr", "")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestParseDeduitLeParentCommun(t *testing.T) {
	s := multi(t)
	if s.CookieDomain != ".agence.fr" {
		t.Fatalf("parent : %q", s.CookieDomain)
	}
	if s.AdminURL != "https://admin.agence.fr" {
		t.Fatalf("la barre finale doit tomber : %q", s.AdminURL)
	}
}

func TestParseRefuseLesConfigurationsBancales(t *testing.T) {
	cases := map[string][4]string{
		"un seul domaine":   {"https://auth.agence.fr", "", "", ""},
		"pas de parent":     {"https://auth.agence.fr", "https://team.autre.fr", "https://admin.agence.fr", "https://client.agence.fr"},
		"parent trop court": {"https://agence.fr", "https://team.agence.fr", "https://admin.agence.fr", "https://client.agence.fr"},
		"chemin dans l'URL": {"https://auth.agence.fr/x", "https://team.agence.fr", "https://admin.agence.fr", "https://client.agence.fr"},
		"domaine en double": {"https://team.agence.fr", "https://team.agence.fr", "https://admin.agence.fr", "https://client.agence.fr"},
	}
	for name, c := range cases {
		if _, err := Parse(c[0], c[1], c[2], c[3], ""); err == nil {
			t.Errorf("%s : accepte", name)
		}
	}

	if _, err := Parse("https://auth.a.fr", "https://team.a.fr", "https://admin.a.fr", "https://client.a.fr", ".b.fr"); err == nil {
		t.Error("COOKIE_DOMAIN etranger aux domaines accepte")
	}
}

func TestMonoDomaineNeRedirigeRien(t *testing.T) {
	s, err := Parse("", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Enabled() || s.Redirect("piilot.fr", "/client", "") != "" {
		t.Fatal("le mode mono-domaine doit rester tel quel")
	}
}

func TestOfPath(t *testing.T) {
	cases := map[string]string{
		"/":                   "",
		"/compte/securite":    "",
		"/login":              Auth,
		"/invitation/abc":     Auth,
		"/reinitialiser/abc":  Auth,
		"/client":             Client,
		"/client/livrables/x": Client,
		"/clientele":          Team,
		"/pm":                 Admin,
		"/parametres/comptes": Admin,
		"/pm/projets":         Team,
		"/pm/mon-travail":     Team,
		"/crm/clients":        Team,
	}
	for path, want := range cases {
		if got := OfPath(path); got != want {
			t.Errorf("%s : %q, attendu %q", path, got, want)
		}
	}
}

func TestRedirectRenvoieVersLeBonDomaine(t *testing.T) {
	s := multi(t)
	cases := []struct{ host, path, query, want string }{
		{"team.agence.fr", "/pm/projets", "", ""},
		{"team.agence.fr", "/parametres/comptes", "", "https://admin.agence.fr/parametres/comptes"},
		{"client.agence.fr", "/pm/projets", "page=2", "https://team.agence.fr/pm/projets?page=2"},
		{"team.agence.fr", "/login", "redirect=%2Fpm", "https://auth.agence.fr/login?redirect=%2Fpm"},
		{"auth.agence.fr", "/client/tickets", "", "https://client.agence.fr/client/tickets"},
		{"admin.agence.fr", "/compte", "", ""},
		{"client.agence.fr", "/compte", "", "https://team.agence.fr/compte"},
		{"admin.agence.fr", "/", "", ""},
		// Un hote inconnu — une sonde, une IP — n'est jamais redirige.
		{"10.0.0.4:8080", "/client", "", ""},
	}
	for _, c := range cases {
		if got := s.Redirect(c.host, c.path, c.query); got != c.want {
			t.Errorf("%s%s : %q, attendu %q", c.host, c.path, got, c.want)
		}
	}
}
