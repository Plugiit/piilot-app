//go:build integration

package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// fakeAnalytics joue une instance Umami 3.x : deux sites, des chiffres fixes,
// trois visiteurs en ce moment.
type fakeAnalytics struct {
	mu    sync.Mutex
	calls map[string]int
}

func (f *fakeAnalytics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/api")
	f.calls[path[strings.LastIndex(path, "/"):]]++

	if path == "/auth/login" {
		var b struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.Password != "umami" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"token":"jeton"}`))
		return
	}
	if r.Header.Get("Authorization") != "Bearer jeton" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case path == "/websites":
		_, _ = w.Write([]byte(`{"data":[{"id":"w1","name":"Vitrine","domain":"atelier.example"},{"id":"w2","name":"Préprod","domain":"preprod.example"}],"count":2,"page":1,"pageSize":100}`))
	case strings.HasSuffix(path, "/stats"):
		_, _ = w.Write([]byte(`{"pageviews":120,"visitors":40,"visits":55,"bounces":20,"totaltime":3600,"comparison":{"pageviews":100,"visitors":50,"visits":60,"bounces":30,"totaltime":3000}}`))
	case strings.HasSuffix(path, "/pageviews"):
		_, _ = w.Write([]byte(`{"pageviews":[{"x":"2026-10-12 00:00:00","y":90}],"sessions":[{"x":"2026-10-12 00:00:00","y":25}]}`))
	case strings.HasSuffix(path, "/metrics"):
		_, _ = w.Write([]byte(`[{"x":"/","y":80},{"x":"/contact","y":12}]`))
	case strings.HasSuffix(path, "/active"):
		_, _ = w.Write([]byte(`{"visitors":3}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// newAnalytics remet le module a zero et le branche sur un faux Umami.
func newAnalytics(t *testing.T, pool *pgxpool.Pool) (*usecase.AnalyticsService, *fakeAnalytics, string) {
	t.Helper()
	ctx := context.Background()
	reset := func() {
		for _, q := range []string{
			`DELETE FROM analytics_websites`,
			`UPDATE analytics_settings SET enabled = false, base_url = '', auth_mode = 'login', username = '', password_enc = NULL,
			        api_key_enc = NULL, token_enc = NULL, api_version = 0, version_pinned = false, cache_minutes = 10,
			        last_check_at = NULL, last_check_error = NULL, last_check_note = NULL, check_requested_at = NULL`,
		} {
			if _, err := pool.Exec(ctx, q); err != nil {
				t.Fatalf("remise a zero : %v", err)
			}
		}
	}
	reset()
	t.Cleanup(reset)

	fake := &fakeAnalytics{calls: map[string]int{}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	svc := usecase.NewAnalyticsService(pool, config.AnalyticsEnv{}, []byte(inboundSecret), slog.New(slog.NewTextHandler(io.Discard, nil)))
	password := "umami"
	if _, err := svc.UpdateSettings(ctx, usecase.AnalyticsSettingsInput{
		Enabled: true, BaseURL: srv.URL, AuthMode: "login", Username: "admin", Password: &password, CacheMinutes: 10,
	}); err != nil {
		t.Fatalf("reglages : %v", err)
	}
	return svc, fake, srv.URL
}

func websiteIDs(t *testing.T, svc *usecase.AnalyticsService) map[string]usecase.AnalyticsWebsiteView {
	t.Helper()
	page, err := svc.Websites(context.Background(), "", 1, 25)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]usecase.AnalyticsWebsiteView{}
	for _, w := range page.Items {
		out[w.Name] = w
	}
	return out
}

func TestLesStatistiquesPassentParLeCache(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, fake, _ := newAnalytics(t, pool)

	// La verification demandee par l'enregistrement : les sites arrivent, la
	// version se lit.
	if err := svc.Tick(ctx); err != nil {
		t.Fatalf("passage : %v", err)
	}
	settings, err := svc.Settings(ctx)
	if err != nil || settings.Status.State != "ok" || settings.Status.Websites != 2 || settings.APIVersion != 3 || settings.Status.LastCheckNote != nil {
		t.Fatalf("apres verification : %+v, %v", settings, err)
	}
	if !settings.PasswordSet || settings.Username != "admin" {
		t.Errorf("secrets mal rendus : %+v", settings)
	}

	sites := websiteIDs(t, svc)
	client := newPortalClient(t, pool)
	if _, err := svc.SetProjectWebsites(ctx, client.projectID, []usecase.ProjectWebsiteInput{
		{WebsiteID: sites["Vitrine"].ID, Environment: "prod"}, {WebsiteID: sites["Préprod"].ID, Environment: "prod"},
	}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("deux productions acceptees : %v", err)
	}
	if view, err := svc.SetProjectWebsites(ctx, client.projectID, []usecase.ProjectWebsiteInput{
		{WebsiteID: sites["Préprod"].ID, Environment: "preprod"}, {WebsiteID: sites["Vitrine"].ID, Environment: "prod"},
	}); err != nil || len(view.Sites) != 2 || view.Sites[0].Environment != "prod" {
		t.Fatalf("liens : %+v, %v", view, err)
	}

	// Le passage suivant prepare la vue par defaut du site de production.
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	view, err := svc.PortalStatistics(ctx, client.userID, client.projectID, "30d")
	if err != nil || view.Status != "ready" || view.Data == nil {
		t.Fatalf("trente jours : %+v, %v", view, err)
	}
	if view.Data.Totals.Pageviews != 120 || view.Data.Previous.Visitors != 50 || len(view.Data.Series) != 1 || len(view.Data.Pages) != 2 || view.Website.Name != "Vitrine" {
		t.Errorf("vue : %+v", view.Data)
	}

	// Une periode jamais lue : une demande, servie au passage suivant. Le
	// service n'est pas appele pendant la lecture.
	before := fake.calls["/stats"]
	if view, err := svc.PortalStatistics(ctx, client.userID, client.projectID, "7d"); err != nil || view.Status != "pending" {
		t.Fatalf("sept jours avant le passage : %+v, %v", view, err)
	}
	if fake.calls["/stats"] != before {
		t.Error("la lecture a appele Umami")
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if view, err := svc.PortalStatistics(ctx, client.userID, client.projectID, "7d"); err != nil || view.Status != "ready" {
		t.Fatalf("sept jours apres le passage : %+v, %v", view, err)
	}

	// Les visiteurs du moment : demandes par l'ecran, releves par la tache.
	if active, err := svc.PortalActive(ctx, client.userID, client.projectID); err != nil || active.Visitors != nil {
		t.Fatalf("visiteurs avant releve : %+v, %v", active, err)
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if active, err := svc.PortalActive(ctx, client.userID, client.projectID); err != nil || active.Visitors == nil || *active.Visitors != 3 {
		t.Errorf("visiteurs apres releve : %+v, %v", active, err)
	}

	// La photo du mois ecoule existe, figee.
	var snapshots int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM analytics_snapshots WHERE website_id = $1`, sites["Vitrine"].ID).Scan(&snapshots); err != nil || snapshots != 1 {
		t.Errorf("photo du mois : %d, %v", snapshots, err)
	}

	// Periodes personnalisees : bornees, et limitees par compte.
	if _, err := svc.PortalStatistics(ctx, client.userID, client.projectID, "custom:2020-01-01:2026-10-01"); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("plus de 24 mois : %v", err)
	}
	if view, err := svc.PortalStatistics(ctx, client.userID, client.projectID, "custom:2026-01-01:2026-03-31"); err != nil || view.Status != "pending" {
		t.Errorf("periode personnalisee : %+v, %v", view, err)
	}
	// La meme demande, encore en attente, ne se duplique pas.
	if _, err := svc.PortalStatistics(ctx, client.userID, client.projectID, "custom:2026-01-01:2026-03-31"); err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM analytics_requests WHERE period_key = 'custom:2026-01-01:2026-03-31' AND served_at IS NULL`).Scan(&pending); err != nil || pending != 1 {
		t.Errorf("demandes en attente : %d, %v", pending, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO analytics_requests (website_id, period_key, requested_by, served_at)
		SELECT $1, 'custom:2026-02-0' || g || ':2026-02-09', $2, now() FROM generate_series(1, 20) g`, sites["Vitrine"].ID, client.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PortalStatistics(ctx, client.userID, client.projectID, "custom:2026-04-01:2026-04-30"); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("limite par compte : %v", err)
	}

	// Un autre client ne voit rien : ni chiffres, ni visiteurs.
	other := newPortalClient(t, pool)
	if _, err := svc.PortalStatistics(ctx, other.userID, client.projectID, "30d"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("autre client : %v", err)
	}
	if _, err := svc.PortalActive(ctx, other.userID, client.projectID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("autre client, visiteurs : %v", err)
	}

	// Module coupe : la page disparait.
	if _, err := pool.Exec(ctx, `UPDATE analytics_settings SET enabled = false`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PortalStatistics(ctx, client.userID, client.projectID, "30d"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("module coupe : %v", err)
	}
}

func TestDeMauvaisIdentifiantsSeVoientALEcran(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, _, url := newAnalytics(t, pool)

	wrong := "faux"
	if _, err := svc.UpdateSettings(ctx, usecase.AnalyticsSettingsInput{
		Enabled: true, BaseURL: url, AuthMode: "login", Username: "admin", Password: &wrong, CacheMinutes: 10,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	settings, _ := svc.Settings(ctx)
	if settings.Status.State != "error" || settings.Status.LastCheckError == nil || !strings.Contains(*settings.Status.LastCheckError, "identifiants") {
		t.Errorf("erreur non signalee : %+v", settings.Status)
	}
	// Une version choisie a la main tient, quoi que dise la detection.
	if _, err := svc.UpdateSettings(ctx, usecase.AnalyticsSettingsInput{
		Enabled: true, BaseURL: url, AuthMode: "login", Username: "admin", APIVersion: 2, CacheMinutes: 10,
	}); err != nil {
		t.Fatal(err)
	}
	if settings, _ := svc.Settings(ctx); settings.APIVersion != 2 || !settings.VersionPinned {
		t.Errorf("version choisie : %+v", settings)
	}
}
