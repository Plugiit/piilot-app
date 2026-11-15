package usecase

import (
	"reflect"
	"testing"
	"time"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

func intPtr(v int) *int { return &v }

func TestHoldReasonsDesSauvegardesDomainesEtMisesAJour(t *testing.T) {
	cases := []struct {
		name     string
		data     ReportData
		sections []string
		want     []string
	}{
		{"sauvegardes non surveillees", ReportData{}, []string{"backups"}, []string{holdBackupMissing}},
		{"sauvegardes en defaut", ReportData{Backups: &ReportBackups{Successes: 3}}, []string{"backups"}, []string{holdBackupStale}},
		{"sauvegardes saines", ReportData{Backups: &ReportBackups{Successes: 30, Healthy: true}}, []string{"backups"}, []string{}},
		{"aucun domaine", ReportData{}, []string{"domain"}, []string{holdDomainMissing}},
		{"echeance inconnue", ReportData{Domain: &ReportDomains{Items: []ReportDomain{{Domain: "a.fr"}}}}, []string{"domain"}, []string{holdDomainMissing}},
		{"echeance proche", ReportData{Domain: &ReportDomains{Items: []ReportDomain{{Domain: "a.fr", DaysLeft: intPtr(12)}}}}, []string{"domain"}, []string{holdDomainExpiring}},
		{"echeance lointaine", ReportData{Domain: &ReportDomains{Items: []ReportDomain{{Domain: "a.fr", DaysLeft: intPtr(200)}}}}, []string{"domain"}, []string{}},
		{"aucune mise a jour", ReportData{}, []string{"updates"}, []string{holdUpdatesMissing}},
		{"que des fusions ordinaires", ReportData{Updates: &ReportUpdates{OtherCount: 4}}, []string{"updates"}, []string{holdUpdatesMissing}},
		{"des mises a jour", ReportData{Updates: &ReportUpdates{DependencyCount: 2}}, []string{"updates"}, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := holdReasonsOf(c.data, c.sections, 1); !reflect.DeepEqual(got, c.want) {
				t.Errorf("motifs : %v, attendu %v", got, c.want)
			}
		})
	}
}

func TestReconnaitLesMisesAJourDeDependances(t *testing.T) {
	cases := []struct {
		name           string
		author, branch string
		labels         []string
		want           bool
	}{
		{"auteur renovate", "renovate[bot]", "chore/x", nil, true},
		{"auteur dependabot", "dependabot[bot]", "x", nil, true},
		{"branche renovate", "max", "renovate/lodash", nil, true},
		{"branche dependabot", "max", "dependabot/npm_and_yarn/vite-7.0.0", nil, true},
		{"majuscules", "Renovate[bot]", "Renovate/x", nil, true},
		{"label dependencies", "max", "fix/panier", []string{"bug", "dependencies"}, true},
		{"label dependencies en majuscules", "max", "fix/panier", []string{" Dependencies "}, true},
		{"autre label", "max", "fix/panier", []string{"bug", "dependency-graph"}, false},
		{"ordinaire", "max", "fix/panier", nil, false},
		{"vide", "", "", nil, false},
	}
	for _, c := range cases {
		if got := isDependencyUpdate(c.author, c.branch, c.labels); got != c.want {
			t.Errorf("%s : %v", c.name, got)
		}
	}
}

func TestReconnaitLesCorrectifsDeSecurite(t *testing.T) {
	cases := []struct {
		name    string
		flagged bool
		labels  []string
		title   string
		want    bool
	}{
		{"label security", false, []string{"security"}, "x", true},
		{"label compose", false, []string{"type: Security"}, "x", true},
		{"label securite", false, []string{"sécurité"}, "x", true},
		{"drapeau du webhook", true, nil, "x", true},
		{"titre [SECURITY]", false, nil, "chore(deps): update lodash [SECURITY]", true},
		{"titre CVE", false, nil, "fix CVE-2031-1234", true},
		{"titre GHSA", false, nil, "Bump foo (GHSA-abcd-1234-efgh)", true},
		{"mise a jour ordinaire", false, []string{"dependencies"}, "chore(deps): update react", false},
		{"dependabot sans label", false, nil, "Bump lodash from 4.17.20 to 4.17.21", false},
		{"vide", false, nil, "", false},
	}
	for _, c := range cases {
		if got := isSecurityUpdate(c.flagged, c.labels, c.title); got != c.want {
			t.Errorf("%s : %v", c.name, got)
		}
	}
}

func TestCleanDomain(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"exemple.fr", "exemple.fr", true},
		{" https://www.Exemple.FR/page?x=1 ", "exemple.fr", true},
		{"sous.exemple.co.uk", "sous.exemple.co.uk", true},
		{"exemple", "exemple", false},
		{"http://localhost", "localhost", false},
		{"127.0.0.1", "127.0.0.1", false},
		{"exemple.fr/../../admin", "exemple.fr", true},
		{"a b.fr", "a b.fr", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := cleanDomain(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%q : %q, %v", c.in, got, ok)
		}
	}
}

func TestLesPaliersDAlerteNeSeDisentQuUneFois(t *testing.T) {
	last := func(v int32) *int32 { return &v }
	cases := []struct {
		name     string
		days     int
		last     *int32
		wantStep int
		wantDue  bool
	}{
		{"loin de l'echeance", 90, nil, 0, false},
		{"premier palier", 60, nil, 60, true},
		{"deja annonce", 55, last(60), 60, false},
		{"palier suivant", 30, last(60), 30, true},
		{"palier saute : seul le plus proche", 6, last(60), 7, true},
		{"expire", -2, last(7), 1, true},
		{"expire, deja dit", -3, last(1), 1, false},
		{"renouvele", 365, last(7), 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			step, due := domainAlertStep(c.days, c.last)
			if step != c.wantStep || due != c.wantDue {
				t.Errorf("palier %d, a dire %v ; attendu %d, %v", step, due, c.wantStep, c.wantDue)
			}
		})
	}
}

func TestBackupHealthy(t *testing.T) {
	now := time.Date(2031, 3, 5, 12, 0, 0, 0, time.UTC)
	at := func(h int) *time.Time { v := now.Add(-time.Duration(h) * time.Hour); return &v }
	cases := []struct {
		name string
		row  db.SiteBackupSource
		want bool
	}{
		{"jamais reussie", db.SiteBackupSource{ExpectedHours: 26}, false},
		{"recente", db.SiteBackupSource{ExpectedHours: 26, LastSuccessAt: at(2)}, true},
		{"trop ancienne", db.SiteBackupSource{ExpectedHours: 26, LastSuccessAt: at(30)}, false},
		{"echec depuis", db.SiteBackupSource{ExpectedHours: 26, LastSuccessAt: at(5), LastFailureAt: at(1)}, false},
		{"echec avant", db.SiteBackupSource{ExpectedHours: 26, LastSuccessAt: at(1), LastFailureAt: at(5)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := backupHealthy(c.row, now); got != c.want {
				t.Errorf("%v", got)
			}
		})
	}
}
