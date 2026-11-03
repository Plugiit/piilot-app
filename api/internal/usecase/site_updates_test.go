package usecase

import "testing"

func TestGithubRepoOf(t *testing.T) {
	cases := []struct {
		in          string
		owner, repo string
		ok          bool
	}{
		{"https://github.com/Agence/Site.git", "agence", "site", true},
		{"git@github.com:agence/site.git", "agence", "site", true},
		{"https://gitlab.com/agence/site", "", "", false},
		{"https://github.com/agence", "", "", false},
		{"https://github.com/agence/site/autre", "", "", false},
		{"https://github.com.evil.test/agence/site", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		owner, repo, ok := githubRepoOf(c.in)
		if owner != c.owner || repo != c.repo || ok != c.ok {
			t.Errorf("%q : %q %q %v", c.in, owner, repo, ok)
		}
	}
}

func TestCibleDesMisesAJourValideSaSaisie(t *testing.T) {
	if !workflowName.MatchString("piilot-update.yml") || !workflowName.MatchString("update.yaml") {
		t.Error("un nom de workflow valide est refuse")
	}
	for _, bad := range []string{"", "../evil.yml", "a/b.yml", "update", "update.sh"} {
		if workflowName.MatchString(bad) {
			t.Errorf("workflow accepte : %q", bad)
		}
	}
	for _, good := range []string{"main", "release/2031", "v1.2"} {
		if !validRef(good) {
			t.Errorf("branche refusee : %q", good)
		}
	}
	for _, bad := range []string{"", "..", "a..b", "/main", "main/", "ma in", "main;rm"} {
		if validRef(bad) {
			t.Errorf("branche acceptee : %q", bad)
		}
	}
}

func TestDerniereExecutionDesMisesAJourRetient(t *testing.T) {
	cases := []struct {
		name string
		data ReportData
		want bool
	}{
		{"robot seul", ReportData{Updates: &ReportUpdates{DependencyCount: 1}}, false},
		{"lancement reussi en production", ReportData{Updates: &ReportUpdates{Runs: []ReportUpdateRun{{Environment: "prod"}}}}, false},
		{"la preproduction ne vaut pas la production", ReportData{Updates: &ReportUpdates{Runs: []ReportUpdateRun{{Environment: "preprod"}}}}, true},
		{"rien", ReportData{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reasons := holdReasonsOf(c.data, []string{"updates"}, 1)
			if got := len(reasons) == 1 && reasons[0] == holdUpdatesMissing; got != c.want {
				t.Errorf("motifs : %v", reasons)
			}
		})
	}
	failed := holdReasonsOf(ReportData{Updates: &ReportUpdates{DependencyCount: 2, LastRunFailed: true}}, []string{"updates"}, 1)
	if len(failed) != 1 || failed[0] != holdUpdateFailed {
		t.Errorf("dernier lancement en echec : %v", failed)
	}
}
