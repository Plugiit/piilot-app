package usecase

import (
	"testing"

	"github.com/plugiit/piilot-app/api/internal/csvio"
)

// Un en-tete, une cle ou un alias en double rendrait la correspondance
// automatique ambigue : deux colonnes du fichier pour un champ, ou l'inverse.
func TestDataSpecsAreUnambiguous(t *testing.T) {
	for _, entity := range DataEntities {
		fields, ok := DataSpec(entity)
		if !ok {
			t.Fatalf("%s : pas de spec", entity)
		}

		keys := map[string]bool{}
		names := map[string]string{}

		for _, f := range fields {
			if f.Key == "" || f.Header == "" {
				t.Errorf("%s : champ incomplet %+v", entity, f)
			}

			if keys[f.Key] {
				t.Errorf("%s : cle %q en double", entity, f.Key)
			}

			keys[f.Key] = true

			// L'en-tete de l'export doit se reconnaitre lui-meme.
			own := csvio.NormalizeHeader(f.Header)
			if other, dup := names[own]; dup {
				t.Errorf("%s : en-tete %q de %s deja pris par %s", entity, f.Header, f.Key, other)
			}

			names[own] = f.Key
		}

		// Un alias ne doit pas viser deux champs differents.
		for _, f := range fields {
			for _, alias := range f.Aliases {
				n := csvio.NormalizeHeader(alias)
				if n == "" {
					t.Errorf("%s : alias %q vide une fois normalise", entity, alias)
				}

				if other, dup := names[n]; dup && other != f.Key {
					t.Errorf("%s : alias %q de %s deja pris par %s", entity, alias, f.Key, other)
				}

				names[n] = f.Key
			}
		}
	}
}

func TestDataSpecsCommonColumns(t *testing.T) {
	for _, entity := range DataEntities {
		fields, _ := DataSpec(entity)

		if fields[0].Key != FieldKeyID || fields[0].Header != "Identifiant Piilot" ||
			fields[1].Key != FieldKeySourceID || fields[1].Header != "Identifiant d'origine" {
			t.Errorf("%s : les deux identifiants doivent ouvrir la spec", entity)
		}
	}

	if _, ok := DataSpec("inconnu"); ok {
		t.Error("une entite inconnue n'a pas de spec")
	}

	want := []string{"Identifiant Piilot", "Identifiant d'origine", "E-mail", "Client", "Projet", "Service", "Date", "Durée (min)", "Note"}
	got := DataHeaders(DataEntityTimeEntries)

	if len(got) != len(want) {
		t.Fatalf("temps : %q", got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("temps colonne %d : %q, attendu %q", i, got[i], want[i])
		}
	}

	// Modifier la copie ne touche pas la spec.
	f, _ := DataSpec(DataEntityClients)
	f[0].Header = "x"

	if g, _ := DataSpec(DataEntityClients); g[0].Header == "x" {
		t.Error("DataSpec doit rendre une copie")
	}
}
