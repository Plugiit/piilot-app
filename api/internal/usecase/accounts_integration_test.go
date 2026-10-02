//go:build integration

// Tests d'integration de la gestion des comptes : invitations, desactivation,
// mot de passe oublie et roles.
//
// Ils figent ce que le code seul ne montre pas : un lien ne sert qu'une fois,
// un compte desactive perd l'acces partout, un mot de passe reinitialise ferme
// les sessions, et les permissions d'administration ne quittent pas le role
// admin.
package usecase_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// newAccounts construit les deux services avec l'envoi d'e-mails actif.
func newAccounts(t *testing.T) (*usecase.AccountService, *usecase.AuthService, *pgxpool.Pool) {
	t.Helper()

	auth, pool := newService(t)
	auth.SetLinks("https://piilot.test", true)

	return usecase.NewAccountService(pool, "https://piilot.test", true), auth, pool
}

// tokenOf extrait le jeton d'un lien rendu par le service.
func tokenOf(t *testing.T, link string) string {
	t.Helper()
	i := strings.LastIndex(link, "/")
	if i < 0 {
		t.Fatalf("lien inattendu : %q", link)
	}
	return link[i+1:]
}

// emailsTo compte les e-mails en file pour une adresse.
func emailsTo(t *testing.T, pool *pgxpool.Pool, address string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM email_outbox WHERE to_address = $1`, address).Scan(&n); err != nil {
		t.Fatalf("lecture de la file : %v", err)
	}
	return n
}

// cleanupEmail efface ce qui a ete cree pour une adresse de test.
func cleanupEmail(t *testing.T, pool *pgxpool.Pool, address string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM email_outbox WHERE to_address = $1`, address)
		_, _ = pool.Exec(ctx, `DELETE FROM invitations WHERE email = $1`, address)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, address)
	})
}

func TestUneInvitationCreeLeCompteEtNeSertQuUneFois(t *testing.T) {
	accounts, auth, pool := newAccounts(t)
	ctx := context.Background()
	adminID, _ := createUser(t, pool, "admin")

	email := "invite-" + uuid.NewString()[:8] + "@piilot.test"
	cleanupEmail(t, pool, email)

	result, err := accounts.Invite(ctx, usecase.InviteInput{Email: email, Role: "team", InvitedBy: adminID})
	if err != nil {
		t.Fatalf("invitation : %v", err)
	}
	if !result.Emailed || emailsTo(t, pool, email) != 1 {
		t.Fatalf("l'e-mail d'invitation n'est pas en file (emailed=%v)", result.Emailed)
	}

	// Une seconde invitation a la meme adresse est refusee : on renvoie la
	// premiere, on n'en empile pas deux.
	if _, err := accounts.Invite(ctx, usecase.InviteInput{Email: email, Role: "team", InvitedBy: adminID}); !errors.Is(err, usecase.ErrInvitationPending) {
		t.Fatalf("seconde invitation : %v, attendu ErrInvitationPending", err)
	}

	token := tokenOf(t, result.Link)
	view, err := auth.Invitation(ctx, token)
	if err != nil || view.Email != email {
		t.Fatalf("lecture de l'invitation : %+v, %v", view, err)
	}

	session, err := auth.AcceptInvitation(ctx, token, usecase.AcceptInput{
		Firstname: "Léa", Lastname: "Invitée", Password: "un-mot-de-passe-solide",
	})
	if err != nil {
		t.Fatalf("acceptation : %v", err)
	}
	if session.Profile.Email != email || session.Profile.Role != "team" || session.RefreshToken == "" {
		t.Errorf("session = %+v", session.Profile)
	}

	if _, err := auth.AcceptInvitation(ctx, token, usecase.AcceptInput{
		Firstname: "X", Lastname: "Y", Password: "un-mot-de-passe-solide",
	}); !errors.Is(err, usecase.ErrInvitationUsed) {
		t.Fatalf("seconde acceptation : %v, attendu ErrInvitationUsed", err)
	}

	// Le compte existe desormais : une nouvelle invitation est refusee.
	if _, err := accounts.Invite(ctx, usecase.InviteInput{Email: email, Role: "team", InvitedBy: adminID}); !errors.Is(err, usecase.ErrAccountExists) {
		t.Fatalf("invitation d'un compte existant : %v", err)
	}
}

func TestRenvoyerUneInvitationRendCaducLAncienLien(t *testing.T) {
	accounts, auth, pool := newAccounts(t)
	ctx := context.Background()
	adminID, _ := createUser(t, pool, "admin")

	email := "renvoi-" + uuid.NewString()[:8] + "@piilot.test"
	cleanupEmail(t, pool, email)

	first, err := accounts.Invite(ctx, usecase.InviteInput{Email: email, Role: "team", InvitedBy: adminID})
	if err != nil {
		t.Fatalf("invitation : %v", err)
	}
	second, err := accounts.ResendInvitation(ctx, first.Invitation.ID)
	if err != nil {
		t.Fatalf("renvoi : %v", err)
	}

	if _, err := auth.Invitation(ctx, tokenOf(t, first.Link)); !errors.Is(err, usecase.ErrInvitationInvalid) {
		t.Errorf("ancien lien : %v, attendu ErrInvitationInvalid", err)
	}
	if _, err := auth.Invitation(ctx, tokenOf(t, second.Link)); err != nil {
		t.Errorf("nouveau lien : %v", err)
	}
}

func TestUnCompteDesactivePerdLAccesPartout(t *testing.T) {
	accounts, auth, pool := newAccounts(t)
	ctx := context.Background()
	adminID, _ := createUser(t, pool, "admin")
	userID, email := createUser(t, pool, "team")
	session := login(t, auth, email)

	if err := accounts.Disable(ctx, adminID, userID); err != nil {
		t.Fatalf("desactivation : %v", err)
	}

	if _, active, err := auth.ActiveRole(ctx, userID); err != nil || active {
		t.Errorf("ActiveRole = %v, %v ; attendu inactif", active, err)
	}
	if _, err := auth.Refresh(ctx, session.RefreshToken, "go-test", nil); !errors.Is(err, domain.ErrSessionExpired) {
		t.Errorf("rafraichissement : %v, attendu ErrSessionExpired", err)
	}
	if _, err := auth.Login(ctx, usecase.LoginInput{Email: email, Password: testPassword}); !errors.Is(err, usecase.ErrAccountDisabled) {
		t.Errorf("connexion : %v, attendu ErrAccountDisabled", err)
	}

	if err := accounts.Enable(ctx, adminID, userID); err != nil {
		t.Fatalf("reactivation : %v", err)
	}
	login(t, auth, email)
}

func TestUnAdminNeTouchePasASonPropreCompte(t *testing.T) {
	accounts, _, pool := newAccounts(t)
	ctx := context.Background()
	adminID, _ := createUser(t, pool, "admin")

	if err := accounts.Disable(ctx, adminID, adminID); !errors.Is(err, usecase.ErrSelfChange) {
		t.Errorf("auto-desactivation : %v", err)
	}
	if err := accounts.SetRole(ctx, adminID, adminID, "team"); !errors.Is(err, usecase.ErrSelfChange) {
		t.Errorf("auto-retrogradation : %v", err)
	}
}

func TestChangerLeRoleSAppliqueALaRequeteSuivante(t *testing.T) {
	accounts, auth, pool := newAccounts(t)
	ctx := context.Background()
	adminID, _ := createUser(t, pool, "admin")
	userID, _ := createUser(t, pool, "team")

	if err := accounts.SetRole(ctx, adminID, userID, "admin"); err != nil {
		t.Fatalf("changement de role : %v", err)
	}
	role, active, err := auth.ActiveRole(ctx, userID)
	if err != nil || !active || role != "admin" {
		t.Errorf("ActiveRole = %q, %v, %v ; attendu admin", role, active, err)
	}
}

func TestMotDePasseOublieEtReinitialisation(t *testing.T) {
	_, auth, pool := newAccounts(t)
	ctx := context.Background()
	_, email := createUser(t, pool, "team")
	cleanupEmail(t, pool, email)
	session := login(t, auth, email)

	// Une adresse inconnue ne produit rien, et ne le dit pas.
	unknown := "inconnu-" + uuid.NewString()[:8] + "@piilot.test"
	if err := auth.ForgotPassword(ctx, unknown); err != nil || emailsTo(t, pool, unknown) != 0 {
		t.Fatalf("adresse inconnue : %v", err)
	}

	if err := auth.ForgotPassword(ctx, email); err != nil {
		t.Fatalf("demande : %v", err)
	}
	if emailsTo(t, pool, email) != 1 {
		t.Fatal("l'e-mail de reinitialisation n'est pas en file")
	}

	// Le lien n'est pas lisible en base : on le recupere dans le corps de
	// l'e-mail, comme le ferait son destinataire.
	var body string
	if err := pool.QueryRow(ctx, `SELECT text_body FROM email_outbox WHERE to_address = $1`, email).Scan(&body); err != nil {
		t.Fatalf("lecture de l'e-mail : %v", err)
	}
	start := strings.Index(body, "https://piilot.test/reinitialiser/")
	if start < 0 {
		t.Fatalf("lien absent de l'e-mail :\n%s", body)
	}
	link := strings.Fields(body[start:])[0]
	token := tokenOf(t, link)

	if got, err := auth.PasswordResetEmail(ctx, token); err != nil || got != email {
		t.Fatalf("lecture du lien : %q, %v", got, err)
	}
	if _, err := auth.ResetPassword(ctx, token, "court"); err == nil {
		t.Error("mot de passe trop court accepte")
	}
	if _, err := auth.ResetPassword(ctx, token, "nouveau-mot-de-passe-solide"); err != nil {
		t.Fatalf("reinitialisation : %v", err)
	}

	// Lien a usage unique, sessions fermees, nouveau mot de passe actif.
	if _, err := auth.ResetPassword(ctx, token, "encore-un-autre-mot-de-passe"); !errors.Is(err, usecase.ErrResetInvalid) {
		t.Errorf("reutilisation du lien : %v", err)
	}
	if _, err := auth.Refresh(ctx, session.RefreshToken, "go-test", nil); !errors.Is(err, domain.ErrSessionExpired) {
		t.Errorf("ancienne session : %v, attendu ErrSessionExpired", err)
	}
	if _, err := auth.Login(ctx, usecase.LoginInput{Email: email, Password: "nouveau-mot-de-passe-solide"}); err != nil {
		t.Errorf("connexion avec le nouveau mot de passe : %v", err)
	}
}

func TestLesPermissionsDAdministrationNeQuittentPasLeRoleAdmin(t *testing.T) {
	accounts, _, _ := newAccounts(t)
	ctx := context.Background()

	matrix, err := accounts.Roles(ctx)
	if err != nil {
		t.Fatalf("lecture des roles : %v", err)
	}
	var team usecase.RoleInfo
	for _, r := range matrix.Roles {
		if r.Code == "team" {
			team = r
		}
	}
	// Remise en l'etat a la fin : la base de test sert aux autres tests.
	t.Cleanup(func() {
		if _, err := accounts.SetRolePermissions(context.Background(), "team", team.Permissions); err != nil {
			t.Errorf("restauration des permissions de team : %v", err)
		}
	})

	if _, err := accounts.SetRolePermissions(ctx, "admin", nil); !errors.Is(err, usecase.ErrRoleLocked) {
		t.Errorf("modification du role admin : %v", err)
	}
	if _, err := accounts.SetRolePermissions(ctx, "team", append(slices.Clone(team.Permissions), "users.write")); err == nil {
		t.Error("users.write accordee a team")
	}
	if _, err := accounts.SetRolePermissions(ctx, "client", []string{"tasks.read"}); err == nil {
		t.Error("une permission du back-office accordee au portail")
	}

	without := slices.DeleteFunc(slices.Clone(team.Permissions), func(code string) bool { return code == "time.write" })
	updated, err := accounts.SetRolePermissions(ctx, "team", without)
	if err != nil {
		t.Fatalf("retrait de time.write : %v", err)
	}
	for _, r := range updated.Roles {
		if r.Code == "team" && slices.Contains(r.Permissions, "time.write") {
			t.Error("time.write toujours accordee a team")
		}
	}
}
