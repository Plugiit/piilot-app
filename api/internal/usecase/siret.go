package usecase

import (
	"fmt"
	"strings"
)

// normalizeSiret retire les espaces et les points qu'on tape ou qu'on colle
// avec un SIRET (« 552 032 534 00646 »).
func normalizeSiret(raw string) string {
	return strings.NewReplacer(" ", "", ".", "", " ", "").Replace(strings.TrimSpace(raw))
}

// validSiret dit si un SIRET est bien forme : quatorze chiffres et une cle de
// Luhn juste. Il ne dit pas qu'il existe — seul le registre le sait.
//
// Les etablissements de La Poste (SIREN 356000000) font exception : hors du
// siege, leur cle est la somme simple des chiffres, multiple de 5.
func validSiret(siret string) bool {
	if len(siret) != 14 {
		return false
	}

	sum, plain := 0, 0
	for i, r := range siret {
		if r < '0' || r > '9' {
			return false
		}

		d := int(r - '0')
		plain += d
		// En partant de la droite, un chiffre sur deux est double.
		if (14-i)%2 == 0 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}

	if sum%10 == 0 {
		return true
	}

	return strings.HasPrefix(siret, "356000000") && plain%5 == 0
}

// vatOfSiren rend le numero de TVA intracommunautaire francais d'une
// entreprise : « FR », une cle de deux chiffres, puis le SIREN. La cle se
// calcule, elle ne se demande pas.
func vatOfSiren(siren string) string {
	n := 0
	for _, r := range siren {
		n = (n*10 + int(r-'0')) % 97
	}

	return fmt.Sprintf("FR%02d%s", (12+3*n)%97, siren)
}
