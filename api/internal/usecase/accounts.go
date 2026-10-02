package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	mailer "github.com/plugiit/piilot-app/api/internal/mail"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// Durees de vie des liens envoyes par e-mail.
const (
	// Une semaine : une invitation arrive parfois un vendredi soir a quelqu'un
	// qui ne l'ouvrira que le lundi suivant.
	invitationTTL = 7 * 24 * time.Hour
	// Une heure : un lien de reinitialisation donne la main sur un compte, il
	// ne doit pas trainer dans une boite mail.
	passwordResetTTL = time.Hour
)

// Erreurs de la gestion des comptes.
var (
	ErrAccountExists = &domain.Error{
		Status: http.StatusConflict, Code: "ACCOUNT_EXISTS",
		Message: "Un compte existe déjà avec cette adresse",
	}
	ErrInvitationPending = &domain.Error{
		Status: http.StatusConflict, Code: "INVITATION_PENDING",
		Message: "Une invitation attend déjà une réponse pour cette adresse",
	}
	ErrSelfChange = &domain.Error{
		Status: http.StatusConflict, Code: "SELF_CHANGE",
		Message: "Vous ne pouvez pas modifier le rôle ni l'état de votre propre compte",
	}
	ErrLastAdmin = &domain.Error{
		Status: http.StatusConflict, Code: "LAST_ADMIN",
		Message: "Il doit rester au moins un administrateur actif",
	}
	ErrRoleLocked = &domain.Error{
		Status: http.StatusConflict, Code: "ROLE_LOCKED",
		Message: "Les permissions de ce rôle ne se modifient pas",
	}
	ErrClientRole = &domain.Error{
		Status: http.StatusConflict, Code: "CLIENT_ROLE",
		Message: "Un compte client reste rattaché à son client : invitez plutôt la personne avec un autre rôle",
	}
)

// adminOnly : permissions qui ne quittent jamais le role admin. Les donner a
// un autre role revient a lui donner l'administration — un compte « team »
// qui pourrait gerer les comptes ou les droits pourrait s'en octroyer
// davantage, et le RBAC deviendrait decoratif.
var adminOnly = []string{"roles.write", "system.update", "users.write"}

// clientPermissions : ce qu'un compte du portail peut recevoir. Le portail ne
// voit que ses propres projets ; lui ouvrir une permission du back-office
// n'aurait pas de sens.
var clientPermissions = []string{
	"deliverables.read", "deliverables.validate", "projects.read", "tickets.read", "tickets.write",
}

// permissionGroups range les permissions par domaine, pour l'ecran des roles.
var permissionGroups = map[string]string{
	"projects": "Projets", "tasks": "Tâches", "deliverables": "Livrables",
	"tickets": "Tickets", "time": "Temps passé", "clients": "CRM",
	"users": "Comptes", "roles": "Rôles", "system": "Système",
	"dashboard": "Pilotage", "budgets": "Pilotage", "pipeline": "CRM",
}

// ClientRef designe le client d'un compte de portail.
type ClientRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Account est une ligne de l'ecran « Comptes ».
type Account struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Firstname   string     `json:"firstname"`
	Lastname    string     `json:"lastname"`
	Initials    string     `json:"initials"`
	AvatarURL   *string    `json:"avatar_url"`
	Role        string     `json:"role"`
	Client      *ClientRef `json:"client"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	// L'appelant lui-meme : l'ecran retire les actions qu'il ne peut pas
	// faire sur son propre compte.
	IsSelf bool `json:"is_self"`
}

// AccountPage est une page de l'ecran « Comptes ».
type AccountPage struct {
	Items    []Account `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	// Pastille de l'onglet « Invitations ».
	OpenInvitations int64 `json:"open_invitations"`
	// Sans envoi d'e-mails, l'ecran dit que les liens se transmettent a la
	// main.
	MailEnabled bool `json:"mail_enabled"`
}

// AccountFilters porte la barre d'outils de l'ecran.
type AccountFilters struct {
	Search   *string
	Role     *string
	Status   *string
	Page     int
	PageSize int
	Viewer   uuid.UUID
}

// Invitation est une ligne de l'onglet « Invitations ».
type Invitation struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	Firstname string     `json:"firstname"`
	Lastname  string     `json:"lastname"`
	Role      string     `json:"role"`
	Client    *ClientRef `json:"client"`
	InvitedBy string     `json:"invited_by"`
	// pending : en attente ; expired : le lien ne vaut plus, il faut la
	// renvoyer.
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// InvitationList est le contenu de l'onglet « Invitations ».
type InvitationList struct {
	Items       []Invitation `json:"items"`
	MailEnabled bool         `json:"mail_enabled"`
}

// SentLink est ce que rend l'envoi d'une invitation ou d'un lien de
// reinitialisation : le lien lui-meme, que l'admin peut copier, et s'il est
// aussi parti par e-mail.
//
// Le lien n'est rendu qu'a cet instant : seule son empreinte est stockee, il
// ne pourra plus etre relu ensuite.
type SentLink struct {
	Link      string    `json:"link"`
	Emailed   bool      `json:"emailed"`
	ExpiresAt time.Time `json:"expires_at"`
}

// InviteResult est le resultat d'une invitation.
type InviteResult struct {
	Invitation Invitation `json:"invitation"`
	SentLink
}

// InviteInput decrit une invitation a envoyer.
type InviteInput struct {
	Email     string
	Firstname string
	Lastname  string
	Role      string
	ClientID  *uuid.UUID
	InvitedBy uuid.UUID
}

// RoleInfo est une colonne de l'ecran « Roles ».
type RoleInfo struct {
	Code        string   `json:"code"`
	Label       string   `json:"label"`
	Editable    bool     `json:"editable"`
	Note        string   `json:"note"`
	Users       int64    `json:"users"`
	Permissions []string `json:"permissions"`
	// Permissions que ce role peut recevoir. Les autres sont grisees.
	Grantable []string `json:"grantable"`
}

// PermissionInfo est une ligne de l'ecran « Roles ».
type PermissionInfo struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Group     string `json:"group"`
	AdminOnly bool   `json:"admin_only"`
}

// RoleMatrix est le contenu de l'ecran « Roles ».
type RoleMatrix struct {
	Roles       []RoleInfo       `json:"roles"`
	Permissions []PermissionInfo `json:"permissions"`
}

// AccountService porte la gestion des comptes, des invitations et des roles.
type AccountService struct {
	pool        *pgxpool.Pool
	q           *db.Queries
	baseURL     string
	mailEnabled bool
}

// NewAccountService construit le service. `baseURL` est l'adresse publique de
// l'application, celle des liens envoyes par e-mail.
func NewAccountService(pool *pgxpool.Pool, baseURL string, mailEnabled bool) *AccountService {
	return &AccountService{
		pool:        pool,
		q:           db.New(pool),
		baseURL:     strings.TrimRight(baseURL, "/"),
		mailEnabled: mailEnabled,
	}
}

// List rend une page de comptes.
func (s *AccountService) List(ctx context.Context, f AccountFilters) (AccountPage, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 25
	}

	total, err := s.q.CountAccounts(ctx, db.CountAccountsParams{Role: f.Role, Status: f.Status, Search: f.Search})
	if err != nil {
		return AccountPage{}, fmt.Errorf("comptage des comptes : %w", err)
	}

	rows, err := s.q.ListAccounts(ctx, db.ListAccountsParams{
		Role: f.Role, Status: f.Status, Search: f.Search,
		PageSize: int32(f.PageSize), PageOffset: int32((f.Page - 1) * f.PageSize),
	})
	if err != nil {
		return AccountPage{}, fmt.Errorf("lecture des comptes : %w", err)
	}

	open, err := s.q.CountOpenInvitations(ctx)
	if err != nil {
		return AccountPage{}, fmt.Errorf("comptage des invitations : %w", err)
	}

	items := make([]Account, 0, len(rows))
	for _, row := range rows {
		account := Account{
			ID:          row.ID,
			Email:       row.Email,
			Firstname:   row.Firstname,
			Lastname:    row.Lastname,
			Initials:    initialsOf(row.Firstname, row.Lastname),
			AvatarURL:   row.AvatarUrl,
			Role:        row.Role,
			Status:      "active",
			LastLoginAt: row.LastLoginAt,
			CreatedAt:   row.CreatedAt,
			IsSelf:      row.ID == f.Viewer,
		}
		if row.DisabledAt != nil {
			account.Status = "disabled"
		}
		if row.ClientID != nil && row.ClientName != nil {
			account.Client = &ClientRef{ID: *row.ClientID, Name: *row.ClientName}
		}
		items = append(items, account)
	}

	return AccountPage{
		Items: items, Total: total, Page: f.Page, PageSize: f.PageSize,
		OpenInvitations: open, MailEnabled: s.mailEnabled,
	}, nil
}

// Invitations rend les invitations qui attendent une reponse.
func (s *AccountService) Invitations(ctx context.Context) (InvitationList, error) {
	rows, err := s.q.ListOpenInvitations(ctx)
	if err != nil {
		return InvitationList{}, fmt.Errorf("lecture des invitations : %w", err)
	}

	items := make([]Invitation, 0, len(rows))
	for _, row := range rows {
		items = append(items, invitationOf(
			row.ID, row.Email, row.Firstname, row.Lastname, row.Role, row.ClientID, row.ClientName,
			row.InviterName, row.ExpiresAt, row.CreatedAt,
		))
	}

	return InvitationList{Items: items, MailEnabled: s.mailEnabled}, nil
}

func invitationOf(
	id uuid.UUID, email, firstname, lastname, role string, clientID *uuid.UUID, clientName, inviter string,
	expiresAt, createdAt time.Time,
) Invitation {
	inv := Invitation{
		ID: id, Email: email, Firstname: firstname, Lastname: lastname, Role: role,
		InvitedBy: inviter, Status: "pending", ExpiresAt: expiresAt, CreatedAt: createdAt,
	}
	if time.Now().After(expiresAt) {
		inv.Status = "expired"
	}
	if clientID != nil {
		inv.Client = &ClientRef{ID: *clientID, Name: clientName}
	}
	return inv
}

// Invite cree une invitation et l'envoie par e-mail si l'envoi est configure.
func (s *AccountService) Invite(ctx context.Context, in InviteInput) (InviteResult, error) {
	in.Email = strings.TrimSpace(in.Email)
	in.Firstname = strings.TrimSpace(in.Firstname)
	in.Lastname = strings.TrimSpace(in.Lastname)

	details := map[string]any{}
	if _, err := mail.ParseAddress(in.Email); err != nil || !strings.Contains(in.Email, "@") {
		details["email"] = "Adresse invalide"
	}
	switch in.Role {
	case "admin", "team":
		in.ClientID = nil
	case "client":
		if in.ClientID == nil {
			details["client_id"] = "Choisissez le client dont la personne suivra les projets"
		}
	default:
		details["role"] = "Rôle inconnu"
	}
	if len(details) > 0 {
		return InviteResult{}, domain.ErrValidation.WithDetails(details)
	}

	if _, err := s.q.GetUserByEmail(ctx, in.Email); err == nil {
		return InviteResult{}, ErrAccountExists
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return InviteResult{}, fmt.Errorf("lecture du compte : %w", err)
	}

	if open, err := s.q.GetOpenInvitationByEmail(ctx, in.Email); err == nil {
		return InviteResult{}, ErrInvitationPending.WithDetails(map[string]any{"invitation_id": open.ID})
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return InviteResult{}, fmt.Errorf("lecture des invitations : %w", err)
	}

	raw, hash, err := security.NewOneTimeToken()
	if err != nil {
		return InviteResult{}, err
	}
	expires := time.Now().Add(invitationTTL)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InviteResult{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	inviter := &in.InvitedBy
	row, err := q.CreateInvitation(ctx, db.CreateInvitationParams{
		Email: in.Email, Firstname: in.Firstname, Lastname: in.Lastname, Role: in.Role,
		ClientID: in.ClientID, TokenHash: hash, InvitedBy: inviter, ExpiresAt: expires,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return InviteResult{}, ErrInvitationPending
		}
		if isForeignKeyViolation(err) {
			return InviteResult{}, domain.ErrValidation.WithDetails(map[string]any{"client_id": "Client introuvable"})
		}
		return InviteResult{}, fmt.Errorf("creation de l'invitation : %w", err)
	}

	link := s.baseURL + "/invitation/" + raw
	emailed, err := s.sendInvitation(ctx, q, row.ID, row.Email, row.Firstname, link, expires)
	if err != nil {
		return InviteResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return InviteResult{}, fmt.Errorf("validation de l'invitation : %w", err)
	}

	full, err := s.q.GetInvitationByToken(ctx, hash)
	if err != nil {
		return InviteResult{}, fmt.Errorf("relecture de l'invitation : %w", err)
	}

	return InviteResult{
		Invitation: invitationOf(full.ID, full.Email, full.Firstname, full.Lastname, full.Role,
			full.ClientID, full.ClientName, full.InviterName, full.ExpiresAt, full.CreatedAt),
		SentLink: SentLink{Link: link, Emailed: emailed, ExpiresAt: expires},
	}, nil
}

// sendInvitation depose l'e-mail d'invitation dans la file, dans la meme
// transaction que l'invitation : l'une n'existe pas sans l'autre.
func (s *AccountService) sendInvitation(
	ctx context.Context, q *db.Queries, invitationID uuid.UUID, email, firstname, link string, expires time.Time,
) (bool, error) {
	if !s.mailEnabled {
		return false, nil
	}

	inv, err := q.GetInvitation(ctx, invitationID)
	if err != nil {
		return false, fmt.Errorf("relecture de l'invitation : %w", err)
	}

	var inviter, clientName string
	if inv.InvitedBy != nil {
		if u, err := q.GetUserByID(ctx, *inv.InvitedBy); err == nil {
			inviter = strings.TrimSpace(u.Firstname + " " + u.Lastname)
		}
	}
	if inv.ClientID != nil {
		if c, err := q.GetClientName(ctx, *inv.ClientID); err == nil {
			clientName = c
		}
	}

	msg, err := mailer.Invitation(email, firstname, inviter, clientName, link, expires)
	if err != nil {
		return false, err
	}

	return true, enqueue(ctx, q, msg)
}

// ResendInvitation remplace le jeton d'une invitation et la renvoie. L'ancien
// lien cesse de valoir.
func (s *AccountService) ResendInvitation(ctx context.Context, id uuid.UUID) (SentLink, error) {
	raw, hash, err := security.NewOneTimeToken()
	if err != nil {
		return SentLink{}, err
	}
	expires := time.Now().Add(invitationTTL)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SentLink{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.RenewInvitation(ctx, db.RenewInvitationParams{ID: id, TokenHash: hash, ExpiresAt: expires})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SentLink{}, domain.ErrNotFound
		}
		return SentLink{}, fmt.Errorf("renouvellement de l'invitation : %w", err)
	}

	link := s.baseURL + "/invitation/" + raw
	emailed, err := s.sendInvitation(ctx, q, row.ID, row.Email, row.Firstname, link, expires)
	if err != nil {
		return SentLink{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return SentLink{}, fmt.Errorf("validation : %w", err)
	}

	return SentLink{Link: link, Emailed: emailed, ExpiresAt: expires}, nil
}

// RevokeInvitation annule une invitation : son lien cesse de valoir.
func (s *AccountService) RevokeInvitation(ctx context.Context, id uuid.UUID) error {
	if err := s.q.RevokeInvitation(ctx, id); err != nil {
		return fmt.Errorf("annulation de l'invitation : %w", err)
	}
	return nil
}

// SetRole change le role d'un compte interne.
//
// Prend effet a la requete suivante du compte : la garde relit le role en
// base a chaque requete.
func (s *AccountService) SetRole(ctx context.Context, actor, id uuid.UUID, role string) error {
	if role != "admin" && role != "team" {
		return domain.ErrValidation.WithDetails(map[string]any{"role": "Rôle attendu : admin ou team"})
	}

	user, err := s.guardChange(ctx, actor, id)
	if err != nil {
		return err
	}
	if user.Role == "client" {
		return ErrClientRole
	}
	if user.Role == role {
		return nil
	}
	if user.Role == "admin" && user.DisabledAt == nil {
		if err := s.keepOneAdmin(ctx); err != nil {
			return err
		}
	}

	if err := s.q.SetUserRole(ctx, db.SetUserRoleParams{ID: id, Role: role}); err != nil {
		return fmt.Errorf("changement de role : %w", err)
	}
	return nil
}

// Disable desactive un compte et ferme toutes ses sessions.
func (s *AccountService) Disable(ctx context.Context, actor, id uuid.UUID) error {
	user, err := s.guardChange(ctx, actor, id)
	if err != nil {
		return err
	}
	if user.DisabledAt != nil {
		return nil
	}
	if user.Role == "admin" {
		if err := s.keepOneAdmin(ctx); err != nil {
			return err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if err := q.DisableUser(ctx, id); err != nil {
		return fmt.Errorf("desactivation : %w", err)
	}
	// Les jetons d'acces en cours ne valent deja plus — la garde relit l'etat
	// du compte —, les jetons de rafraichissement sont revoques pour qu'aucune
	// session ne puisse renaitre.
	if err := q.RevokeAllUserRefreshTokens(ctx, id); err != nil {
		return fmt.Errorf("revocation des sessions : %w", err)
	}
	if err := q.InvalidateUserPasswordResets(ctx, id); err != nil {
		return fmt.Errorf("annulation des liens de reinitialisation : %w", err)
	}

	return tx.Commit(ctx)
}

// Enable reactive un compte.
func (s *AccountService) Enable(ctx context.Context, actor, id uuid.UUID) error {
	if _, err := s.guardChange(ctx, actor, id); err != nil {
		return err
	}
	if err := s.q.EnableUser(ctx, id); err != nil {
		return fmt.Errorf("reactivation : %w", err)
	}
	return nil
}

// PasswordResetLink cree un lien de reinitialisation pour un compte, a la
// demande d'un admin. C'est le recours quand la personne n'a plus acces a sa
// boite mail, ou quand l'instance n'envoie pas d'e-mails.
func (s *AccountService) PasswordResetLink(ctx context.Context, actor, id uuid.UUID) (SentLink, error) {
	user, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SentLink{}, domain.ErrNotFound
		}
		return SentLink{}, fmt.Errorf("lecture du compte : %w", err)
	}
	if user.DisabledAt != nil {
		return SentLink{}, domain.ErrValidation.WithDetails(map[string]any{
			"account": "Réactivez le compte avant de lui envoyer un lien",
		})
	}

	return issuePasswordReset(ctx, s.pool, s.q, user, s.baseURL, s.mailEnabled)
}

// guardChange lit le compte vise et refuse qu'un admin agisse sur le sien : se
// retirer son propre role ou se desactiver ne se rattrape pas sans un autre
// admin.
func (s *AccountService) guardChange(ctx context.Context, actor, id uuid.UUID) (db.User, error) {
	if actor == id {
		return db.User{}, ErrSelfChange
	}

	user, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, domain.ErrNotFound
		}
		return db.User{}, fmt.Errorf("lecture du compte : %w", err)
	}
	return user, nil
}

// keepOneAdmin refuse l'operation si elle retirerait le dernier admin actif.
func (s *AccountService) keepOneAdmin(ctx context.Context) error {
	admins, err := s.q.CountActiveAdmins(ctx)
	if err != nil {
		return fmt.Errorf("comptage des administrateurs : %w", err)
	}
	if admins <= 1 {
		return ErrLastAdmin
	}
	return nil
}

// Roles rend la matrice des roles et des permissions.
func (s *AccountService) Roles(ctx context.Context) (RoleMatrix, error) {
	roles, err := s.q.ListRoles(ctx, db.ListRolesParams{PageSize: 50, PageOffset: 0})
	if err != nil {
		return RoleMatrix{}, fmt.Errorf("lecture des roles : %w", err)
	}
	perms, err := s.q.ListPermissions(ctx)
	if err != nil {
		return RoleMatrix{}, fmt.Errorf("lecture des permissions : %w", err)
	}
	grants, err := s.q.ListRolePermissionCodes(ctx)
	if err != nil {
		return RoleMatrix{}, fmt.Errorf("lecture des droits : %w", err)
	}
	counts, err := s.q.CountUsersByRole(ctx)
	if err != nil {
		return RoleMatrix{}, fmt.Errorf("comptage par role : %w", err)
	}

	byRole := map[string][]string{}
	for _, g := range grants {
		byRole[g.RoleCode] = append(byRole[g.RoleCode], g.PermissionCode)
	}
	users := map[string]int64{}
	for _, c := range counts {
		users[c.Role] = c.Total
	}

	all := make([]string, 0, len(perms))
	matrix := RoleMatrix{Permissions: make([]PermissionInfo, 0, len(perms))}
	for _, p := range perms {
		all = append(all, p.Code)
		prefix, _, _ := strings.Cut(p.Code, ".")
		group := permissionGroups[prefix]
		if group == "" {
			group = "Autres"
		}
		matrix.Permissions = append(matrix.Permissions, PermissionInfo{
			Code: p.Code, Label: p.Label, Group: group, AdminOnly: slices.Contains(adminOnly, p.Code),
		})
	}

	// Ordre de lecture : du plus large au plus restreint.
	order := map[string]int{"admin": 0, "team": 1, "client": 2}
	slices.SortStableFunc(roles, func(a, b db.Role) int { return rank(order, a.Code) - rank(order, b.Code) })

	for _, r := range roles {
		info := RoleInfo{
			Code: r.Code, Label: r.Label, Users: users[r.Code],
			Permissions: nonNil(byRole[r.Code]), Grantable: grantableFor(r.Code, all),
			Editable: r.Code != "admin",
		}
		switch r.Code {
		case "admin":
			info.Note = "Toutes les permissions, toujours : c'est ce qui garantit qu'un administrateur peut tout rattraper."
		case "client":
			info.Note = "Comptes du portail : seules les permissions du portail s'appliquent."
		}
		matrix.Roles = append(matrix.Roles, info)
	}

	return matrix, nil
}

// SetRolePermissions remplace les permissions d'un role.
func (s *AccountService) SetRolePermissions(ctx context.Context, role string, permissions []string) (RoleMatrix, error) {
	if role == "admin" {
		return RoleMatrix{}, ErrRoleLocked
	}
	if _, err := s.q.GetRoleByCode(ctx, role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RoleMatrix{}, domain.ErrNotFound
		}
		return RoleMatrix{}, fmt.Errorf("lecture du role : %w", err)
	}

	perms, err := s.q.ListPermissions(ctx)
	if err != nil {
		return RoleMatrix{}, fmt.Errorf("lecture des permissions : %w", err)
	}
	all := make([]string, 0, len(perms))
	for _, p := range perms {
		all = append(all, p.Code)
	}
	grantable := grantableFor(role, all)

	for _, code := range permissions {
		if !slices.Contains(grantable, code) {
			return RoleMatrix{}, domain.ErrValidation.WithDetails(map[string]any{
				"permissions": fmt.Sprintf("« %s » ne peut pas être accordée à ce rôle", code),
			})
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RoleMatrix{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if err := q.ClearRolePermissions(ctx, role); err != nil {
		return RoleMatrix{}, fmt.Errorf("effacement des permissions : %w", err)
	}
	for _, code := range permissions {
		if err := q.GrantRolePermission(ctx, db.GrantRolePermissionParams{RoleCode: role, PermissionCode: code}); err != nil {
			return RoleMatrix{}, fmt.Errorf("attribution de %s : %w", code, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return RoleMatrix{}, fmt.Errorf("validation : %w", err)
	}

	return s.Roles(ctx)
}

// grantableFor rend les permissions qu'un role peut recevoir.
func grantableFor(role string, all []string) []string {
	switch role {
	case "admin":
		return all
	case "client":
		return clientPermissions
	default:
		out := make([]string, 0, len(all))
		for _, code := range all {
			if !slices.Contains(adminOnly, code) {
				out = append(out, code)
			}
		}
		return out
	}
}

func rank(order map[string]int, code string) int {
	if r, ok := order[code]; ok {
		return r
	}
	return len(order)
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// enqueue depose un e-mail dans la file d'envoi.
func enqueue(ctx context.Context, q *db.Queries, msg mailer.Message) error {
	if err := q.EnqueueEmail(ctx, db.EnqueueEmailParams{
		Kind: msg.Kind, ToAddress: msg.To, Subject: msg.Subject, TextBody: msg.Text, HtmlBody: msg.HTML,
	}); err != nil {
		return fmt.Errorf("mise en file de l'e-mail : %w", err)
	}
	return nil
}

// issuePasswordReset cree un lien de reinitialisation et l'envoie si l'envoi
// est configure. Partage par la demande d'un admin et par « mot de passe
// oublie ».
func issuePasswordReset(
	ctx context.Context, pool *pgxpool.Pool, q *db.Queries, user db.User, baseURL string, mailEnabled bool,
) (SentLink, error) {
	raw, hash, err := security.NewOneTimeToken()
	if err != nil {
		return SentLink{}, err
	}
	expires := time.Now().Add(passwordResetTTL)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return SentLink{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := q.WithTx(tx)

	if _, err := qtx.CreatePasswordReset(ctx, db.CreatePasswordResetParams{
		UserID: user.ID, TokenHash: hash, ExpiresAt: expires,
	}); err != nil {
		return SentLink{}, fmt.Errorf("creation du lien : %w", err)
	}

	link := strings.TrimRight(baseURL, "/") + "/reinitialiser/" + raw
	if mailEnabled {
		msg, err := mailer.PasswordReset(user.Email, user.Firstname, link, expires)
		if err != nil {
			return SentLink{}, err
		}
		if err := enqueue(ctx, qtx, msg); err != nil {
			return SentLink{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return SentLink{}, fmt.Errorf("validation : %w", err)
	}

	return SentLink{Link: link, Emailed: mailEnabled, ExpiresAt: expires}, nil
}
