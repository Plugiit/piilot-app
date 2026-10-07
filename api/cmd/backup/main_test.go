package main

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	now := time.Date(2026, 10, 7, 10, 0, 0, 0, paris)

	if got := nextRun(now, "03:00"); got.Day() != 8 || got.Hour() != 3 {
		t.Errorf("apres l'heure : %v", got)
	}
	if got := nextRun(now, "22:30"); got.Day() != 7 || got.Hour() != 22 || got.Minute() != 30 {
		t.Errorf("avant l'heure : %v", got)
	}
	if got := nextRun(now, "10:00"); got.Day() != 8 {
		t.Errorf("pile l'heure : %v", got)
	}
}
