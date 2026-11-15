package coolify

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

const token = "3|abcdef"

// serve sert un faux Coolify qui verifie le jeton et rend la reponse donnee.
func serve(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("Authorization = %q", got)
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL + "/", Token: token}
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestListApplications200(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications" {
			t.Errorf("chemin = %s", r.URL.Path)
		}
		reply(200, `[
			{"uuid":"a1","name":"Site vitrine","fqdn":"https://vitrine.test","environment":{"name":"production","project":{"name":"Client A"}}},
			{"uuid":"a2","name":"API","environment_name":"staging","project_name":"Client B"},
			{"name":"sans uuid"}
		]`)(w, r)
	})
	apps, err := c.ListApplications(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("%d applications, attendu 2 (celle sans uuid est ignoree)", len(apps))
	}
	if a := apps[0]; a.UUID != "a1" || a.Name != "Site vitrine" || a.FQDN != "https://vitrine.test" || a.EnvironmentName != "production" || a.ProjectName != "Client A" {
		t.Errorf("a1 = %+v", a)
	}
	if a := apps[1]; a.EnvironmentName != "staging" || a.ProjectName != "Client B" || a.FQDN != "" {
		t.Errorf("a2 = %+v", a)
	}
}

func TestBaseURLAcceptsAPIV1Suffix(t *testing.T) {
	c := serve(t, reply(200, `[]`))
	c.BaseURL = strings.TrimSuffix(c.BaseURL, "/") + "/api/v1/"
	if _, err := c.ListApplications(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestUnauthorized401(t *testing.T) {
	c := serve(t, reply(401, `{"message":"Unauthenticated."}`))
	if _, err := c.ListApplications(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("erreur = %v", err)
	}
	if _, err := c.ListDeployments(context.Background(), "a1", time.Time{}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("erreur = %v", err)
	}
}

func TestForbidden403(t *testing.T) {
	c := serve(t, reply(403, `{"message":"You are not allowed to access the API."}`))
	_, err := c.ListApplications(context.Background())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("erreur = %v", err)
	}
	if !strings.Contains(err.Error(), "not allowed") || strings.Contains(err.Error(), token) {
		t.Errorf("message = %q", err)
	}
}

func TestOtherStatus(t *testing.T) {
	c := serve(t, reply(502, `bad gateway`))
	_, err := c.ListApplications(context.Background())
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("erreur = %v", err)
	}
}

func TestUnexpectedJSON(t *testing.T) {
	for name, body := range map[string]string{
		"html":             `<html>login</html>`,
		"objet sans liste": `{"foo":"bar"}`,
		"nombre":           `42`,
	} {
		t.Run(name, func(t *testing.T) {
			c := serve(t, reply(200, body))
			if _, err := c.ListApplications(context.Background()); err == nil {
				t.Fatal("une reponse inattendue n'a pas ete signalee")
			}
		})
	}
}

func TestToleratesWrongTypes(t *testing.T) {
	c := serve(t, reply(200, `[{"uuid":"a1","name":12,"fqdn":null,"environment":"prod"}, "texte", 3]`))
	apps, err := c.ListApplications(context.Background())
	if err != nil || len(apps) != 1 || apps[0].UUID != "a1" || apps[0].Name != "" {
		t.Fatalf("apps = %+v, err = %v", apps, err)
	}
}

func TestListDeploymentsObjectAndMissingFields(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/deployments/applications/a1" {
			t.Errorf("chemin = %s", r.URL.Path)
		}
		if q := r.URL.Query(); q.Get("skip") != "0" || q.Get("take") != "100" {
			t.Errorf("pagination = %v", q)
		}
		reply(200, `{"count":3,"deployments":[
			{"deployment_uuid":"d3","status":"finished","git_commit_sha":"abc123","commit_message":"fix: x","created_at":"2026-11-03T10:00:00.000000Z","updated_at":"2026-11-03T10:02:00.000000Z"},
			{"status":"failed","created_at":"2026-11-02 08:00:00"},
			{"status":"finished"}
		]}`)(w, r)
	})
	got, err := c.ListDeployments(context.Background(), "a1", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("%d deploiements", len(got))
	}
	d := got[0]
	if d.UUID != "d3" || d.Status != "finished" || d.CommitSHA != "abc123" || d.CommitMessage != "fix: x" ||
		!d.CreatedAt.Equal(time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)) || !d.UpdatedAt.Equal(time.Date(2026, 11, 3, 10, 2, 0, 0, time.UTC)) {
		t.Errorf("d3 = %+v", d)
	}
	if got[1].CommitSHA != "" || got[1].UUID != "" || !got[1].CreatedAt.Equal(time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("deploiement partiel = %+v", got[1])
	}
	if !got[2].CreatedAt.IsZero() {
		t.Errorf("date absente : %v", got[2].CreatedAt)
	}
}

func TestListDeploymentsPaginationAndSince(t *testing.T) {
	// 250 deploiements, du plus recent (jour 250) au plus ancien (jour 1).
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var calls []string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.RawQuery)
		var skip, take int
		_, _ = fmt.Sscan(r.URL.Query().Get("skip"), &skip)
		_, _ = fmt.Sscan(r.URL.Query().Get("take"), &take)
		var items []string
		for i := skip; i < skip+take && i < 250; i++ {
			day := 250 - i
			items = append(items, fmt.Sprintf(`{"deployment_uuid":"d%d","status":"finished","created_at":"%s"}`, day, base.AddDate(0, 0, day).Format(time.RFC3339)))
		}
		reply(200, "["+strings.Join(items, ",")+"]")(w, r)
	})

	all, err := c.ListDeployments(context.Background(), "a1", time.Time{})
	if err != nil || len(all) != 250 || len(calls) != 3 {
		t.Fatalf("sans borne : %d deploiements, %d appels, err = %v", len(all), len(calls), err)
	}

	calls = nil
	since := base.AddDate(0, 0, 200)
	recent, err := c.ListDeployments(context.Background(), "a1", since)
	if err != nil {
		t.Fatal(err)
	}
	// Jours 200 a 250 : 51 deploiements ; la page pleine fait lire la suivante,
	// qui n'a plus que des anciens et arrete la lecture.
	if len(recent) != 51 || len(calls) != 2 {
		t.Fatalf("avec borne : %d deploiements, %d appels", len(recent), len(calls))
	}

	// Une borne apres la premiere page entiere : on s'arrete sans lire la suite.
	calls = nil
	none, err := c.ListDeployments(context.Background(), "a1", base.AddDate(0, 0, 400))
	if err != nil || len(none) != 0 || len(calls) != 1 {
		t.Fatalf("borne future : %d deploiements, %d appels, err = %v", len(none), len(calls), err)
	}
}

func TestListDeploymentsEscapesUUID(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v1/deployments/applications/a%2F..%2Fb" {
			t.Errorf("chemin = %s", r.URL.EscapedPath())
		}
		reply(200, `[]`)(w, r)
	})
	if _, err := c.ListDeployments(context.Background(), "a/../b", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListDeployments(context.Background(), " ", time.Time{}); err == nil {
		t.Fatal("un uuid vide a ete accepte")
	}
}

func TestMissingBaseURL(t *testing.T) {
	if _, err := (&Client{Token: token}).ListApplications(context.Background()); err == nil {
		t.Fatal("une adresse vide a ete acceptee")
	}
}

// serveDeployments sert n deploiements, un par jour a partir de base, dans
// l'ordre voulu, avec la pagination skip/take.
func serveDeployments(t *testing.T, n int, newestFirst bool, calls *int) *Client {
	t.Helper()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return serve(t, func(w http.ResponseWriter, r *http.Request) {
		*calls++
		var skip, take int
		_, _ = fmt.Sscan(r.URL.Query().Get("skip"), &skip)
		_, _ = fmt.Sscan(r.URL.Query().Get("take"), &take)
		var items []string
		for i := skip; i < skip+take && i < n; i++ {
			day := i + 1
			if newestFirst {
				day = n - i
			}
			items = append(items, fmt.Sprintf(`{"deployment_uuid":"d%d","status":"finished","created_at":"%s"}`, day, base.AddDate(0, 0, day).Format(time.RFC3339)))
		}
		reply(200, "["+strings.Join(items, ",")+"]")(w, r)
	})
}

func TestListDeploymentsOldestFirstReadsEverything(t *testing.T) {
	calls := 0
	c := serveDeployments(t, 250, false, &calls)
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 200)
	got, err := c.ListDeployments(context.Background(), "a1", since)
	if err != nil {
		t.Fatal(err)
	}
	// Jours 200 a 250, tous sur la derniere page : aucune lecture anticipee.
	if len(got) != 51 || calls != 3 {
		t.Fatalf("%d deploiements, %d appels", len(got), calls)
	}
}

func TestListDeploymentsNewestFirstStopsEarly(t *testing.T) {
	calls := 0
	c := serveDeployments(t, 1500, true, &calls)
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1450)
	got, err := c.ListDeployments(context.Background(), "a1", since)
	if err != nil || len(got) != 51 || calls != 2 {
		t.Fatalf("%d deploiements, %d appels, err = %v", len(got), calls, err)
	}
}

func TestListDeploymentsCapFailsInsteadOfTruncating(t *testing.T) {
	for _, newestFirst := range []bool{true, false} {
		calls := 0
		c := serveDeployments(t, maxPages*pageSize+50, newestFirst, &calls)
		// since ancien : tout est recent, la lecture ne peut pas s'arreter.
		_, err := c.ListDeployments(context.Background(), "a1", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
		if err == nil || calls != maxPages {
			t.Fatalf("newestFirst=%v : err = %v, %d appels", newestFirst, err, calls)
		}
	}
}

func TestIsNewestFirst(t *testing.T) {
	d := func(day int) Deployment { return Deployment{CreatedAt: time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC)} }
	for name, c := range map[string]struct {
		in   []Deployment
		want bool
	}{
		"decroissant":  {[]Deployment{d(3), d(2), d(1)}, true},
		"croissant":    {[]Deployment{d(1), d(2), d(3)}, false},
		"desordre":     {[]Deployment{d(3), d(1), d(2)}, false},
		"un seul":      {[]Deployment{d(1)}, false},
		"sans dates":   {[]Deployment{{}, {}}, false},
		"memes dates":  {[]Deployment{d(1), d(1)}, false},
		"avec un trou": {[]Deployment{d(3), {}, d(1)}, true},
	} {
		if got := isNewestFirst(c.in); got != c.want {
			t.Errorf("%s : %v", name, got)
		}
	}
}
