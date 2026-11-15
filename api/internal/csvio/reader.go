package csvio

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Encodages detectes.
const (
	EncodingUTF8        = "utf-8"
	EncodingWindows1252 = "windows-1252"
)

// ErrEmpty : le fichier ne contient pas meme une ligne d'en-tetes.
var ErrEmpty = errors.New("fichier vide")

// ErrUnsupportedEncoding : UTF-16 (« Unicode » d'Excel) n'est pas lu.
var ErrUnsupportedEncoding = errors.New("encodage non pris en charge")

// windows1252 donne les caracteres de 0x80 a 0x9F, la seule plage ou
// Windows-1252 differe de Latin-1. Zero : position non attribuee.
var windows1252 = [32]rune{
	'€', 0, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0, 'Ž', 0,
	0, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0, 'ž', 'Ÿ',
}

// Row est une ligne de donnees. Line est son numero dans le fichier, en-tete
// comprise (la premiere ligne de donnees porte le numero 2), celui que
// l'utilisateur retrouve dans son tableur.
type Row struct {
	Line  int
	Cells []string
}

// Reader lit un CSV de tableur. Le fichier est lu en entier (l'appelant en
// borne la taille) pour decider de l'encodage avant de decouper.
type Reader struct {
	cr        *csv.Reader
	delimiter rune
	encoding  string
	headers   []string
}

// NewReader detecte l'encodage et le separateur, lit la ligne d'en-tetes.
//
// Encodage : UTF-8 (avec ou sans BOM) si le contenu l'est, Windows-1252
// sinon. Separateur : `;`, `,` ou tabulation, celui qui revient le plus dans
// la premiere ligne hors guillemets ; `;` en cas d'egalite ou d'absence.
func NewReader(in io.Reader) (*Reader, error) {
	raw, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("lecture du csv : %w", err)
	}

	if bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) || bytes.HasPrefix(raw, []byte{0xFE, 0xFF}) {
		return nil, fmt.Errorf("utf-16 : %w", ErrUnsupportedEncoding)
	}

	raw = bytes.TrimPrefix(raw, []byte(BOM))

	encoding := EncodingUTF8

	text := string(raw)
	if !utf8.Valid(raw) {
		encoding = EncodingWindows1252
		text = decodeWindows1252(raw)
	}

	if strings.TrimSpace(text) == "" {
		return nil, ErrEmpty
	}

	r := &Reader{delimiter: detectDelimiter(text), encoding: encoding}

	r.cr = csv.NewReader(strings.NewReader(text))
	r.cr.Comma = r.delimiter
	r.cr.FieldsPerRecord = -1
	r.cr.ReuseRecord = false

	headers, err := r.cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrEmpty
		}

		return nil, fmt.Errorf("lecture des en-tetes : %w", err)
	}

	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}

	r.headers = headers

	return r, nil
}

// Delimiter est le separateur detecte.
func (r *Reader) Delimiter() rune { return r.delimiter }

// Encoding est l'encodage detecte : EncodingUTF8 ou EncodingWindows1252.
func (r *Reader) Encoding() string { return r.encoding }

// Headers rend les en-tetes tels qu'ecrits dans le fichier (espaces rognes).
func (r *Reader) Headers() []string { return r.headers }

// Next rend la ligne suivante, io.EOF a la fin. Les lignes vides sont
// sautees. Une ligne mal formee rend une erreur portant son numero ; la
// lecture peut continuer apres.
func (r *Reader) Next() (Row, error) {
	cells, err := r.cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Row{}, io.EOF
		}

		var pe *csv.ParseError
		if errors.As(err, &pe) {
			return Row{Line: pe.StartLine}, fmt.Errorf("ligne %d : %w", pe.StartLine, err)
		}

		return Row{}, fmt.Errorf("lecture du csv : %w", err)
	}

	line, _ := r.cr.FieldPos(0)

	for i, c := range cells {
		cells[i] = cleanCell(c)
	}

	return Row{Line: line, Cells: cells}, nil
}

// cleanCell retire l'apostrophe anti-formule posee par Cell (une seule, voir
// Cell) et rogne les espaces de fin ; sinon rogne les espaces. Une apostrophe
// devant autre chose qu'un declencheur est du texte. Pas de rognage a gauche
// quand l'apostrophe part : la valeur commence par un declencheur, dont la
// tabulation, qu'il faut garder.
func cleanCell(c string) string {
	if len(c) > 1 && c[0] == '\'' && formulaAfterQuotes(c) {
		return strings.TrimRightFunc(c[1:], unicode.IsSpace)
	}

	return strings.TrimSpace(c)
}

// detectDelimiter compte les candidats sur la premiere ligne, hors guillemets.
func detectDelimiter(text string) rune {
	counts := map[rune]int{}
	quoted := false

scan:
	for _, r := range text {
		switch {
		case r == '"':
			quoted = !quoted
		case quoted:
		case r == '\n':
			break scan
		case r == ';' || r == ',' || r == '\t':
			counts[r]++
		}
	}

	best := rune(Delimiter)
	for _, candidate := range []rune{',', '\t'} {
		if counts[candidate] > counts[best] {
			best = candidate
		}
	}

	return best
}

func decodeWindows1252(content []byte) string {
	var b strings.Builder
	b.Grow(len(content))

	for _, c := range content {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c < 0xA0:
			if r := windows1252[c-0x80]; r != 0 {
				b.WriteRune(r)
			} else {
				b.WriteRune(rune(c))
			}
		default:
			b.WriteRune(rune(c))
		}
	}

	return b.String()
}

var accentFolds = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ä", "a", "ã", "a", "å", "a",
	"ç", "c",
	"è", "e", "é", "e", "ê", "e", "ë", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i",
	"ñ", "n",
	"ò", "o", "ó", "o", "ô", "o", "ö", "o", "õ", "o",
	"ù", "u", "ú", "u", "û", "u", "ü", "u",
	"ý", "y", "ÿ", "y",
	"œ", "oe", "æ", "ae",
)

// NormalizeHeader ramene un en-tete a une forme comparable : minuscules, sans
// accents, ponctuation et espaces repetes remplaces par un espace.
// « Durée (min) » et « DUREE  MIN » donnent « duree min ».
func NormalizeHeader(header string) string {
	s := accentFolds.Replace(strings.ToLower(strings.TrimPrefix(header, BOM)))

	var b strings.Builder

	space := false

	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}

			space = false

			b.WriteRune(r)

			continue
		}

		space = true
	}

	return b.String()
}
