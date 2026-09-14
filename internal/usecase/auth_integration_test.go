//go:build integration

// Tests d'integration de l'authentification.
//
// Isoles derriere le tag `integration` pour que `go test ./...` — lance par
// `make check` et par le Dockerfile, sans base — reste vert. Ils ne sont donc
// jamais ignores en silence : soit ils tournent (make test-integration), soit
// ils ne sont pas compiles du tout.
//
// La base cible est nommee par TEST_DATABASE_URL et non par DATABASE_URL :
// ces tests creent et suppriment des lignes, et une variable dediee rend
// impossible de les pointer par megarde sur une base qui compte.
package usecase_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/repository"
	"github.com/plugiit/plugiit-api-go/internal/security"
	"github.com/plugiit/plugiit-api-go/internal/storage"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

const (
	testAccessTTL  = 15 * time.Minute
	testRefreshTTL = 30 * 24 * time.Hour
	// Mot de passe de test : au-dessus du plancher de 12 caracteres impose par
	// security.HashPassword.
	testPassword = "mot-de-passe-de-test-suffisant"
	// Plafond d'une photo de profil, comme en production.
	testMaxAvatar = 2 * (1 << 20)
)

// newService ouvre la base et construit le service. Echoue bruyamment si
// TEST_DATABASE_URL manque : un test d'integration silencieusement ignore est
// pire qu'absent, il fait croire a une couverture qui n'existe pas.
func newService(t *testing.T) (*usecase.AuthService, *pgxpool.Pool) {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL est obligatoire : ces tests exigent une vraie base (make up, puis make test-integration)")
	}

	ctx := context.Background()

	pool, err := repository.NewPool(ctx, url, repository.DefaultPoolConfig())
	if err != nil {
		t.Fatalf("ouverture du pool : %v", err)
	}
	t.Cleanup(pool.Close)

	signer := security.NewTokenSigner([]byte("secret-de-test-suffisamment-long-32"), "plugiit-api")

	// Les avatars atterrissent dans un dossier temporaire, efface avec le test :
	// rien ici ne lit ni n'ecrit de photo, mais le service exige un stockage.
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("stockage de test : %v", err)
	}

	return usecase.NewAuthService(pool, signer, testAccessTTL, testRefreshTTL, files, testMaxAvatar), pool
}

// createUser insere un compte jetable et programme sa suppression. L'email est
// unique a chaque appel pour que deux tests puissent tourner sans se marcher
// dessus.
func createUser(t *testing.T, pool *pgxpool.Pool, role string) (uuid.UUID, string) {
	t.Helper()

	ctx := context.Background()

	hash, err := security.HashPassword(testPassword)
	if err != nil {
		t.Fatalf("hachage : %v", err)
	}

	email := "test-" + uuid.NewString() + "@plugiit.test"

	var id uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, firstname, lastname, role)
		 VALUES ($1, $2, 'Test', 'Integration', $3) RETURNING id`,
		email, hash, role,
	).Scan(&id)
	if err != nil {
		t.Fatalf("creation du compte de test : %v", err)
	}

	// Suppression physique et non soft delete : ces lignes n'ont aucune valeur
	// et ne doivent pas s'accumuler dans la base de developpement. La cascade
	// emporte les refresh_tokens.
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("nettoyage du compte de test : %v", err)
		}
	})

	return id, email
}

func login(t *testing.T, svc *usecase.AuthService, email string) usecase.Session {
	t.Helper()

	session, err := svc.Login(context.Background(), usecase.LoginInput{
		Email:     email,
		Password:  testPassword,
		UserAgent: "go-test",
	})
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}

	return session
}

func TestLoginAccepteLesBonsIdentifiantsEtChargeLesPermissions(t *testing.T) {
	svc, pool := newService(t)
	userID, email := createUser(t, pool, "admin")

	session := login(t, svc, email)

	if session.Profile.ID != userID {
		t.Errorf("id = %s, attendu %s", session.Profile.ID, userID)
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Fatal("les deux jetons doivent etre emis")
	}
	if len(session.Profile.Permissions) == 0 {
		t.Error("le profil admin doit porter des permissions")
	}
}

func TestLoginRefuseUnMauvaisMotDePasse(t *testing.T) {
	svc, pool := newService(t)
	_, email := createUser(t, pool, "admin")

	_, err := svc.Login(context.Background(), usecase.LoginInput{Email: email, Password: "ce-n-est-pas-le-bon"})
	if err == nil {
		t.Fatal("un mauvais mot de passe doit etre refuse")
	}

	if domainErr, ok := domain.AsError(err); !ok || domainErr.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("erreur = %v, attendu INVALID_CREDENTIALS", err)
	}
}

// Un email inconnu doit produire exactement la meme erreur qu'un mot de passe
// faux : toute difference renseignerait sur l'existence du compte.
func TestLoginNeDistinguePasUnEmailInconnuDUnMauvaisMotDePasse(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.Login(context.Background(), usecase.LoginInput{
		Email:    "inconnu-" + uuid.NewString() + "@plugiit.test",
		Password: testPassword,
	})

	if domainErr, ok := domain.AsError(err); !ok || domainErr.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("erreur = %v, attendu INVALID_CREDENTIALS", err)
	}
}

func TestRefreshFaitTournerLeJetonEtInvalideLAncien(t *testing.T) {
	svc, pool := newService(t)
	_, email := createUser(t, pool, "admin")

	premiere := login(t, svc, email)

	seconde, err := svc.Refresh(context.Background(), premiere.RefreshToken, "go-test", nil)
	if err != nil {
		t.Fatalf("rotation : %v", err)
	}

	if seconde.RefreshToken == premiere.RefreshToken {
		t.Fatal("le jeton de rafraichissement doit changer a chaque rotation")
	}

	// L'ancien jeton ne doit plus rien valoir.
	if _, err := svc.Refresh(context.Background(), premiere.RefreshToken, "go-test", nil); err == nil {
		t.Fatal("l'ancien jeton doit etre refuse apres rotation")
	}
}

// Le test qui compte : rejouer un jeton deja consomme doit couper TOUTES les
// sessions du compte, y compris celle qui vient d'etre legitimement emise.
// C'est ce qui rend un vol detectable au lieu de silencieux.
func TestRejeuDUnJetonConsommeRevoqueToutesLesSessions(t *testing.T) {
	svc, pool := newService(t)
	_, email := createUser(t, pool, "admin")

	premiere := login(t, svc, email)

	seconde, err := svc.Refresh(context.Background(), premiere.RefreshToken, "go-test", nil)
	if err != nil {
		t.Fatalf("rotation : %v", err)
	}

	// Rejeu du jeton deja consomme.
	if _, err := svc.Refresh(context.Background(), premiere.RefreshToken, "go-test", nil); err == nil {
		t.Fatal("le rejeu doit etre refuse")
	}

	// Represailles : le jeton legitime issu de la rotation doit lui aussi etre
	// mort. Sans cela, un attaquant garderait la main.
	_, err = svc.Refresh(context.Background(), seconde.RefreshToken, "go-test", nil)
	if err == nil {
		t.Fatal("le jeton legitime doit avoir ete revoque en represailles du rejeu")
	}

	if domainErr, ok := domain.AsError(err); !ok || domainErr.Code != "SESSION_EXPIRED" {
		t.Fatalf("erreur = %v, attendu SESSION_EXPIRED", err)
	}
}

func TestLogoutRevoqueLaSessionEtEstIdempotent(t *testing.T) {
	svc, pool := newService(t)
	_, email := createUser(t, pool, "admin")

	session := login(t, svc, email)

	if err := svc.Logout(context.Background(), session.RefreshToken); err != nil {
		t.Fatalf("deconnexion : %v", err)
	}

	if _, err := svc.Refresh(context.Background(), session.RefreshToken, "go-test", nil); err == nil {
		t.Fatal("le jeton doit etre refuse apres deconnexion")
	}

	// Rappeler logout ne doit pas echouer : le client doit pouvoir se
	// deconnecter sans savoir s'il l'etait deja.
	if err := svc.Logout(context.Background(), session.RefreshToken); err != nil {
		t.Fatalf("la deconnexion doit etre idempotente, erreur : %v", err)
	}

	if err := svc.Logout(context.Background(), "jeton-qui-n-a-jamais-existe"); err != nil {
		t.Fatalf("un jeton inconnu ne doit pas faire echouer la deconnexion : %v", err)
	}
}

func TestMeRelitLeProfilDepuisLaBase(t *testing.T) {
	svc, pool := newService(t)
	userID, email := createUser(t, pool, "admin")

	login(t, svc, email)

	profile, err := svc.Me(context.Background(), userID)
	if err != nil {
		t.Fatalf("lecture du profil : %v", err)
	}

	if profile.Email != strings.ToLower(email) {
		t.Errorf("email = %q, attendu %q", profile.Email, email)
	}
	if profile.Role != "admin" {
		t.Errorf("role = %q, attendu admin", profile.Role)
	}
}

// Le RBAC est lu en base : ce test verifie la politique reellement seedee, pas
// une correspondance codee en dur.
func TestHasPermissionSuitLaPolitiqueEnBase(t *testing.T) {
	svc, _ := newService(t)

	cas := []struct {
		role       string
		permission string
		attendu    bool
	}{
		{"admin", "projects.read", true},
		{"admin", "users.write", true},
		{"team", "projects.read", true},
		// team ne doit pas pouvoir s'octroyer des droits ni administrer les
		// comptes : c'est la separation qui protege le RBAC lui-meme.
		{"team", "users.write", false},
		{"team", "roles.write", false},
		{"client", "deliverables.validate", true},
		{"client", "projects.write", false},
		{"client", "users.read", false},
		{"admin", "permission.qui.n.existe.pas", false},
	}

	for _, tc := range cas {
		t.Run(tc.role+"/"+tc.permission, func(t *testing.T) {
			granted, err := svc.HasPermission(context.Background(), tc.role, tc.permission)
			if err != nil {
				t.Fatalf("verification : %v", err)
			}
			if granted != tc.attendu {
				t.Errorf("accorde = %v, attendu %v", granted, tc.attendu)
			}
		})
	}
}
