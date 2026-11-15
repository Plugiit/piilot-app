package gitsource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const repo = "https://github.com/agence/site"

func day(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }

func pull(n int, merged, updated time.Time, extra string) string {
	m := "null"
	if !merged.IsZero() {
		m = `"` + merged.Format(time.RFC3339) + `"`
	}
	return fmt.Sprintf(`{"number":%d,"title":"PR %d","html_url":"https://github.com/agence/site/pull/%d","created_at":"%s","updated_at":"%s","merged_at":%s,"user":{"login":"renovate[bot]"},"head":{"ref":"renovate/x-%d"},"labels":[{"name":"dependencies"},{"name":"security"}]%s}`,
		n, n, n, day(1).Format(time.RFC3339), updated.Format(time.RFC3339), m, n, extra)
}

func serve(t *testing.T, h http.HandlerFunc) *GitHub {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer jeton" {
			t.Errorf("Authorization = %q", got)
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return &GitHub{Token: "jeton", BaseURL: srv.URL}
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestMergedPullRequestsFiltersOnMergeDate(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/agence/site/pulls" {
			t.Errorf("chemin = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("state") != "closed" || q.Get("sort") != "updated" || q.Get("direction") != "desc" {
			t.Errorf("requete = %v", q)
		}
		reply(200, "["+strings.Join([]string{
			pull(1, day(20), day(21), ""),
			pull(2, time.Time{}, day(19), ""), // fermee sans fusion
			pull(3, day(10), day(10), ""),     // avant la periode
			pull(4, day(28), day(28), ""),     // apres la periode
			pull(5, day(15), day(16), ""),
		}, ",")+"]")(w, r)
	})
	got, err := c.MergedPullRequests(context.Background(), repo, day(12), day(25))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Number != 1 || got[1].Number != 5 {
		t.Fatalf("%+v", got)
	}
	p := got[0]
	if p.Author != "renovate[bot]" || p.Branch != "renovate/x-1" || p.Title != "PR 1" || len(p.Labels) != 2 || p.Labels[0] != "dependencies" ||
		!p.MergedAt.Equal(day(20)) || p.OpenedAt == nil || p.URL == "" {
		t.Errorf("champs = %+v", p)
	}
}

func TestMergedPullRequestsFollowsLinkAndStopsOnOlderPage(t *testing.T) {
	calls := 0
	var c *GitHub
	c = serve(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := r.URL.Query().Get("page")
		switch page {
		case "":
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/agence/site/pulls?page=2>; rel="next", <%s/repos/agence/site/pulls?page=9>; rel="last"`, c.BaseURL, c.BaseURL))
			reply(200, "["+pull(1, day(24), day(24), "")+","+pull(2, day(23), day(23), "")+"]")(w, r)
		case "2":
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/agence/site/pulls?page=3>; rel="next"`, c.BaseURL))
			// Une page qui garde une entree recente : la lecture continue.
			reply(200, "["+pull(3, day(13), day(22), "")+"]")(w, r)
		case "3":
			// Page entierement plus ancienne que since : la lecture s'arrete ici.
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/agence/site/pulls?page=4>; rel="next"`, c.BaseURL))
			reply(200, "["+pull(4, day(5), day(6), "")+"]")(w, r)
		default:
			t.Errorf("page %s lue alors que la precedente etait entierement ancienne", page)
		}
	})
	got, err := c.MergedPullRequests(context.Background(), repo, day(12), day(30))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || calls != 3 {
		t.Fatalf("%d pull requests, %d appels", len(got), calls)
	}
}

func TestMergedPullRequestsDoesNotDependOnOrderInsideAPage(t *testing.T) {
	// Une entree ancienne au milieu d'une page ne coupe pas la lecture.
	calls := 0
	var c *GitHub
	c = serve(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("page") == "" {
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/agence/site/pulls?page=2>; rel="next"`, c.BaseURL))
			reply(200, "["+pull(1, day(24), day(24), "")+","+pull(2, day(2), day(3), "")+","+pull(3, day(20), day(20), "")+"]")(w, r)
			return
		}
		reply(200, "["+pull(4, day(18), day(18), "")+"]")(w, r)
	})
	got, err := c.MergedPullRequests(context.Background(), repo, day(12), day(30))
	if err != nil || len(got) != 3 || calls != 2 {
		t.Fatalf("%d pull requests, %d appels, err = %v", len(got), calls, err)
	}
}

func TestMergedPullRequestsCapFailsInsteadOfTruncating(t *testing.T) {
	calls := 0
	var c *GitHub
	c = serve(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Link", fmt.Sprintf(`<%s/repos/agence/site/pulls?page=%d>; rel="next"`, c.BaseURL, calls+1))
		reply(200, "["+pull(calls, day(20), day(20), "")+"]")(w, r)
	})
	_, err := c.MergedPullRequests(context.Background(), repo, day(1), day(30))
	if !errors.Is(err, ErrTooMany) || calls != maxPages {
		t.Fatalf("err = %v, %d appels", err, calls)
	}
}

func TestMergedPullRequestsRefusesForeignNextLink(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://evil.example/steal>; rel="next"`)
		reply(200, "["+pull(1, day(24), day(24), "")+"]")(w, r)
	})
	if _, err := c.MergedPullRequests(context.Background(), repo, day(1), day(30)); err == nil {
		t.Fatal("un lien vers un autre hote a ete suivi")
	}
}

func TestMergedPullRequestsErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		body    string
		header  map[string]string
		want    error
		contain string
	}{
		"401":            {401, `{"message":"Bad credentials"}`, nil, ErrUnauthorized, ""},
		"403 droit":      {403, `{"message":"Resource not accessible by personal access token"}`, nil, ErrForbidden, "Resource not accessible"},
		"403 debit":      {403, `{"message":"API rate limit exceeded"}`, map[string]string{"X-RateLimit-Remaining": "0"}, nil, "limite de débit"},
		"429":            {429, `{}`, nil, nil, "limite de débit"},
		"404":            {404, `{"message":"Not Found"}`, nil, ErrRepoNotFound, ""},
		"500":            {500, `oops`, nil, nil, "500"},
		"json illisible": {200, `<html>`, nil, nil, "inattendue"},
	} {
		t.Run(name, func(t *testing.T) {
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.header {
					w.Header().Set(k, v)
				}
				reply(tc.status, tc.body)(w, r)
			})
			_, err := c.MergedPullRequests(context.Background(), repo, day(1), day(30))
			if err == nil {
				t.Fatal("pas d'erreur")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v", err)
			}
			if tc.contain != "" && !strings.Contains(err.Error(), tc.contain) {
				t.Errorf("message = %q", err)
			}
			if strings.Contains(err.Error(), "jeton\"") {
				t.Errorf("le jeton fuit : %q", err)
			}
		})
	}
}

func TestMergedPullRequestsRejectsNonGitHubRepo(t *testing.T) {
	c := &GitHub{Token: "jeton", BaseURL: "http://127.0.0.1:1"}
	for _, r := range []string{"https://gitlab.com/agence/site", "https://github.com/agence", "https://github.com/a/b/c", "", "agence/site"} {
		if _, err := c.MergedPullRequests(context.Background(), r, day(1), day(30)); !errors.Is(err, ErrUnsupportedRepo) {
			t.Errorf("%q : %v", r, err)
		}
	}
}

func TestGitLabIsNotImplemented(t *testing.T) {
	if _, err := (GitLab{}).MergedPullRequests(context.Background(), "https://gitlab.com/a/b", day(1), day(2)); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
}

func TestNextLink(t *testing.T) {
	for in, want := range map[string]string{
		"":                                 "",
		`<https://a/b?page=2>; rel="next"`: "https://a/b?page=2",
		`<https://a/b?page=1>; rel="prev", <https://a/b?page=3>; rel="next"`: "https://a/b?page=3",
		`<https://a/b?page=9>; rel="last"`:                                   "",
	} {
		if got := nextLink(in); got != want {
			t.Errorf("%q -> %q", in, got)
		}
	}
}
