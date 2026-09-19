package handler

import "testing"

func TestCSVCellNeutraliseLesFormules(t *testing.T) {
	cases := map[string]string{
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+33 6 12":          "'+33 6 12",
		"-2":                "'-2",
		"@SUM(A1)":          "'@SUM(A1)",
		"Réunion client":    "Réunion client",
		"":                  "",
	}
	for in, want := range cases {
		if got := csvCell(in); got != want {
			t.Errorf("csvCell(%q) = %q, attendu %q", in, got, want)
		}
	}
}
