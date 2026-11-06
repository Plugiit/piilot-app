package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// testBotKey a la forme d'une cle du bot ; stubBotKeys ne reconnait qu'elle.
var testBotKey = security.BotKeyPrefix + strings.Repeat("A", 43)

// stubBotKeys remplace la table des cles.
type stubBotKeys struct {
	err error
}

func (s stubBotKeys) AuthenticateBotKey(_ context.Context, raw string) (string, uuid.UUID, bool, error) {
	if s.err != nil {
		return "", uuid.Nil, false, s.err
	}
	if raw != testBotKey {
		return "", uuid.Nil, false, nil
	}
	return "bot", uuid.New(), true, nil
}

// stubDiscord remplace le service : il note ce qui l'a atteint.
type stubDiscord struct {
	redeemed bool
	listed   bool
}

func (s *stubDiscord) Status(context.Context, uuid.UUID) (usecase.DiscordStatus, error) {
	return usecase.DiscordStatus{}, nil
}

func (s *stubDiscord) CreateCode(context.Context, uuid.UUID) (usecase.DiscordCode, error) {
	return usecase.DiscordCode{Code: "K7QM-4XZD"}, nil
}

func (s *stubDiscord) Unlink(context.Context, uuid.UUID) error { return nil }

func (s *stubDiscord) RateKey(string) string { return "empreinte" }

func (s *stubDiscord) Redeem(_ context.Context, _, discordUserID string) (usecase.DiscordLinkCreated, error) {
	s.redeemed = true
	return usecase.DiscordLinkCreated{DiscordLink: usecase.DiscordLink{DiscordUserID: discordUserID, Role: "client"}}, nil
}

func (s *stubDiscord) Link(context.Context, string) (usecase.DiscordLink, error) {
	return usecase.DiscordLink{}, usecase.ErrDiscordLinkNotFound
}

func (s *stubDiscord) Links(context.Context, string, int) (usecase.DiscordLinkPage, error) {
	s.listed = true
	return usecase.DiscordLinkPage{Items: []usecase.DiscordLink{}}, nil
}

type stubBotEvents struct{}

func (stubBotEvents) Events(context.Context, *int64, int) (usecase.BotEventPage, error) {
	return usecase.BotEventPage{Items: []usecase.BotEvent{}, NextAfter: "0"}, nil
}

// errLimiter simule un Redis injoignable.
type errLimiter struct{}

func (errLimiter) Allow(context.Context, string, int, time.Duration) (bool, time.Duration, error) {
	return false, 0, errors.New("redis injoignable")
}

func (errLimiter) Reset(context.Context, string) error { return nil }

// botApp monte le routeur complet avec les routes du bot.
func botApp(t *testing.T, keys middleware.BotKeyChecker, perms stubPermissions, limiter RateLimiter) (*fiber.App, *stubDiscord, *security.TokenSigner) {
	t.Helper()

	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(discardLogger())})
	signer := security.NewTokenSigner([]byte(testSecret), "piilot-api")
	guard := middleware.NewGuard(signer, perms)
	if keys != nil {
		guard.SetBotKeys(keys)
	}
	discord := &stubDiscord{}

	Register(app, Deps{
		Health:    NewHealth(nil, nil, BuildInfo{Version: "test"}),
		Auth:      NewAuth(&stubAuth{}, CookieConfig{}, &stubLimiter{}, discardLogger()),
		Discord:   NewDiscord(discord, limiter),
		BotEvents: NewBotEvents(stubBotEvents{}),
		Guard:     guard,
	})
	return app, discord, signer
}

func send(t *testing.T, app *fiber.App, req *http.Request) (int, string) {
	t.Helper()
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s : %v", req.Method, req.URL.Path, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body)
}

func botRequest(method, path, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+testBotKey)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// Garde-fou : toute route sous /api/v1/bot exige une cle du bot. Ce test casse
// si une route y est montee hors du groupe garde.
func TestRoutesBotExigentUneCle(t *testing.T) {
	app, _, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{})

	seen := 0
	for _, route := range app.GetRoutes(true) {
		if route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		if !strings.HasPrefix(route.Path, "/api/v1/bot/") {
			continue
		}
		seen++
		status, _ := send(t, app, httptest.NewRequest(route.Method, route.Path, nil))
		if status != http.StatusUnauthorized {
			t.Errorf("%s %s sans cle : statut = %d, attendu 401", route.Method, route.Path, status)
		}
	}
	if seen == 0 {
		t.Fatal("aucune route sous /api/v1/bot")
	}
}

func TestUneCleValideOuvreLesRoutesDuBot(t *testing.T) {
	app, discord, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{})

	status, body := send(t, app, botRequest(http.MethodGet, "/api/v1/bot/discord-links", ""))
	if status != http.StatusOK || !discord.listed {
		t.Fatalf("statut = %d (%s), liste atteinte = %v", status, body, discord.listed)
	}
	if status, _ := send(t, app, botRequest(http.MethodGet, "/api/v1/bot/events?after=12&limit=50", "")); status != http.StatusOK {
		t.Errorf("flux : statut = %d", status)
	}
}

func TestUneCleRevoqueeOuInconnueEstRefusee(t *testing.T) {
	app, discord, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{})

	other, _, err := security.NewBotKey()
	if err != nil {
		t.Fatal(err)
	}
	for name, header := range map[string]string{
		"cle revoquee ou inconnue": "Bearer " + other,
		"sans Bearer":              testBotKey,
		"Bearer vide":              "Bearer ",
		"autre schema":             "Basic " + testBotKey,
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bot/discord-links", nil)
		req.Header.Set("Authorization", header)
		if status, _ := send(t, app, req); status != http.StatusUnauthorized {
			t.Errorf("%s : statut = %d, attendu 401", name, status)
		}
	}
	if discord.listed {
		t.Error("le handler a ete atteint sans cle valide")
	}
}

func TestSansVerificateurDeCleToutAppelDuBotEstRefuse(t *testing.T) {
	app, _, _ := botApp(t, nil, stubPermissions{granted: true}, &stubLimiter{})

	if status, _ := send(t, app, botRequest(http.MethodGet, "/api/v1/bot/discord-links", "")); status != http.StatusUnauthorized {
		t.Fatalf("statut = %d, attendu 401", status)
	}
}

func TestUnePanneDeLectureDesClesNOuvreRien(t *testing.T) {
	app, discord, _ := botApp(t, stubBotKeys{err: errors.New("base injoignable")}, stubPermissions{granted: true}, &stubLimiter{})

	if status, _ := send(t, app, botRequest(http.MethodGet, "/api/v1/bot/discord-links", "")); status != http.StatusInternalServerError {
		t.Fatalf("statut = %d, attendu 500", status)
	}
	if discord.listed {
		t.Error("le handler a ete atteint malgre la panne")
	}
}

func TestUnCookieDeSessionEstRefuseSurLesRoutesDuBot(t *testing.T) {
	app, discord, signer := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{})

	token, err := signer.Sign(uuid.New(), "admin", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	// Une session seule, puis une session munie d'une cle valide : refusees
	// toutes deux. Un cookie signe un navigateur, qui n'a rien a faire ici.
	alone := httptest.NewRequest(http.MethodGet, "/api/v1/bot/discord-links", nil)
	alone.AddCookie(&http.Cookie{Name: middleware.AccessCookieName, Value: token})
	withKey := botRequest(http.MethodGet, "/api/v1/bot/discord-links", "")
	withKey.AddCookie(&http.Cookie{Name: middleware.AccessCookieName, Value: token})

	for name, req := range map[string]*http.Request{"session seule": alone, "session et cle": withKey} {
		if status, _ := send(t, app, req); status != http.StatusUnauthorized {
			t.Errorf("%s : statut = %d, attendu 401", name, status)
		}
	}
	if discord.listed {
		t.Error("le handler a ete atteint avec un cookie de session")
	}
}

func TestUneCleDuBotNOuvreAucuneAutreRoute(t *testing.T) {
	app, _, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{})

	for _, path := range []string{"/api/v1/auth/me", "/api/v1/auth/me/discord", "/api/v1/admin/dashboard", "/api/v1/client/projects"} {
		if status, _ := send(t, app, botRequest(http.MethodGet, path, "")); status != http.StatusUnauthorized {
			t.Errorf("GET %s avec une cle du bot : statut = %d, attendu 401", path, status)
		}
	}
}

func TestUneCleSansLaPermissionEstInterdite(t *testing.T) {
	app, discord, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: false}, &stubLimiter{})

	status, _ := send(t, app, botRequest(http.MethodPost, "/api/v1/bot/discord-links",
		`{"code":"K7QM-4XZD","discord_user_id":"123456789012345678"}`))
	if status != http.StatusForbidden || discord.redeemed {
		t.Fatalf("statut = %d, rachat atteint = %v ; attendu 403 sans rachat", status, discord.redeemed)
	}
}

func TestRachatValideLaFormeAvantToutAutre(t *testing.T) {
	app, discord, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{})

	cases := map[string]struct {
		body  string
		field string
	}{
		"identifiant en nombre":  {`{"code":"K7QM-4XZD","discord_user_id":123456789012345678}`, "discord_user_id"},
		"identifiant trop court": {`{"code":"K7QM-4XZD","discord_user_id":"1234"}`, "discord_user_id"},
		"code mal forme":         {`{"code":"K7QM-4XZ","discord_user_id":"123456789012345678"}`, "code"},
		"code absent":            {`{"discord_user_id":"123456789012345678"}`, "code"},
	}
	for name, c := range cases {
		status, body := send(t, app, botRequest(http.MethodPost, "/api/v1/bot/discord-links", c.body))
		if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"`+c.field+`"`) {
			t.Errorf("%s : statut = %d, corps = %s ; attendu 422 sur %s", name, status, body, c.field)
		}
	}
	if discord.redeemed {
		t.Error("un rachat mal forme a atteint le service")
	}
}

func TestRachatRepond201AvecLeContrat(t *testing.T) {
	limiter := &stubLimiter{}
	app, discord, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, limiter)

	status, body := send(t, app, botRequest(http.MethodPost, "/api/v1/bot/discord-links",
		`{"code":"k7qm4xzd","discord_user_id":"123456789012345678"}`))
	if status != http.StatusCreated || !discord.redeemed {
		t.Fatalf("statut = %d (%s)", status, body)
	}
	for _, field := range []string{`"discord_user_id":"123456789012345678"`, `"role":"client"`, `"linked_at"`, `"replaced_discord_user_id":null`} {
		if !strings.Contains(body, field) {
			t.Errorf("reponse sans %s : %s", field, body)
		}
	}
	if strings.Contains(body, `"UserID"`) || strings.Contains(body, `"user_id"`) {
		t.Errorf("le compte lie fuit dans la reponse : %s", body)
	}
	// Un rachat reussi ne remet pas le compteur du membre a zero : sinon, un
	// membre muni de son propre code relancerait ses essais a volonte.
	if limiter.resets != 0 {
		t.Errorf("compteur du membre remis a zero %d fois, attendu jamais", limiter.resets)
	}
}

func TestRachatLimiteRepond429AvecRetryAfter(t *testing.T) {
	app, discord, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{blocked: true})

	res, err := app.Test(botRequest(http.MethodPost, "/api/v1/bot/discord-links",
		`{"code":"K7QM-4XZD","discord_user_id":"123456789012345678"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") != "42" {
		t.Fatalf("statut = %d, Retry-After = %q", res.StatusCode, res.Header.Get("Retry-After"))
	}
	if discord.redeemed {
		t.Error("le rachat a ete tente malgre la limite")
	}
}

func TestRachatSansCompteurEstRefuse(t *testing.T) {
	app, discord, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, errLimiter{})

	status, body := send(t, app, botRequest(http.MethodPost, "/api/v1/bot/discord-links",
		`{"code":"K7QM-4XZD","discord_user_id":"123456789012345678"}`))
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "SERVICE_UNAVAILABLE") || discord.redeemed {
		t.Fatalf("statut = %d (%s), rachat atteint = %v ; attendu 503 sans rachat", status, body, discord.redeemed)
	}
}

func TestParametresDesListesDuBot(t *testing.T) {
	app, _, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{})

	for _, path := range []string{
		"/api/v1/bot/events?limit=0",
		"/api/v1/bot/events?limit=101",
		"/api/v1/bot/events?after=-1",
		"/api/v1/bot/events?after=abc",
		"/api/v1/bot/discord-links?after=12",
		"/api/v1/bot/discord-links?limit=deux",
	} {
		if status, _ := send(t, app, botRequest(http.MethodGet, path, "")); status != http.StatusUnprocessableEntity {
			t.Errorf("GET %s : statut = %d, attendu 422", path, status)
		}
	}
}

// failingDiscord fait echouer la lecture d'une liaison sur une panne, pour
// observer ce que le journal en garde.
type failingDiscord struct{ stubDiscord }

func (failingDiscord) Link(context.Context, string) (usecase.DiscordLink, error) {
	return usecase.DiscordLink{}, errors.New("base injoignable")
}

func TestUneErreurServeurNeJournalisePasLIdentifiantDiscord(t *testing.T) {
	var logs strings.Builder
	log := slog.New(slog.NewJSONHandler(&logs, nil))

	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(log)})
	guard := middleware.NewGuard(security.NewTokenSigner([]byte(testSecret), "piilot-api"), stubPermissions{granted: true})
	guard.SetBotKeys(stubBotKeys{})
	Register(app, Deps{
		Health:    NewHealth(nil, nil, BuildInfo{Version: "test"}),
		Auth:      NewAuth(&stubAuth{}, CookieConfig{}, &stubLimiter{}, discardLogger()),
		Discord:   NewDiscord(&failingDiscord{}, &stubLimiter{}),
		BotEvents: NewBotEvents(stubBotEvents{}),
		Guard:     guard,
	})

	status, _ := send(t, app, botRequest(http.MethodGet, "/api/v1/bot/discord-links/123456789012345678", ""))
	if status != http.StatusInternalServerError {
		t.Fatalf("statut = %d, attendu 500", status)
	}
	if strings.Contains(logs.String(), "123456789012345678") {
		t.Fatalf("l'identifiant Discord est journalise : %s", logs.String())
	}
	if !strings.Contains(logs.String(), "/api/v1/bot/discord-links/:discord_user_id") {
		t.Fatalf("le motif de la route manque au journal : %s", logs.String())
	}
}

func TestRouteParIdentifiantRendLe404DuContrat(t *testing.T) {
	app, _, _ := botApp(t, stubBotKeys{}, stubPermissions{granted: true}, &stubLimiter{})

	status, body := send(t, app, botRequest(http.MethodGet, "/api/v1/bot/discord-links/123456789012345678", ""))
	if status != http.StatusNotFound || !strings.Contains(body, "DISCORD_LINK_NOT_FOUND") {
		t.Fatalf("statut = %d (%s)", status, body)
	}
}
