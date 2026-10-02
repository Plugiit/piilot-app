package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// Parcours publics par lien : accepter une invitation, reinitialiser un mot de
// passe oublie. Ils s'ouvrent sans session, avec un jeton a usage unique recu
// par e-mail.

// Erreurs des liens. Le message dit quoi faire, pas seulement ce qui ne va pas :
// la personne qui les lit n'a souvent aucun autre moyen de comprendre.
var (
	ErrInvitationInvalid = &domain.Error{
		Status: http.StatusNotFound, Code: "INVITATION_INVALID",
		Message: "Ce lien d'invitation n'est pas valable. Il a peut-être été remplacé par un plus récent : vérifiez vos e-mails, ou demandez une nouvelle invitation.",
	}
	ErrInvitationExpired = &domain.Error{
		Status: http.StatusGone, Code: "INVITATION_EXPIRED",
		Message: "Ce lien d'invitation a expiré. Demandez un nouveau lien à la personne à l'origine de l'invitation.",
	}
	ErrInvitationUsed = &domain.Error{
		Status: http.StatusGone, Code: "INVITATION_USED",
		Message: "Cette invitation a déjà servi. Connectez-vous avec l'adresse et le mot de passe choisis.",
	}
	ErrResetInvalid = &domain.Error{
		Status: http.StatusGone, Code: "RESET_INVALID",
		Message: "Ce lien n'est plus valable : il a expiré ou a déjà servi. Faites une nouvelle demande.",
	}
)

// InvitationView est ce que la page d'acceptation affiche.
type InvitationView struct {
	Email      string    `json:"email"`
	Firstname  string    `json:"firstname"`
	Lastname   string    `json:"lastname"`
	Role       string    `json:"role"`
	InvitedBy  string    `json:"invited_by"`
	ClientName string    `json:"client_name"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// AcceptInput porte ce que l'invite choisit en activant son compte.
type AcceptInput struct {
	Firstname string
	Lastname  string
	Password  string
	UserAgent string
	IP        *netip.Addr
}

// Invitation lit une invitation par le jeton de son lien.
func (s *AuthService) Invitation(ctx context.Context, raw string) (InvitationView, error) {
	row, err := s.openInvitation(ctx, s.q, raw)
	if err != nil {
		return InvitationView{}, err
	}

	return InvitationView{
		Email: row.Email, Firstname: row.Firstname, Lastname: row.Lastname, Role: row.Role,
		InvitedBy: row.InviterName, ClientName: row.ClientName, ExpiresAt: row.ExpiresAt,
	}, nil
}

// openInvitation lit une invitation et dit pourquoi elle ne vaut plus, le cas
// echeant.
func (s *AuthService) openInvitation(ctx context.Context, q *db.Queries, raw string) (db.GetInvitationByTokenRow, error) {
	if raw == "" {
		return db.GetInvitationByTokenRow{}, ErrInvitationInvalid
	}

	row, err := q.GetInvitationByToken(ctx, security.HashOneTimeToken(raw))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return row, ErrInvitationInvalid
	case err != nil:
		return row, fmt.Errorf("lecture de l'invitation : %w", err)
	case row.AcceptedAt != nil:
		return row, ErrInvitationUsed
	case row.RevokedAt != nil:
		return row, ErrInvitationInvalid
	case time.Now().After(row.ExpiresAt):
		return row, ErrInvitationExpired
	}

	return row, nil
}

// AcceptInvitation cree le compte de l'invite et ouvre sa session : il arrive
// connecte dans son espace, sans repasser par la page de connexion.
func (s *AuthService) AcceptInvitation(ctx context.Context, raw string, in AcceptInput) (Session, error) {
	in.Firstname = strings.TrimSpace(in.Firstname)
	in.Lastname = strings.TrimSpace(in.Lastname)

	details := map[string]any{}
	if in.Firstname == "" {
		details["firstname"] = "Votre prénom est requis"
	}
	if in.Lastname == "" {
		details["lastname"] = "Votre nom est requis"
	}
	hash, err := security.HashPassword(in.Password)
	if err != nil {
		details["password"] = passwordProblem(in.Password)
	}
	if len(details) > 0 {
		return Session{}, domain.ErrValidation.WithDetails(details)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	inv, err := s.openInvitation(ctx, q, raw)
	if err != nil {
		return Session{}, err
	}

	user, err := q.CreateInvitedUser(ctx, db.CreateInvitedUserParams{
		Email: inv.Email, PasswordHash: hash, Firstname: in.Firstname, Lastname: in.Lastname,
		Role: inv.Role, ClientID: inv.ClientID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Session{}, ErrAccountExists
		}
		return Session{}, fmt.Errorf("creation du compte : %w", err)
	}

	if err := q.AcceptInvitation(ctx, db.AcceptInvitationParams{ID: inv.ID, UserID: &user.ID}); err != nil {
		return Session{}, fmt.Errorf("cloture de l'invitation : %w", err)
	}
	if err := q.TouchUserLogin(ctx, user.ID); err != nil {
		return Session{}, fmt.Errorf("premiere connexion : %w", err)
	}

	session, err := s.issue(ctx, q, user.ID, user.Role, in.UserAgent, in.IP)
	if err != nil {
		return Session{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("validation : %w", err)
	}

	session.Profile, err = s.profile(ctx, user)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

// ForgotPassword envoie un lien de reinitialisation si l'adresse est celle
// d'un compte actif.
//
// Ne dit jamais si l'adresse existe : la reponse est la meme dans tous les cas,
// sinon le formulaire servirait a enumerer les comptes. Pour la meme raison,
// le plafond par compte (trois liens par heure) est silencieux.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if email == "" || !s.mailEnabled {
		return nil
	}

	user, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lecture du compte : %w", err)
	}
	if user.DisabledAt != nil {
		return nil
	}

	recent, err := s.q.CountRecentPasswordResets(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("comptage des demandes : %w", err)
	}
	if recent >= 3 {
		return nil
	}

	_, err = issuePasswordReset(ctx, s.pool, s.q, user, s.baseURL, s.mailEnabled)
	return err
}

// MailEnabled dit si l'instance envoie des e-mails. La page « mot de passe
// oublie » s'en sert pour dire, avant meme la saisie, qu'aucun e-mail ne
// partira.
func (s *AuthService) MailEnabled() bool { return s.mailEnabled }

// PasswordResetEmail rend l'adresse du compte d'un lien de reinitialisation
// valable. La page l'affiche pour que la personne sache quel compte elle
// modifie.
func (s *AuthService) PasswordResetEmail(ctx context.Context, raw string) (string, error) {
	row, err := s.openReset(ctx, s.q, raw)
	if err != nil {
		return "", err
	}
	return row.Email, nil
}

func (s *AuthService) openReset(ctx context.Context, q *db.Queries, raw string) (db.GetPasswordResetRow, error) {
	if raw == "" {
		return db.GetPasswordResetRow{}, ErrResetInvalid
	}

	row, err := q.GetPasswordReset(ctx, security.HashOneTimeToken(raw))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return row, ErrResetInvalid
	case err != nil:
		return row, fmt.Errorf("lecture du lien : %w", err)
	case row.UsedAt != nil, time.Now().After(row.ExpiresAt), row.UserDisabledAt != nil, row.UserDeletedAt != nil:
		return row, ErrResetInvalid
	}
	return row, nil
}

// ResetPassword change le mot de passe par un lien de reinitialisation, puis
// ferme toutes les sessions du compte et rend caducs les autres liens. Rend
// l'adresse du compte, que la page de connexion pre-remplit.
func (s *AuthService) ResetPassword(ctx context.Context, raw, password string) (string, error) {
	hash, err := security.HashPassword(password)
	if err != nil {
		return "", domain.ErrValidation.WithDetails(map[string]any{"password": passwordProblem(password)})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := s.openReset(ctx, q, raw)
	if err != nil {
		return "", err
	}

	if err := q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: row.UserID, PasswordHash: hash}); err != nil {
		return "", fmt.Errorf("ecriture du mot de passe : %w", err)
	}
	if err := q.InvalidateUserPasswordResets(ctx, row.UserID); err != nil {
		return "", fmt.Errorf("cloture des liens : %w", err)
	}
	// Un mot de passe oublie peut etre un mot de passe vole : les sessions
	// ouvertes avec l'ancien sont fermees.
	if err := q.RevokeAllUserRefreshTokens(ctx, row.UserID); err != nil {
		return "", fmt.Errorf("revocation des sessions : %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("validation : %w", err)
	}
	return row.Email, nil
}

// passwordProblem traduit le refus d'un mot de passe pour l'ecran.
func passwordProblem(password string) string {
	if len(password) < security.MinPasswordLength {
		return fmt.Sprintf("%d caractères au minimum", security.MinPasswordLength)
	}
	return "Trop long : 72 octets au maximum"
}
