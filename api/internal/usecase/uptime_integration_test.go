//go:build integration

package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// fakeKuma sert un /metrics dont on regle l'etat des sites, et peut tomber.
type fakeKuma struct {
	mu     sync.Mutex
	status map[string]int // nom -> etat
	broken bool
}

func (k *fakeKuma) set(name string, status int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.status[name] = status
}

func (k *fakeKuma) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, pass, _ := r.BasicAuth(); pass != "uk2_test" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if k.broken {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	ids := map[string]int{"Vitrine": 1, "Preprod": 2}
	for name, status := range k.status {
		labels := fmt.Sprintf(`monitor_id="%d",monitor_name="%s",monitor_type="http",monitor_url="https://%d.example",monitor_hostname="null",monitor_port="null"`, ids[name], name, ids[name])
		_, _ = fmt.Fprintf(w, "monitor_status{%s} %d\nmonitor_response_time{%s} 120\n", labels, status, labels)
	}
}

// newUptime remet le module a zero et le branche sur un faux Kuma.
func newUptime(t *testing.T, pool *pgxpool.Pool) (*usecase.UptimeService, *fakeKuma) {
	t.Helper()
	ctx := context.Background()
	reset := func() {
		for _, q := range []string{
			`DELETE FROM uptime_monitors`,
			`UPDATE uptime_settings SET enabled = false, base_url = '', api_key_enc = NULL, poll_minutes = 2, webhook_secret = '',
			        webhook_last_at = NULL, last_poll_at = NULL, last_poll_error = NULL, poll_requested_at = NULL`,
		} {
			if _, err := pool.Exec(ctx, q); err != nil {
				t.Fatalf("remise a zero : %v", err)
			}
		}
	}
	reset()
	t.Cleanup(reset)

	kuma := &fakeKuma{status: map[string]int{"Vitrine": 1, "Preprod": 1}}
	srv := httptest.NewServer(kuma)
	t.Cleanup(srv.Close)

	svc := usecase.NewUptimeService(pool, config.UptimeEnv{}, []byte(inboundSecret), "https://piilot.test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	key := "uk2_test"
	if _, err := svc.UpdateSettings(ctx, usecase.UptimeSettingsInput{Enabled: true, BaseURL: srv.URL + "/metrics", APIKey: &key, PollMinutes: 1}); err != nil {
		t.Fatalf("reglages : %v", err)
	}
	return svc, kuma
}

func monitorsByName(t *testing.T, svc *usecase.UptimeService) map[string]usecase.UptimeMonitorView {
	t.Helper()
	page, err := svc.Monitors(context.Background(), "", 1, 25)
	if err != nil {
		t.Fatalf("sites : %v", err)
	}
	out := map[string]usecase.UptimeMonitorView{}
	for _, m := range page.Items {
		out[m.Name] = m
	}
	return out
}

func TestLaReleveTientLHistoriqueEtLesIncidents(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, kuma := newUptime(t, pool)

	// Une releve demandee par l'enregistrement : les deux sites arrivent.
	if !svc.PollDue(ctx) {
		t.Fatal("l'enregistrement n'a pas demande de releve")
	}
	if err := svc.Poll(ctx); err != nil {
		t.Fatalf("releve : %v", err)
	}
	sites := monitorsByName(t, svc)
	if len(sites) != 2 || sites["Vitrine"].Status != "up" || sites["Vitrine"].Uptime30d == nil || *sites["Vitrine"].Uptime30d != 100 {
		t.Fatalf("apres la premiere releve : %+v", sites)
	}
	settings, err := svc.Settings(ctx)
	if err != nil || settings.Status.State != "ok" || settings.Status.Monitors != 2 || !settings.APIKeySet || settings.BaseURL == "" {
		t.Fatalf("etat du module : %+v, %v", settings.Status, err)
	}

	// La vitrine tombe : la releve ouvre un incident, faute de webhook.
	kuma.set("Vitrine", 0)
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	incidents, err := svc.Incidents(ctx, 1, 10)
	if err != nil || incidents.Total != 1 || incidents.Items[0].EndedAt != nil || incidents.Items[0].Source != "poll" {
		t.Fatalf("incident deduit : %+v, %v", incidents, err)
	}

	// Kuma tombe a son tour : la releve ratee ne compte ni pour ni contre.
	kuma.mu.Lock()
	kuma.broken = true
	kuma.mu.Unlock()
	if err := svc.Poll(ctx); err == nil {
		t.Fatal("une releve ratee n'a pas ete signalee")
	}
	var total, up, down, missing int
	if err := pool.QueryRow(ctx, `SELECT d.checks_total, d.checks_up, d.checks_down, d.checks_missing FROM uptime_daily d
		JOIN uptime_monitors m ON m.id = d.monitor_id WHERE m.name = 'Vitrine'`).Scan(&total, &up, &down, &missing); err != nil {
		t.Fatal(err)
	}
	if total != 3 || up != 1 || down != 1 || missing != 1 {
		t.Errorf("agregat du jour : %d controles, %d en ligne, %d en panne, %d manquants", total, up, down, missing)
	}
	if got := monitorsByName(t, svc)["Vitrine"].Uptime30d; got == nil || *got != 50 {
		t.Errorf("disponibilite : %v, attendu 50 %% (le controle manquant n'entre pas)", got)
	}
	settings, _ = svc.Settings(ctx)
	if settings.Status.State != "error" || settings.Status.LastPollError == nil {
		t.Errorf("l'erreur de releve ne se voit pas : %+v", settings.Status)
	}
	kuma.mu.Lock()
	kuma.broken = false
	kuma.mu.Unlock()

	// Le webhook precise l'incident ouvert par la releve : heure exacte et
	// raison. Un mauvais jeton est refuse.
	if _, err := svc.Webhook(ctx, "kuma", "mauvais", []byte(`{}`)); !errors.Is(err, usecase.ErrUptimeForbidden) {
		t.Errorf("mauvais jeton : %v", err)
	}
	token := webhookToken(t, settings.WebhookURL)
	earlier := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Millisecond)
	downEvent := fmt.Sprintf(`{"heartbeat":{"monitorID":1,"status":0,"time":%q,"msg":"timeout"},"monitor":{"id":1,"name":"Vitrine"}}`, earlier.Format("2006-01-02 15:04:05.000"))
	if res, err := svc.Webhook(ctx, "kuma", token, []byte(downEvent)); err != nil || res.Outcome != "recorded" {
		t.Fatalf("panne poussee : %+v, %v", res, err)
	}
	incidents, _ = svc.Incidents(ctx, 1, 10)
	if incidents.Total != 1 || incidents.Items[0].Message != "timeout" || !incidents.Items[0].StartedAt.Equal(earlier) || incidents.Items[0].Source != "webhook" {
		t.Fatalf("incident precise : %+v", incidents.Items)
	}

	// Le retour pousse ferme l'incident, avec sa duree.
	up1 := `{"heartbeat":{"monitorID":1,"status":1,"time":"` + time.Now().UTC().Format("2006-01-02 15:04:05.000") + `","msg":"200 - OK"},"monitor":{"id":1,"name":"Vitrine"}}`
	if _, err := svc.Webhook(ctx, "kuma", token, []byte(up1)); err != nil {
		t.Fatal(err)
	}
	incidents, _ = svc.Incidents(ctx, 1, 10)
	if incidents.Items[0].EndedAt == nil || incidents.Items[0].DurationSeconds == nil || *incidents.Items[0].DurationSeconds < 590 {
		t.Errorf("incident clos : %+v", incidents.Items[0])
	}
	// L'essai d'une notification depuis Kuma est accepte et ignore.
	if res, err := svc.Webhook(ctx, "kuma", token, []byte(`{"heartbeat":null,"monitor":null,"msg":"Uptime Kuma Testing"}`)); err != nil || res.Outcome != "ignored" {
		t.Errorf("essai : %+v, %v", res, err)
	}

	// Un site retire de Kuma reste, marque absent.
	kuma.mu.Lock()
	delete(kuma.status, "Preprod")
	kuma.mu.Unlock()
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if m := monitorsByName(t, svc)["Preprod"]; m.Present {
		t.Errorf("site disparu toujours present : %+v", m)
	}
}

func TestUnProjetSeLieASesSites(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, _ := newUptime(t, pool)
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sites := monitorsByName(t, svc)
	projectID := newProject(t, pool, 10)

	view, err := svc.ProjectUptime(ctx, projectID)
	if err != nil || !view.ModuleEnabled || len(view.Sites) != 0 {
		t.Fatalf("projet sans site : %+v, %v", view, err)
	}

	view, err = svc.SetProjectSites(ctx, projectID, []usecase.ProjectSiteInput{
		{MonitorID: sites["Preprod"].ID, Environment: "preprod"},
		{MonitorID: sites["Vitrine"].ID, Environment: "prod"},
	})
	if err != nil || len(view.Sites) != 2 || view.Sites[0].Environment != "prod" || len(view.Sites[0].Days) != 30 {
		t.Fatalf("liens : %+v, %v", view, err)
	}
	if last := view.Sites[0].Days[29]; last.Uptime == nil || *last.Uptime != 100 {
		t.Errorf("frise du jour : %+v", last)
	}
	if got := monitorsByName(t, svc)["Vitrine"].Projects; len(got) != 1 {
		t.Errorf("projets du site : %v", got)
	}

	for name, sites := range map[string][]usecase.ProjectSiteInput{
		"environnement": {{MonitorID: sites["Vitrine"].ID, Environment: "staging"}},
		"doublon":       {{MonitorID: sites["Vitrine"].ID, Environment: "prod"}, {MonitorID: sites["Vitrine"].ID, Environment: "preprod"}},
		"inconnu":       {{MonitorID: uuid.New(), Environment: "prod"}},
	} {
		if _, err := svc.SetProjectSites(ctx, projectID, sites); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s accepte : %v", name, err)
		}
	}

	// Une liste vide detache tout : le lien est facultatif.
	if view, err := svc.SetProjectSites(ctx, projectID, nil); err != nil || len(view.Sites) != 0 {
		t.Errorf("detacher : %+v, %v", view, err)
	}
	if _, err := svc.ProjectUptime(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("projet inconnu : %v", err)
	}
}

// webhookToken lit le jeton de l'adresse du webhook.
func webhookToken(t *testing.T, webhookURL string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, webhookURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	token := req.URL.Query().Get("token")
	if token == "" {
		t.Fatalf("adresse sans jeton : %s", webhookURL)
	}
	return token
}

func TestLeClientVoitLaDisponibiliteDeSesSites(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, _ := newUptime(t, pool)
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	sites := monitorsByName(t, svc)
	client := newPortalClient(t, pool)
	if _, err := svc.SetProjectSites(ctx, client.projectID, []usecase.ProjectSiteInput{
		{MonitorID: sites["Vitrine"].ID, Environment: "prod"},
		{MonitorID: sites["Preprod"].ID, Environment: "preprod"},
	}); err != nil {
		t.Fatal(err)
	}

	// La production seule, avec ses trente jours.
	got, err := svc.PortalUptime(ctx, client.userID)
	if err != nil || len(got) != 1 || got[0].Name != "Vitrine" || got[0].ProjectID != client.projectID || got[0].Status != "up" || got[0].Uptime == nil || *got[0].Uptime != 100 {
		t.Fatalf("disponibilite du client : %+v, %v", got, err)
	}

	// Un autre client ne voit rien, ni personne quand le module est coupe.
	if other, err := svc.PortalUptime(ctx, newPortalClient(t, pool).userID); err != nil || len(other) != 0 {
		t.Errorf("autre client : %+v, %v", other, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE uptime_settings SET enabled = false`); err != nil {
		t.Fatal(err)
	}
	if off, err := svc.PortalUptime(ctx, client.userID); err != nil || len(off) != 0 {
		t.Errorf("module coupe : %+v, %v", off, err)
	}
}
