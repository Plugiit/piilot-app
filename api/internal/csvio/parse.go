package csvio

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ErrFormat : la valeur n'a pas le format attendu.
var ErrFormat = errors.New("format invalide")

// ParseDate lit « AAAA-MM-JJ » ou « JJ/MM/AAAA » (jour et mois sur un ou deux
// chiffres). Le resultat est minuit UTC.
func ParseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)

	for _, layout := range []string{"2006-01-02", "2/1/2006"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("date %q : %w", value, ErrFormat)
}

// ParseDecimal lit un nombre ecrit a la francaise : « 1,5 », « 1 234,56 »,
// « -3 ». Le point est accepte comme decimal ; quand les deux apparaissent,
// le dernier est le decimal et l'autre separe les milliers (« 1.234,56 »).
func ParseDecimal(value string) (float64, error) {
	s := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}

		return r
	}, value)

	if s == "" {
		return 0, fmt.Errorf("nombre vide : %w", ErrFormat)
	}

	comma, dot := strings.LastIndexByte(s, ','), strings.LastIndexByte(s, '.')

	switch {
	case comma >= 0 && dot >= 0:
		if comma > dot {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	case comma >= 0:
		if strings.Count(s, ",") > 1 {
			return 0, fmt.Errorf("nombre %q : %w", value, ErrFormat)
		}

		s = strings.Replace(s, ",", ".", 1)
	}

	// Seuls chiffres, signe de tete et un point : ni exposant, ni « inf ».
	digits := 0

	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '.' && strings.Count(s, ".") == 1:
		case (r == '-' || r == '+') && i == 0:
		default:
			return 0, fmt.Errorf("nombre %q : %w", value, ErrFormat)
		}
	}

	if digits == 0 {
		return 0, fmt.Errorf("nombre %q : %w", value, ErrFormat)
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, fmt.Errorf("nombre %q : %w", value, ErrFormat)
	}

	return f, nil
}

// ParseDuration lit une duree et la rend en minutes entieres :
//   - « 90 » : un entier seul, ce sont des minutes ;
//   - « 1h30 », « 1h », « 1h30m », « 45m », « 45min » ;
//   - « 1,5 » ou « 1.5 » : un decimal, ce sont des heures (90 minutes).
//
// Une duree negative est refusee ; les bornes metier sont a l'appelant.
func ParseDuration(value string) (int, error) {
	s := strings.ToLower(strings.TrimSpace(value))
	s = strings.Join(strings.Fields(s), "")

	if s == "" {
		return 0, fmt.Errorf("duree vide : %w", ErrFormat)
	}

	if h := strings.IndexByte(s, 'h'); h >= 0 {
		hours, err := atoiStrict(s[:h])
		if err != nil {
			return 0, fmt.Errorf("duree %q : %w", value, ErrFormat)
		}

		rest := strings.TrimSuffix(strings.TrimSuffix(s[h+1:], "min"), "m")

		minutes := 0
		if rest != "" {
			if minutes, err = atoiStrict(rest); err != nil || minutes > 59 {
				return 0, fmt.Errorf("duree %q : %w", value, ErrFormat)
			}
		}

		return hours*60 + minutes, nil
	}

	if rest, ok := cutSuffix(s, "min", "m"); ok {
		minutes, err := atoiStrict(rest)
		if err != nil {
			return 0, fmt.Errorf("duree %q : %w", value, ErrFormat)
		}

		return minutes, nil
	}

	if !strings.ContainsAny(s, ",.") {
		minutes, err := atoiStrict(s)
		if err != nil {
			return 0, fmt.Errorf("duree %q : %w", value, ErrFormat)
		}

		return minutes, nil
	}

	hours, err := ParseDecimal(s)
	if err != nil || hours < 0 {
		return 0, fmt.Errorf("duree %q : %w", value, ErrFormat)
	}

	return int(math.Round(hours * 60)), nil
}

// atoiStrict lit des chiffres seuls, sans signe.
func atoiStrict(s string) (int, error) {
	if s == "" || len(s) > 6 {
		return 0, ErrFormat
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, ErrFormat
		}
	}

	return strconv.Atoi(s)
}

func cutSuffix(s string, suffixes ...string) (string, bool) {
	for _, suffix := range suffixes {
		if rest, ok := strings.CutSuffix(s, suffix); ok {
			return rest, true
		}
	}

	return s, false
}
