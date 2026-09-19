package usecase

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestBucketForSuitLaLongueurDeLaPeriode(t *testing.T) {
	cases := []struct {
		from, to, want string
	}{
		{"2026-09-01", "2026-09-30", "day"},
		{"2026-07-01", "2026-09-30", "week"},
		{"2026-01-01", "2026-12-31", "month"},
	}
	for _, c := range cases {
		if got := bucketFor(day(c.from), day(c.to)); got != c.want {
			t.Errorf("%s -> %s : %s, attendu %s", c.from, c.to, got, c.want)
		}
	}
}

func TestBucketStartCommeDateTrunc(t *testing.T) {
	// 2026-09-19 est un samedi : sa semaine commence le lundi 14.
	if got := bucketStart(day("2026-09-19"), "week").Format(dateLayout); got != "2026-09-14" {
		t.Errorf("semaine : %s", got)
	}
	// Un lundi est son propre debut de semaine.
	if got := bucketStart(day("2026-09-14"), "week").Format(dateLayout); got != "2026-09-14" {
		t.Errorf("lundi : %s", got)
	}
	if got := bucketStart(day("2026-09-19"), "month").Format(dateLayout); got != "2026-09-01" {
		t.Errorf("mois : %s", got)
	}
}

func TestPeriodeBornee(t *testing.T) {
	ok := TimeReportFilters{From: day("2026-01-01"), To: day("2026-12-31")}
	if err := ok.validate(); err != nil {
		t.Errorf("une annee doit passer : %v", err)
	}

	tooLong := TimeReportFilters{From: day("2025-01-01"), To: day("2026-01-02")}
	if err := tooLong.validate(); err == nil {
		t.Error("plus d'un an doit etre refuse")
	}

	reversed := TimeReportFilters{From: day("2026-09-30"), To: day("2026-09-01")}
	if err := reversed.validate(); err == nil {
		t.Error("une fin avant le debut doit etre refusee")
	}
}
