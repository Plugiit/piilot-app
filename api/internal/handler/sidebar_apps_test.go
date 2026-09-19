package handler

import "testing"

// TestLooksLikeSVG couvre la fonction qui decide si un fichier est servi comme
// un SVG.
//
// C'est une decision de securite : un fichier reconnu a tort laisserait servir
// autre chose sous un type d'image. Les cas negatifs comptent donc autant que
// les positifs.
func TestLooksLikeSVG(t *testing.T) {
	cas := []struct {
		nom     string
		contenu string
		attendu bool
	}{
		{"balise nue", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`, true},
		{"espaces avant", "\n\t  <svg viewBox=\"0 0 24 24\"></svg>", true},
		{"declaration xml", `<?xml version="1.0"?><svg></svg>`, true},
		{"doctype", `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN"><svg></svg>`, true},
		{"commentaire", "<!-- logo -->\n<svg></svg>", true},
		{"declaration et doctype", `<?xml version="1.0"?>` + "\n" + `<!DOCTYPE svg><svg></svg>`, true},

		{"page html", `<!DOCTYPE html><html><body>bonjour</body></html>`, false},
		{"html sans doctype", `<html><script>alert(1)</script></html>`, false},
		{"texte simple", "ceci n'est pas une image", false},
		{"vide", "", false},
		{"chevron seul", "<", false},
		// Une balise ouverte sans fin ne doit pas faire boucler ni passer.
		{"declaration jamais fermee", `<?xml version="1.0"`, false},
		// Le nom doit etre exact : « svgx » n'est pas « svg ».
		{"balise voisine", `<svgx></svgx>`, false},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := looksLikeSVG([]byte(c.contenu)); got != c.attendu {
				t.Fatalf("looksLikeSVG(%q) = %v, attendu %v", c.contenu, got, c.attendu)
			}
		})
	}
}
