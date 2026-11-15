package csvio

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func readAll(t *testing.T, in string) (*Reader, []Row) {
	t.Helper()

	r, err := NewReader(strings.NewReader(in))
	if err != nil {
		t.Fatalf("NewReader : %v", err)
	}

	var rows []Row

	for {
		row, err := r.Next()
		if errors.Is(err, io.EOF) {
			return r, rows
		}

		if err != nil {
			t.Fatalf("Next : %v", err)
		}

		rows = append(rows, row)
	}
}

func TestReaderDelimiter(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want rune
	}{
		{"point-virgule", "a;b;c\n1;2;3\n", ';'},
		{"virgule", "a,b,c\n1,2,3\n", ','},
		{"tabulation", "a\tb\tc\n1\t2\t3\n", '\t'},
		{"un seul champ", "a\n1\n", ';'},
		{"egalite", "a;b,c\n1;2,3\n", ';'},
		{"virgule dans des guillemets", "\"Nom, prenom\";Email\n\"Dupont, Jean\";j@x.fr\n", ';'},
		{"decimaux a la virgule, separateur point-virgule", "a;b\n1,5;2,5\n", ';'},
		{"crlf", "a,b\r\n1,2\r\n", ','},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, rows := readAll(t, tc.in)
			if r.Delimiter() != tc.want {
				t.Fatalf("separateur %q, attendu %q", r.Delimiter(), tc.want)
			}

			if len(rows) != 1 {
				t.Fatalf("%d lignes, attendu 1", len(rows))
			}
		})
	}
}

func TestReaderBOMAndEncoding(t *testing.T) {
	cp1252 := []byte("Nom;Ville\r\nCaf\xe9 \x80uro;\x93Nancy\x94\r\n")

	cases := []struct {
		name     string
		in       []byte
		encoding string
		headers  []string
		row      []string
	}{
		{"utf-8 sans bom", []byte("Nom;Ville\nCafé;Nancy\n"), EncodingUTF8, []string{"Nom", "Ville"}, []string{"Café", "Nancy"}},
		{"utf-8 avec bom", append([]byte(BOM), "Nom;Ville\nCafé;Nancy\n"...), EncodingUTF8, []string{"Nom", "Ville"}, []string{"Café", "Nancy"}},
		{"windows-1252", cp1252, EncodingWindows1252, []string{"Nom", "Ville"}, []string{"Café €uro", "“Nancy”"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewReader(bytes.NewReader(tc.in))
			if err != nil {
				t.Fatal(err)
			}

			if r.Encoding() != tc.encoding {
				t.Errorf("encodage %q, attendu %q", r.Encoding(), tc.encoding)
			}

			if !reflect.DeepEqual(r.Headers(), tc.headers) {
				t.Errorf("en-tetes %q, attendu %q", r.Headers(), tc.headers)
			}

			row, err := r.Next()
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(row.Cells, tc.row) {
				t.Errorf("ligne %q, attendu %q", row.Cells, tc.row)
			}
		})
	}
}

func TestReaderRefuses(t *testing.T) {
	if _, err := NewReader(strings.NewReader("")); !errors.Is(err, ErrEmpty) {
		t.Errorf("fichier vide : %v", err)
	}

	if _, err := NewReader(strings.NewReader(BOM + "  \n")); !errors.Is(err, ErrEmpty) {
		t.Errorf("fichier blanc : %v", err)
	}

	if _, err := NewReader(bytes.NewReader([]byte{0xFF, 0xFE, 'a', 0})); !errors.Is(err, ErrUnsupportedEncoding) {
		t.Errorf("utf-16 : %v", err)
	}
}

func TestReaderLineNumbers(t *testing.T) {
	_, rows := readAll(t, "a;b\n1;2\n\n\"x\ny\";3\n4;5\n")

	want := []int{2, 4, 6}
	for i, row := range rows {
		if row.Line != want[i] {
			t.Errorf("ligne %d : numero %d, attendu %d", i, row.Line, want[i])
		}
	}

	if len(rows) != len(want) {
		t.Fatalf("%d lignes, attendu %d", len(rows), len(want))
	}
}

func TestReaderBadLineDoesNotStopTheRest(t *testing.T) {
	r, err := NewReader(strings.NewReader("a;b\n1;\"oops\n2;3\n"))
	if err != nil {
		t.Fatal(err)
	}

	// Un guillemet jamais ferme consomme la fin du fichier : on veut une
	// erreur avec un numero de ligne, pas un silence.
	if _, err := r.Next(); err == nil || !strings.Contains(err.Error(), "ligne 2") {
		t.Fatalf("erreur attendue sur la ligne 2, obtenu %v", err)
	}
}

func TestReaderFormulaPrefix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"'=SUM(A1)", "=SUM(A1)"},
		{"'+33612345678", "+33612345678"},
		{"'-5", "-5"},
		{"'@nom", "@nom"},
		{"'=x  ", "=x"},
		{"''=x", "'=x"},
		{"'bonjour", "'bonjour"},
		{"'", "'"},
		{"  Dupont  ", "Dupont"},
		{"=brut", "=brut"},
	}

	for _, tc := range cases {
		_, rows := readAll(t, "a\n"+quote(tc.in)+"\n")
		if got := rows[0].Cells[0]; got != tc.want {
			t.Errorf("%q : %q, attendu %q", tc.in, got, tc.want)
		}
	}
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func TestNormalizeHeader(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Durée (min)", "duree min"},
		{"DUREE  MIN", "duree min"},
		{"  E-mail ", "e mail"},
		{"Identifiant d'origine", "identifiant d origine"},
		{"SIRET client", "siret client"},
		{"Prénom", "prenom"},
		{BOM + "Client", "client"},
		{"Œuvre", "oeuvre"},
		{"", ""},
		{"---", ""},
	}

	for _, tc := range cases {
		if got := NormalizeHeader(tc.in); got != tc.want {
			t.Errorf("%q : %q, attendu %q", tc.in, got, tc.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	ok := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

	cases := []struct {
		in      string
		want    time.Time
		wantErr bool
	}{
		{"2026-11-15", ok(2026, 11, 15), false},
		{"15/11/2026", ok(2026, 11, 15), false},
		{"5/1/2026", ok(2026, 1, 5), false},
		{" 05/01/2026 ", ok(2026, 1, 5), false},
		{"2024-02-29", ok(2024, 2, 29), false},
		{"2026-02-29", time.Time{}, true},
		{"31/04/2026", time.Time{}, true},
		{"11/15/2026", time.Time{}, true},
		{"15/11/26", time.Time{}, true},
		{"2026-1-5", time.Time{}, true},
		{"", time.Time{}, true},
		{"demain", time.Time{}, true},
	}

	for _, tc := range cases {
		got, err := ParseDate(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("%q : erreur %v", tc.in, err)

			continue
		}

		if !tc.wantErr && !got.Equal(tc.want) {
			t.Errorf("%q : %v, attendu %v", tc.in, got, tc.want)
		}
	}
}

func TestParseDecimal(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"1,5", 1.5, false},
		{"1.5", 1.5, false},
		{"12", 12, false},
		{"-3,25", -3.25, false},
		{"1 234,56", 1234.56, false},
		{"1 234,56", 1234.56, false},
		{"1.234,56", 1234.56, false},
		{"1,234.56", 1234.56, false},
		{",5", 0.5, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1e5", 0, true},
		{"inf", 0, true},
		{"NaN", 0, true},
		{"1,2,3", 0, true},
		{"1.2.3", 0, true},
		{"--1", 0, true},
		{"1-", 0, true},
		{",", 0, true},
	}

	for _, tc := range cases {
		got, err := ParseDecimal(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("%q : erreur %v", tc.in, err)

			continue
		}

		if !tc.wantErr && got != tc.want {
			t.Errorf("%q : %v, attendu %v", tc.in, got, tc.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"90", 90, false},
		{"1h30", 90, false},
		{"1h", 60, false},
		{"1H30", 90, false},
		{"1h30m", 90, false},
		{"1 h 30", 90, false},
		{"2h05", 125, false},
		{"45m", 45, false},
		{"45min", 45, false},
		{"1,5", 90, false},
		{"1.5", 90, false},
		{"0,25", 15, false},
		{"0", 0, false},
		{"", 0, true},
		{"1h75", 0, true},
		{"-30", 0, true},
		{"-1,5", 0, true},
		{"h30", 0, true},
		{"abc", 0, true},
		{"1h30x", 0, true},
		{"1e2", 0, true},
	}

	for _, tc := range cases {
		got, err := ParseDuration(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("%q : erreur %v", tc.in, err)

			continue
		}

		if !tc.wantErr && got != tc.want {
			t.Errorf("%q : %d, attendu %d", tc.in, got, tc.want)
		}
	}
}

func TestCell(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"Dupont", "Dupont"},
		{"=SUM(A1)", "'=SUM(A1)"},
		{"+33612345678", "'+33612345678"},
		{"-5", "'-5"},
		{"@nom", "'@nom"},
		{"\tx", "'\tx"},
		{"\rx", "'\rx"},
		{"a=b", "a=b"},
		{"'=x", "''=x"},
		{"'bonjour", "'bonjour"},
	}

	for _, tc := range cases {
		if got := Cell(tc.in); got != tc.want {
			t.Errorf("%q : %q, attendu %q", tc.in, got, tc.want)
		}
	}
}

func TestWriterFormat(t *testing.T) {
	var buf bytes.Buffer

	w := NewWriter(&buf)

	if buf.Len() != 0 {
		t.Fatal("rien ne doit etre ecrit avant le premier enregistrement")
	}

	_ = w.Write([]string{"a", "b;c"})
	_ = w.Write([]string{"1", "2"})

	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	want := BOM + "a;\"b;c\"\r\n1;2\r\n"
	if buf.String() != want {
		t.Errorf("%q, attendu %q", buf.String(), want)
	}
}

func TestFormatDecimal(t *testing.T) {
	if got := FormatDecimal(1.5, 2); got != "1,50" {
		t.Errorf("%q", got)
	}

	if got := FormatDecimal(-0.25, 2); got != "-0,25" {
		t.Errorf("%q", got)
	}
}

// Aller-retour : ce que l'export ecrit, l'import le relit a l'identique,
// y compris les valeurs qui cassent un CSV naif ou declenchent un tableur.
func TestRoundTripTrickyValues(t *testing.T) {
	values := []string{
		"simple",
		"point;virgule",
		"virgule, ici",
		`guillemet " au milieu`,
		`"entre guillemets"`,
		"deux\nlignes",
		"=SUM(A1:A9)",
		"+33 6 12 34 56 78",
		"-5",
		"@mention",
		"=HYPERLINK(\"http://x\";\"clic\")",
		"\ttabulation en tete",
		"Café Crème – œuvre",
		"a=b",
		"  espaces autour  ",
		"",
		"'apostrophe seule",
		"dernier",
	}

	// Les espaces en bordure sont rognes a la lecture (voir cleanCell) : on
	// compare a la valeur rognee, sauf pour les valeurs a declencheur.
	expect := func(v string) string {
		if v != "" && strings.ContainsRune(formulaTriggers, rune(v[0])) {
			return v
		}

		return strings.TrimSpace(v)
	}

	var buf bytes.Buffer

	w := NewWriter(&buf)
	_ = w.Write([]string{"A", "B", "C"})

	for i := 0; i+2 < len(values); i += 3 {
		_ = w.Write([]string{Cell(values[i]), Cell(values[i+1]), Cell(values[i+2])})
	}

	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if r.Encoding() != EncodingUTF8 || r.Delimiter() != ';' {
		t.Fatalf("encodage %q, separateur %q", r.Encoding(), r.Delimiter())
	}

	i := 0

	for {
		row, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatal(err)
		}

		for _, got := range row.Cells {
			if want := expect(values[i]); got != want {
				t.Errorf("valeur %d : %q, attendu %q", i, got, want)
			}

			i++
		}
	}

	if i != (len(values)/3)*3 {
		t.Errorf("%d valeurs relues sur %d", i, (len(values)/3)*3)
	}
}

// Une seule valeur par ligne pour ne rien perdre au decoupage par trois.
func TestRoundTripEachValueAlone(t *testing.T) {
	for _, v := range []string{"=1+1", "-5", "@x", "a;b", "l1\nl2", `"q"`, "'bonjour", "'=x", "''-5", "'"} {
		var buf bytes.Buffer

		w := NewWriter(&buf)
		_ = w.Write([]string{"A"})
		_ = w.Write([]string{Cell(v)})
		_ = w.Flush()

		_, rows := readAll(t, buf.String())
		if rows[0].Cells[0] != v {
			t.Errorf("%q relu %q", v, rows[0].Cells[0])
		}
	}
}
