//go:build integration

package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/coolify"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// fakeCoolify remplace l'API de Coolify : ses applications se reglent, et il
// retient l'adresse et le jeton avec lesquels on l'a appele.
type fakeCoolify struct {
	mu      sync.Mutex
	apps    []coolify.Application
	err     error
	deploys map[string][]coolify.Deployment
	depErr  error
	baseURL string
	token   string
	calls   int
}

func (f *fakeCoolify) ListApplications(context.Context) ([]coolify.Application, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.apps, f.err
}

func (f *fakeCoolify) ListDeployments(_ context.Context, appUUID string, _ time.Time) ([]coolify.Deployment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deploys[appUUID], f.depErr
}

func (f *fakeCoolify) set(apps []coolify.Application, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.apps, f.err = apps, err
}

// newCoolify remet le module a zero et le branche sur un faux Coolify.
func newCoolify(t *testing.T, pool *pgxpool.Pool, env config.CoolifyEnv) (*usecase.CoolifyService, *fakeCoolify) {
	t.Helper()
	ctx := context.Background()
	reset := func() {
		for _, q := range []string{
			`DELETE FROM project_coolify_applications`,
			`DELETE FROM coolify_applications`,
			`UPDATE coolify_settings SET enabled = false, base_url = '', api_token_enc = NULL, poll_minutes = 60,
			        last_poll_at = NULL, last_poll_error = NULL, poll_requested_at = NULL`,
		} {
			if _, err := pool.Exec(ctx, q); err != nil {
				t.Fatalf("remise a zero : %v", err)
			}
		}
	}
	reset()
	t.Cleanup(reset)

	fake := &fakeCoolify{}
	svc := usecase.NewCoolifyService(pool, env, []byte(inboundSecret), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.SetProviderFactory(func(baseURL, token string) coolify.Provider {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		fake.baseURL, fake.token = baseURL, token
		return fake
	})
	return svc, fake
}

func TestCoolifyLeJetonEstChiffreEtNeSortJamais(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{})

	token := "7|jeton-secret-de-test"
	view, err := svc.UpdateSettings(ctx, usecase.CoolifySettingsInput{Enabled: true, BaseURL: "https://coolify.test/api/v1/", APIToken: &token})
	if err != nil {
		t.Fatalf("reglages : %v", err)
	}
	if !view.APITokenSet || view.BaseURL != "https://coolify.test" || view.PollMinutes != 60 || view.Status.State != "pending" || !view.Status.PollPending {
		t.Fatalf("vue : %+v", view)
	}
	if raw, _ := json.Marshal(view); bytes.Contains(raw, []byte("jeton-secret")) {
		t.Fatalf("le jeton sort dans la vue : %s", raw)
	}

	// En base : chiffre, et rien de lisible.
	var enc []byte
	if err := pool.QueryRow(ctx, `SELECT api_token_enc FROM coolify_settings WHERE id = 1`).Scan(&enc); err != nil {
		t.Fatal(err)
	}
	if len(enc) == 0 || bytes.Contains(enc, []byte("jeton-secret")) {
		t.Fatalf("jeton non chiffre en base : %q", enc)
	}
	var clear int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM coolify_settings WHERE row(coolify_settings.*)::text LIKE '%jeton-secret%'`).Scan(&clear); err != nil || clear != 0 {
		t.Fatalf("jeton en clair dans la ligne : %d, %v", clear, err)
	}

	// Un enregistrement sans jeton garde celui de la base, et la releve s'en sert.
	if _, err := svc.UpdateSettings(ctx, usecase.CoolifySettingsInput{Enabled: true, BaseURL: "https://coolify.test", PollMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Poll(ctx); err != nil {
		t.Fatalf("releve : %v", err)
	}
	if fake.token != token || fake.baseURL != "https://coolify.test" {
		t.Fatalf("le client a recu %q, %q", fake.baseURL, fake.token)
	}

	// Une chaine vide l'efface.
	empty := ""
	view, err = svc.UpdateSettings(ctx, usecase.CoolifySettingsInput{Enabled: true, BaseURL: "https://coolify.test", APIToken: &empty})
	if err != nil || view.APITokenSet {
		t.Fatalf("effacement : %+v, %v", view, err)
	}
}

func TestCoolifyLaRelevePeupleLesApplications(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{})
	fake.set([]coolify.Application{
		{UUID: "a1", Name: "Vitrine", FQDN: "https://vitrine.test", EnvironmentName: "production", ProjectName: "Client A"},
		{UUID: "a2", Name: "API", EnvironmentName: "staging", ProjectName: "Client A"},
	}, nil)

	token := "1|abc"
	if _, err := svc.UpdateSettings(ctx, usecase.CoolifySettingsInput{Enabled: true, BaseURL: "https://coolify.test", APIToken: &token}); err != nil {
		t.Fatal(err)
	}
	if !svc.PollDue(ctx) {
		t.Fatal("l'enregistrement n'a pas demande de releve")
	}
	if err := svc.Poll(ctx); err != nil {
		t.Fatalf("releve : %v", err)
	}
	apps, err := svc.Applications(ctx)
	if err != nil || len(apps) != 2 || !apps[0].Present || apps[0].Name != "API" && apps[0].Name != "Vitrine" {
		t.Fatalf("applications : %+v, %v", apps, err)
	}
	view, _ := svc.Settings(ctx)
	if view.Status.State != "ok" || view.Status.Applications != 2 || view.Status.PollPending || view.Status.LastPollAt == nil {
		t.Fatalf("etat : %+v", view.Status)
	}
	// Releve faite et intervalle non ecoule : rien a refaire.
	if svc.PollDue(ctx) {
		t.Fatal("une releve est demandee alors que l'intervalle n'est pas ecoule")
	}

	// Vitrine disparait, Api change de nom : l'absente est marquee, pas effacee.
	fake.set([]coolify.Application{{UUID: "a2", Name: "API v2", ProjectName: "Client A"}}, nil)
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	apps, _ = svc.Applications(ctx)
	byUUID := map[string]usecase.CoolifyApplicationView{}
	for _, a := range apps {
		byUUID[a.UUID] = a
	}
	if len(apps) != 2 || byUUID["a1"].Present || !byUUID["a2"].Present || byUUID["a2"].Name != "API v2" {
		t.Fatalf("apres la deuxieme releve : %+v", apps)
	}
	if view, _ = svc.Settings(ctx); view.Status.Applications != 1 {
		t.Fatalf("applications presentes : %d", view.Status.Applications)
	}

	// Une releve ratee garde les applications et affiche l'erreur.
	fake.set(nil, coolify.ErrUnauthorized)
	if err := svc.Poll(ctx); !errors.Is(err, coolify.ErrUnauthorized) {
		t.Fatalf("erreur = %v", err)
	}
	view, _ = svc.Settings(ctx)
	if view.Status.State != "error" || view.Status.LastPollError == nil || view.Status.Applications != 1 {
		t.Fatalf("apres l'echec : %+v", view.Status)
	}
	modules, err := svc.Modules(ctx)
	if err != nil || len(modules) != 1 || modules[0].Code != "deployments" || modules[0].State != "error" || !modules[0].Enabled {
		t.Fatalf("carte : %+v, %v", modules, err)
	}
}

func TestCoolifyLEnvironnementPrime(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{URL: "https://env.test/", APIToken: "9|env"})

	other := "1|base"
	view, err := svc.UpdateSettings(ctx, usecase.CoolifySettingsInput{Enabled: true, BaseURL: "https://base.test", APIToken: &other})
	if err != nil {
		t.Fatal(err)
	}
	if !view.BaseURLFromEnv || !view.APITokenFromEnv || view.BaseURL != "https://env.test" || !view.APITokenSet {
		t.Fatalf("vue : %+v", view)
	}
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if fake.baseURL != "https://env.test" || fake.token != "9|env" {
		t.Fatalf("le client a recu %q, %q", fake.baseURL, fake.token)
	}
	var enc []byte
	if err := pool.QueryRow(ctx, `SELECT api_token_enc FROM coolify_settings WHERE id = 1`).Scan(&enc); err != nil || len(enc) != 0 {
		t.Fatalf("un jeton a ete range en base malgre l'environnement : %q, %v", enc, err)
	}
}

func TestCoolifyValidation(t *testing.T) {
	_, pool := newService(t)
	svc, _ := newCoolify(t, pool, config.CoolifyEnv{})
	ctx := context.Background()

	for name, in := range map[string]usecase.CoolifySettingsInput{
		"adresse manquante":     {Enabled: true},
		"adresse invalide":      {BaseURL: "ftp://x"},
		"intervalle trop court": {BaseURL: "https://c.test", PollMinutes: 5},
		"intervalle trop long":  {BaseURL: "https://c.test", PollMinutes: 1441},
	} {
		if _, err := svc.UpdateSettings(ctx, in); err == nil {
			t.Errorf("%s : accepte", name)
		}
	}

	// Sans adresse ni jeton, la releve dit ce qui manque.
	if err := svc.Poll(ctx); err == nil {
		t.Fatal("releve sans adresse")
	}
	if view, _ := svc.Settings(ctx); view.Status.LastPollError == nil {
		t.Fatal("l'erreur n'est pas enregistree")
	}
}
