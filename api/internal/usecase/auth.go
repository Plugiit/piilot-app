// Package usecase orchestre les regles metier entre les handlers HTTP et la
// base. Un usecase ne connait ni Fiber, ni le format des reponses, ni les
// cookies : il recoit des valeurs et en retourne.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/storage"
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
	// Etat civil et coordonnees. Chaines vides plutot que nulles quand rien
	// n'est renseigne : l'ecran les pose dans des champs de saisie, qui n'ont
	// que faire de la nuance entre « vide » et « absent ».
	Gender     string `json:"gender"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`
	Country    string `json:"country"`
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
	// Photos de profil. Le meme magasin que les pieces jointes : en ouvrir un
	// second voudrait dire une seconde sauvegarde a tenir.
	files     storage.Store
	maxAvatar int64
}

// NewAuthService construit le service. Le pool est requis en plus des requetes
// generees : la rotation d'un jeton ouvre une transaction.
func NewAuthService(
	pool *pgxpool.Pool,
	signer *security.TokenSigner,
	accessTTL, refreshTTL time.Duration,
	files storage.Store,
	maxAvatar int64,
) *AuthService {
	return &AuthService{
		pool:       pool,
		q:          db.New(pool),
		signer:     signer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		files:      files,
		maxAvatar:  maxAvatar,
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
		Gender:      user.Gender,
		Phone:       user.Phone,
		Address:     user.Address,
		PostalCode:  user.PostalCode,
		City:        user.City,
		Country:     user.Country,
		Permissions: permissions,
	}, nil
}

// genders borne les valeurs admises pour le sexe.
//
// La chaine vide en fait partie : c'est ce que vaut « non renseigne », et un
// compte n'a pas a se declarer pour exister. La contrainte est aussi posee en
// base — celle-ci evite un aller-retour pour une faute de frappe, celle-la
// garantit que rien d'autre n'entre par une autre porte.
var genders = map[string]bool{
	"":          true,
	"male":      true,
	"female":    true,
	"nonbinary": true,
}

// UpdateProfileInput porte ce qu'un compte peut changer de lui-meme. Chaque
// champ absent laisse la valeur en place.
type UpdateProfileInput struct {
	Firstname  *string
	Lastname   *string
	Email      *string
	Gender     *string
	Phone      *string
	Address    *string
	PostalCode *string
	City       *string
	Country    *string
}

// UpdateProfile modifie l'etat civil et l'adresse du compte appelant.
//
// Ni le role ni les permissions n'y passent : ce sont des droits, ils se
// changent depuis l'administration des comptes. Un compte qui pourrait elever
// son propre role rendrait le RBAC decoratif.
func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (Profile, error) {
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)

		if !strings.Contains(email, "@") {
			return Profile{}, domain.ErrValidation.WithDetails(map[string]any{
				"email": "Adresse invalide",
			})
		}

		in.Email = &email
	}

	if in.Gender != nil && !genders[*in.Gender] {
		return Profile{}, domain.ErrValidation.WithDetails(map[string]any{
			"gender": "Valeur inconnue",
		})
	}

	user, err := s.q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID:         userID,
		Firstname:  in.Firstname,
		Lastname:   in.Lastname,
		Email:      in.Email,
		Gender:     in.Gender,
		Phone:      in.Phone,
		Address:    in.Address,
		PostalCode: in.PostalCode,
		City:       in.City,
		Country:    in.Country,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, domain.ErrNotFound
		}
		// L'unicite est posee en base : c'est elle qui arbitre, pas une lecture
		// prealable qui laisserait passer deux inscriptions simultanees.
		if isUniqueViolation(err) {
			return Profile{}, domain.ErrValidation.WithDetails(map[string]any{
				"email": "Cette adresse est déjà utilisée",
			})
		}
		return Profile{}, fmt.Errorf("mise a jour du compte : %w", err)
	}

	return s.profile(ctx, user)
}

// ChangePassword remplace le mot de passe apres verification de l'actuel.
//
// Le mot de passe courant est exige meme si l'appelant est deja authentifie :
// un jeton vole suffirait autrement a verrouiller le compte de son titulaire.
// Toutes les sessions sont ensuite revoquees, y compris celle qui vient de
// faire le changement — c'est le sens d'un changement de mot de passe.
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUnauthorized
		}
		return fmt.Errorf("lecture du compte : %w", err)
	}

	if !security.VerifyPassword(user.PasswordHash, current) {
		return domain.ErrValidation.WithDetails(map[string]any{
			"current_password": "Mot de passe incorrect",
		})
	}

	hash, err := security.HashPassword(next)
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{
			"password": err.Error(),
		})
	}

	if err := s.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: hash,
	}); err != nil {
		return fmt.Errorf("ecriture du mot de passe : %w", err)
	}

	if err := s.q.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("revocation des sessions : %w", err)
	}

	return nil
}

// SetAvatar pose la photo du compte, ou la retire quand l'adresse est vide.
func (s *AuthService) SetAvatar(ctx context.Context, userID uuid.UUID, url *string) (Profile, error) {
	user, err := s.q.UpdateUserAvatar(ctx, db.UpdateUserAvatarParams{
		ID:        userID,
		AvatarUrl: url,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, domain.ErrNotFound
		}
		return Profile{}, fmt.Errorf("mise a jour de la photo : %w", err)
	}

	return s.profile(ctx, user)
}

// avatarTypes borne les formats acceptes pour une photo de profil.
//
// Une liste blanche et non un refus du seul SVG : le SVG porte du script, mais
// c'est la logique inverse qui tient — on sait ce qu'un navigateur affiche
// sans risque, on ne sait pas ce qu'il fera de tout le reste.
var avatarTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// SetAvatarFile range la photo envoyee et la rattache au compte.
//
// L'ancienne est effacee apres coup : la garder ferait grossir le disque d'une
// image morte a chaque changement, et plus rien ne la designe une fois la
// nouvelle adresse ecrite.
func (s *AuthService) SetAvatarFile(
	ctx context.Context,
	userID uuid.UUID,
	contentType string,
	content io.Reader,
) (Profile, error) {
	if !avatarTypes[contentType] {
		return Profile{}, domain.ErrValidation.WithDetails(map[string]any{
			"file": "Format accepté : PNG, JPEG, WEBP ou GIF",
		})
	}

	previous, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, domain.ErrUnauthorized
		}
		return Profile{}, fmt.Errorf("lecture du compte : %w", err)
	}

	key, _, err := s.files.Save(content, s.maxAvatar)
	if err != nil {
		if errors.Is(err, storage.ErrTooLarge) {
			return Profile{}, domain.ErrValidation.WithDetails(map[string]any{
				"file": "Image trop volumineuse",
			})
		}
		return Profile{}, fmt.Errorf("ecriture de la photo : %w", err)
	}

	url := avatarURL(key)

	profile, err := s.SetAvatar(ctx, userID, &url)
	if err != nil {
		// La ligne n'a pas ete ecrite : le fichier ne doit pas rester seul.
		_ = s.files.Remove(key)

		return Profile{}, err
	}

	if old := avatarKey(previous.AvatarUrl); old != "" {
		_ = s.files.Remove(old)
	}

	return profile, nil
}

// RemoveAvatar retire la photo du compte et efface le fichier.
func (s *AuthService) RemoveAvatar(ctx context.Context, userID uuid.UUID) (Profile, error) {
	previous, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, domain.ErrUnauthorized
		}
		return Profile{}, fmt.Errorf("lecture du compte : %w", err)
	}

	profile, err := s.SetAvatar(ctx, userID, nil)
	if err != nil {
		return Profile{}, err
	}

	if old := avatarKey(previous.AvatarUrl); old != "" {
		_ = s.files.Remove(old)
	}

	return profile, nil
}

// OpenAvatar rend le contenu d'une photo rangee dans le magasin.
//
// La cle doit etre celle d'un compte : le magasin est commun aux pieces
// jointes, et ouvrir une cle quelconque reviendrait a servir les fichiers des
// projets par une porte qui n'en verifie pas les droits.
func (s *AuthService) OpenAvatar(ctx context.Context, key string) (io.ReadCloser, error) {
	url := avatarURL(key)

	known, err := s.q.AvatarURLExists(ctx, &url)
	if err != nil {
		return nil, fmt.Errorf("verification de la photo : %w", err)
	}

	if !known {
		return nil, domain.ErrNotFound
	}

	content, err := s.files.Open(key)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	return content, nil
}

// avatarPrefix est le chemin sous lequel les photos se relisent.
const avatarPrefix = "/api/v1/auth/avatars/"

func avatarURL(key string) string { return avatarPrefix + key }

// avatarKey retrouve la cle d'une adresse que nous avons ecrite.
//
// Rend une chaine vide pour tout le reste : les comptes d'avant portent des
// images en « data: » dans ce meme champ, et il n'y a rien a effacer pour
// celles-la.
func avatarKey(url *string) string {
	if url == nil || !strings.HasPrefix(*url, avatarPrefix) {
		return ""
	}

	return strings.TrimPrefix(*url, avatarPrefix)
}
