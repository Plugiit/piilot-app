package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// Non-regression : la garde doit s'executer AVANT le handler.
//
// Fiber enchaine les handlers dans l'ordre ou ils sont passes a Get(). Une
// garde placee en second ne s'executerait que si le handler appelait Next(),
// donc jamais. Le bug etait invisible tant que le handler repondait 501 sans
// lire les claims : il repondait « non authentifie » pour la mauvaise raison.
//
// Ce test le detecte parce qu'il presente un jeton VALIDE : si la garde ne
// tourne pas en premier, les claims sont absents et /auth/me repond 401 au
// lieu de servir le profil.
func TestMeServiLeProfilAvecUnJetonValide(t *testing.T) {
	userID := uuid.New()
	auth := &stubAuth{profile: usecase.Profile{ID: userID, Email: "test@plugiit.com", Role: "admin"}}

	app, signer := testAppWith(t, auth, &stubLimiter{})

	token, err := signer.Sign(userID, "admin", time.Minute)
	if err != nil {
		t.Fatalf("signature du jeton : %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.AccessCookieName, Value: token})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("statut = %d, attendu %d — la garde ne s'execute probablement pas avant le handler", res.StatusCode, http.StatusOK)
	}

	if !auth.meCalled {
		t.Fatal("le handler Me n'a pas ete atteint alors que le jeton etait valide")
	}
}

// Symetrique du precedent : sans jeton, le handler ne doit jamais tourner.
func TestMeRefuseSansJetonEtNAtteintPasLeHandler(t *testing.T) {
	auth := &stubAuth{}

	app, _ := testAppWith(t, auth, &stubLimiter{})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("statut = %d, attendu %d", res.StatusCode, http.StatusUnauthorized)
	}

	if auth.meCalled {
		t.Fatal("le handler Me a ete atteint sans authentification")
	}
}

// Un jeton signe par une autre cle ne doit pas passer.
func TestMeRefuseUnJetonSigneParUneAutreCle(t *testing.T) {
	auth := &stubAuth{}

	app, _ := testAppWith(t, auth, &stubLimiter{})

	intrus := security.NewTokenSigner([]byte("une-autre-cle-de-32-octets-au-moins"), "plugiit-api")

	token, err := intrus.Sign(uuid.New(), "admin", time.Minute)
	if err != nil {
		t.Fatalf("signature du jeton : %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.AccessCookieName, Value: token})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("statut = %d, attendu %d", res.StatusCode, http.StatusUnauthorized)
	}

	if auth.meCalled {
		t.Fatal("le handler Me a ete atteint avec un jeton forge")
	}
}

// La validation du login rejette un corps incomplet avant d'atteindre le
// service : inutile de solliciter la base pour un champ vide.
func TestLoginRefuseUnCorpsIncomplet(t *testing.T) {
	auth := &stubAuth{}

	app, _ := testAppWith(t, auth, &stubLimiter{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"","password":""}`))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("statut = %d, attendu %d", res.StatusCode, http.StatusUnprocessableEntity)
	}

	if auth.loginCall {
		t.Fatal("le service a ete appele malgre un corps invalide")
	}
}

// Quota depasse : la reponse doit etre 429, porter Retry-After, et surtout ne
// jamais atteindre le service — c'est le cout bcrypt qu'on refuse de payer.
func TestLoginRefuseAuDelaDuQuotaSansAppelerLeService(t *testing.T) {
	auth := &stubAuth{}

	app, _ := testAppWith(t, auth, &stubLimiter{blocked: true})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"a@b.com","password":"peu-importe"}`))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("statut = %d, attendu %d", res.StatusCode, http.StatusTooManyRequests)
	}

	if retry := res.Header.Get("Retry-After"); retry != "42" {
		t.Errorf("Retry-After = %q, attendu \"42\"", retry)
	}

	if auth.loginCall {
		t.Fatal("le service a ete appele malgre le quota depasse")
	}
}

// Une connexion reussie libere les compteurs : les tentatives ratees d'un
// utilisateur qui retrouve son mot de passe ne doivent pas le penaliser.
func TestLoginReussiRemetLesCompteursAZero(t *testing.T) {
	auth := &stubAuth{}
	limiter := &stubLimiter{}

	app, _ := testAppWith(t, auth, limiter)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"a@b.com","password":"bon-mot-de-passe"}`))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("statut = %d, attendu %d", res.StatusCode, http.StatusOK)
	}

	// Deux compteurs : celui de l'adresse et celui du compte.
	if limiter.resets != 2 {
		t.Errorf("compteurs remis a zero = %d, attendu 2", limiter.resets)
	}
}
