package config

import "testing"

func TestTrackerValidate(t *testing.T) {
	const id = "7b376ab6-cf03-4a00-9831-c5ea22aa570e"

	cases := []struct {
		name    string
		tracker TrackerEnv
		ok      bool
	}{
		{"absent", TrackerEnv{}, true},
		{"complet", TrackerEnv{ScriptURL: "https://umami.agence.fr/script.js", WebsiteID: id}, true},
		{"adresse sans identifiant", TrackerEnv{ScriptURL: "https://umami.agence.fr/script.js"}, false},
		{"identifiant sans adresse", TrackerEnv{WebsiteID: id}, false},
		{"adresse en clair", TrackerEnv{ScriptURL: "http://umami.agence.fr/script.js", WebsiteID: id}, false},
		{"adresse relative", TrackerEnv{ScriptURL: "/script.js", WebsiteID: id}, false},
		{"identifiant qui n'est pas un UUID", TrackerEnv{ScriptURL: "https://umami.agence.fr/script.js", WebsiteID: `x" onload="alert(1)`}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tracker.validate()
			if tc.ok && err != nil {
				t.Fatalf("refuse a tort : %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("accepte a tort")
			}
		})
	}
}
