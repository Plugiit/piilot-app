package usecase

import "testing"

func TestValidSiret(t *testing.T) {
	cases := map[string]bool{
		"55203253400646": true,  // Danone, etablissement du boulevard Haussmann
		"55203253400703": true,  // Danone, siege
		"55203253400647": false, // cle fausse
		"5520325340064":  false, // treize chiffres
		"5520325340064A": false,
		"35600000000048": true, // La Poste, siege : cle de Luhn
		"35600000049837": true, // La Poste, bureau : somme simple
		"55203253400010": false,
	}
	for siret, want := range cases {
		if got := validSiret(siret); got != want {
			t.Errorf("%s : %v, attendu %v", siret, got, want)
		}
	}
}

func TestNormalizeSiret(t *testing.T) {
	if got := normalizeSiret(" 552 032 534.00646 "); got != "55203253400646" {
		t.Fatalf("%q", got)
	}
}

func TestVatOfSiren(t *testing.T) {
	// Le registre donne FR27552032534 pour Danone.
	if got := vatOfSiren("552032534"); got != "FR27552032534" {
		t.Fatalf("%q", got)
	}
}
