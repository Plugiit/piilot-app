// Package usecase orchestre les regles metier entre les handlers HTTP et la
// base. Un usecase ne connait ni Fiber, ni le format des reponses, ni les
// cookies : il recoit des valeurs et en retourne.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/repository/db"
	"github.com/plugiit/plugiit-api-go/internal/security"
)

// Profile est l'identite rendue au front. Volontairement distinct de db.User :
// ce dernier porte password_hash et totp_secret, qui ne doivent jamais quitter
// le serveur. Le seul moyen fiable de ne pas les serialiser par accident est de
// ne pas les mettre dans le type serialise.
type Profile struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Firstname string    `json:"firstname"`
	Lastname  string    `json:"lastname"`
	Role      string    `json:"role"`
	AvatarURL *string   `json:"avatar_url"`
	// Permissions du role, servies au front pour qu'il masque les actions
	// inaccessibles. C'est un confort d'affichage, pas une protection.
	Permissions []string `json:"permissions"`
}

// Session regroupe ce qu'une connexion reussie produit. Les jetons en clair ne
// existent que le temps de poser les cookies : rien ne les persiste.
type Session struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	Profile          Profile
}

// LoginInput porte les identifiants et le contexte de la requete. User-agent et
// IP sont conserves sur le jeton pour qu'une session suspecte soit
// identifiable a posteriori.
type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        *netip.Addr
}

// AuthService porte l'authentification : connexion, rotation, deconnexion.
type AuthService struct {
	pool       *pgxpool.Pool
	q          *db.Queries
	signer     *security.TokenSigner
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewAuthService construit le service. Le pool est requis en plus des requetes
// generees : la rotation d'un jeton ouvre une transaction.
func NewAuthService(pool *pgxpool.Pool, signer *security.TokenSigner, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		pool:       pool,
		q:          db.New(pool),
		signer:     signer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Login verifie les identifiants et ouvre une session.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (Session, error) {
	user, err := s.q.GetUserByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Meme cout de calcul qu'une verification reelle. Sans cela, une
			// reponse immediate signalerait que l'email n'existe pas, ce qui
			// suffit a enumerer les comptes.
			security.WasteComparison(in.Password)
			return Session{}, domain.ErrInvalidCredentials
		}
		return Session{}, fmt.Errorf("lecture du compte : %w", err)
	}

	if !security.VerifyPassword(user.PasswordHash, in.Password) {
		return Session{}, domain.ErrInvalidCredentials
	}

	// Avant l'emission : si cette ecriture echoue, aucun jeton n'a ete cree et
	// la base ne garde pas de ligne orpheline.
	if err := s.q.TouchUserLogin(ctx, user.ID); err != nil {
		return Session{}, fmt.Errorf("mise a jour de la derniere connexion : %w", err)
	}

	session, err := s.issue(ctx, s.q, user.ID, user.Role, in.UserAgent, in.IP)
	if err != nil {
		return Session{}, err
	}

	session.Profile, err = s.profile(ctx, user)
	if err != nil {
		return Session{}, err
	}

	return session, nil
}

// Refresh echange un jeton de rafraichissement contre une session neuve.
//
// La rotation est systematique : le jeton presente est revoque et remplace. Un
// jeton ne sert donc qu'une fois, ce qui rend un vol detectable — l'attaquant
// et le porteur legitime finiront par presenter le meme jeton deja consomme.
func (s *AuthService) Refresh(ctx context.Context, rawToken, userAgent string, ip *netip.Addr) (Session, error) {
	if rawToken == "" {
		return Session{}, domain.ErrSessionExpired
	}

	row, err := s.q.GetRefreshTokenWithUser(ctx, security.HashRefreshToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, domain.ErrSessionExpired
		}
		return Session{}, fmt.Errorf("lecture du jeton de rafraichissement : %w", err)
	}

	// Rejeu : un jeton deja revoque est represente. Impossible de distinguer le
	// porteur legitime de l'attaquant — l'un des deux rejoue une copie. On coupe
	// donc toutes les sessions du compte plutot que d'arbitrer a l'aveugle.
	if row.RevokedAt != nil {
		if err := s.q.RevokeAllUserRefreshTokens(ctx, row.UserID); err != nil {
			return Session{}, fmt.Errorf("revocation apres rejeu : %w", err)
		}
		return Session{}, domain.ErrSessionExpired
	}

	if time.Now().After(row.ExpiresAt) {
		return Session{}, domain.ErrSessionExpired
	}

	// Le compte a pu etre supprime depuis l'emission du jeton : la jointure de
	// la requete evite qu'un compte efface obtienne un acces neuf.
	if row.UserDeletedAt != nil {
		return Session{}, domain.ErrSessionExpired
	}

	// Revocation et emission dans la meme transaction : sans atomicite, une
	// panne entre les deux laisserait soit deux jetons valides, soit aucun.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	if err := qtx.RevokeRefreshToken(ctx, row.TokenID); err != nil {
		return Session{}, fmt.Errorf("revocation du jeton consomme : %w", err)
	}

	session, err := s.issue(ctx, qtx, row.UserID, row.UserRole, userAgent, ip)
	if err != nil {
		return Session{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("validation de la rotation : %w", err)
	}

	user, err := s.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		return Session{}, fmt.Errorf("lecture du compte : %w", err)
	}

	session.Profile, err = s.profile(ctx, user)
	if err != nil {
		return Session{}, err
	}

	return session, nil
}

// Logout revoque le jeton de rafraichissement presente.
//
// Idempotent et silencieux sur un jeton inconnu : une deconnexion ne doit
// jamais echouer, et repondre « ce jeton n'existe pas » renseignerait un
// appelant qui teste des valeurs au hasard.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}

	row, err := s.q.GetRefreshTokenWithUser(ctx, security.HashRefreshToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("lecture du jeton de rafraichissement : %w", err)
	}

	if err := s.q.RevokeRefreshToken(ctx, row.TokenID); err != nil {
		return fmt.Errorf("revocation du jeton : %w", err)
	}

	return nil
}

// Me relit l'identite de l'appelant depuis la base.
//
// Relue et non deduite des claims : une permission retiree ou un compte
// supprime prend effet immediatement, sans attendre l'expiration du jeton.
func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (Profile, error) {
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Jeton valide mais compte disparu : l'appelant n'est plus personne.
			return Profile{}, domain.ErrUnauthorized
		}
		return Profile{}, fmt.Errorf("lecture du compte : %w", err)
	}

	return s.profile(ctx, user)
}

// HasPermission indique si un role detient une permission. Consomme par la
// garde HTTP, qui ne connait ni sqlc ni le schema.
func (s *AuthService) HasPermission(ctx context.Context, roleCode, permissionCode string) (bool, error) {
	granted, err := s.q.RoleHasPermission(ctx, db.RoleHasPermissionParams{
		RoleCode:       roleCode,
		PermissionCode: permissionCode,
	})
	if err != nil {
		return false, fmt.Errorf("verification de la permission : %w", err)
	}

	return granted, nil
}

// issue emet un couple acces + rafraichissement. Prend le Querier en argument
// pour servir aussi bien hors transaction (login) que dedans (rotation).
func (s *AuthService) issue(ctx context.Context, q *db.Queries, userID uuid.UUID, role, userAgent string, ip *netip.Addr) (Session, error) {
	raw, hash, err := security.NewRefreshToken()
	if err != nil {
		return Session{}, err
	}

	now := time.Now()
	refreshExpiresAt := now.Add(s.refreshTTL)

	if _, err := q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: hash,
		UserAgent: userAgent,
		Ip:        ip,
		ExpiresAt: refreshExpiresAt,
	}); err != nil {
		return Session{}, fmt.Errorf("creation du jeton de rafraichissement : %w", err)
	}

	access, err := s.signer.Sign(userID, role, s.accessTTL)
	if err != nil {
		return Session{}, err
	}

	return Session{
		AccessToken:      access,
		AccessExpiresAt:  now.Add(s.accessTTL),
		RefreshToken:     raw,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

// profile assemble l'identite rendue au front, permissions comprises.
func (s *AuthService) profile(ctx context.Context, user db.User) (Profile, error) {
	permissions, err := s.q.ListPermissionsByRole(ctx, user.Role)
	if err != nil {
		return Profile{}, fmt.Errorf("lecture des permissions : %w", err)
	}

	return Profile{
		ID:          user.ID,
		Email:       user.Email,
		Firstname:   user.Firstname,
		Lastname:    user.Lastname,
		Role:        user.Role,
		AvatarURL:   user.AvatarUrl,
		Permissions: permissions,
	}, nil
}
