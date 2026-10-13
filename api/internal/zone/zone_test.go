package zone

import (
	"testing"
	"time"
)

func TestLeFuseauParDefautPuisRegle(t *testing.T) {
	if Name() != Default {
		t.Fatalf("par defaut : %s", Name())
	}
	montreal, err := time.LoadLocation("America/Montreal")
	if err != nil {
		t.Skip("base des fuseaux absente")
	}
	Set(montreal)
	defer Set(func() *time.Location { l, _ := time.LoadLocation(Default); return l }())
	if Name() != "America/Montreal" {
		t.Errorf("apres reglage : %s", Name())
	}
	Set(nil)
	if Name() != "America/Montreal" {
		t.Error("un fuseau nul a efface le reglage")
	}
}
