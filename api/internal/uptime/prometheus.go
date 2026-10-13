package uptime

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Sample est une ligne de mesure du format texte de Prometheus.
type Sample struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// ParsePrometheus lit le format texte d'exposition de Prometheus : une mesure
// par ligne, `nom{label="valeur",...} valeur [horodatage]`. Les commentaires
// (# HELP, # TYPE) et les lignes vides sont ignores.
//
// Ecrit ici plutot qu'importe : le client officiel tire une dizaine de
// paquets pour lire quelques centaines de lignes.
func ParsePrometheus(r io.Reader) ([]Sample, error) {
	var out []Sample
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		s, err := parseSample(text)
		if err != nil {
			return nil, fmt.Errorf("ligne %d : %w", line, err)
		}
		out = append(out, s)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("lecture des mesures : %w", err)
	}
	return out, nil
}

func parseSample(text string) (Sample, error) {
	s := Sample{Labels: map[string]string{}}

	end := strings.IndexAny(text, "{ \t")
	if end <= 0 {
		return s, fmt.Errorf("mesure sans valeur : %q", text)
	}
	s.Name = text[:end]
	rest := text[end:]

	if strings.HasPrefix(rest, "{") {
		labels, after, err := parseLabels(rest[1:])
		if err != nil {
			return s, err
		}
		s.Labels = labels
		rest = after
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return s, fmt.Errorf("mesure %s sans valeur", s.Name)
	}
	v, err := parseValue(fields[0])
	if err != nil {
		return s, fmt.Errorf("valeur de %s : %w", s.Name, err)
	}
	s.Value = v
	return s, nil
}

// parseLabels lit `a="x",b="y"}` et rend ce qui suit l'accolade fermante.
func parseLabels(text string) (map[string]string, string, error) {
	labels := map[string]string{}
	i := 0
	for {
		for i < len(text) && (text[i] == ' ' || text[i] == ',') {
			i++
		}
		if i >= len(text) {
			return nil, "", fmt.Errorf("labels non fermes")
		}
		if text[i] == '}' {
			return labels, text[i+1:], nil
		}

		eq := strings.IndexByte(text[i:], '=')
		if eq < 0 {
			return nil, "", fmt.Errorf("label sans valeur")
		}
		name := strings.TrimSpace(text[i : i+eq])
		i += eq + 1
		if i >= len(text) || text[i] != '"' {
			return nil, "", fmt.Errorf("valeur du label %s sans guillemets", name)
		}
		i++

		var b strings.Builder
		closed := false
		for i < len(text) {
			c := text[i]
			if c == '\\' && i+1 < len(text) {
				switch text[i+1] {
				case 'n':
					b.WriteByte('\n')
				default:
					b.WriteByte(text[i+1])
				}
				i += 2
				continue
			}
			if c == '"' {
				closed = true
				i++
				break
			}
			b.WriteByte(c)
			i++
		}
		if !closed {
			return nil, "", fmt.Errorf("valeur du label %s non fermee", name)
		}
		labels[name] = b.String()
	}
}

func parseValue(text string) (float64, error) {
	switch text {
	case "+Inf":
		return math.Inf(1), nil
	case "-Inf":
		return math.Inf(-1), nil
	case "NaN":
		return math.NaN(), nil
	}
	return strconv.ParseFloat(text, 64)
}
