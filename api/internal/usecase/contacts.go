package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// CrmContactItem est une ligne du tableau « Contacts ».
//
// Elle porte le nom de son client : hors de la fiche d'un client, une personne
// sans entreprise ne dit pas de qui l'on parle.
type CrmContactItem struct {
	ID uuid.UUID `json:"id"`
	// Nuls tant que la personne n'a pas d'entreprise : un contact peut naître
	// libre et se rattacher ensuite.
	ClientID   *uuid.UUID `json:"client_id"`
	ClientName *string    `json:"client_name"`
	Firstname  string     `json:"firstname"`
	Lastname   string     `json:"lastname"`
	Role       string     `json:"role"`
	Email      *string    `json:"email"`
	Phone      string     `json:"phone"`
	// IsPrimary dit si le client designe cette personne comme interlocuteur
	// principal.
	IsPrimary bool      `json:"is_primary"`
	CreatedAt time.Time `json:"created_at"`
}

// CrmContactPage est une page de la liste.
type CrmContactPage struct {
	Items    []CrmContactItem `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// CrmContactFilters porte ce que la barre d'outils peut demander.
type CrmContactFilters struct {
	Search   *string
	ClientID *uuid.UUID
	// OnlyFree ne garde que les contacts sans entreprise.
	OnlyFree bool
	Sort     string
	Dir      string
	Page     int
	PageSize int
}

var crmContactSorts = map[string]struct{}{
	"name":   {},
	"client": {},
}

// ContactService sert les ecrans « Contacts ».
type ContactService struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewContactService(pool *pgxpool.Pool) *ContactService {
	return &ContactService{pool: pool, q: db.New(pool)}
}

// List renvoie une page du tableau des contacts.
func (s *ContactService) List(ctx context.Context, f CrmContactFilters) (CrmContactPage, error) {
	if _, ok := crmContactSorts[f.Sort]; !ok {
		f.Sort = "name"
	}
	if f.Dir != "desc" {
		f.Dir = "asc"
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 25
	}

	rows, err := s.q.ListCrmContacts(ctx, db.ListCrmContactsParams{
		Search:     f.Search,
		ClientID:   f.ClientID,
		OnlyFree:   f.OnlyFree,
		Sort:       f.Sort,
		Dir:        f.Dir,
		PageSize:   int32(f.PageSize),
		PageOffset: int32((f.Page - 1) * f.PageSize),
	})
	if err != nil {
		return CrmContactPage{}, fmt.Errorf("liste des contacts : %w", err)
	}

	total, err := s.q.CountCrmContacts(ctx, db.CountCrmContactsParams{
		Search:   f.Search,
		ClientID: f.ClientID,
		OnlyFree: f.OnlyFree,
	})
	if err != nil {
		return CrmContactPage{}, fmt.Errorf("compte des contacts : %w", err)
	}

	items := make([]CrmContactItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, CrmContactItem{
			ID:         row.ID,
			ClientID:   row.ClientID,
			ClientName: row.ClientName,
			Firstname:  row.Firstname,
			Lastname:   row.Lastname,
			Role:       row.Role,
			Email:      row.Email,
			Phone:      row.Phone,
			IsPrimary:  row.IsPrimary,
			CreatedAt:  row.CreatedAt,
		})
	}

	return CrmContactPage{
		Items:    items,
		Total:    total,
		Page:     f.Page,
		PageSize: f.PageSize,
	}, nil
}

// CreateContactInput decrit un contact a creer.
type CreateContactInput struct {
	// Nul pour un contact libre : on rencontre quelqu'un avant de savoir chez
	// qui il travaille.
	ClientID  *uuid.UUID
	Firstname string
	Lastname  string
	Role      string
	Email     string
	Phone     string
	// Primary designe la personne comme interlocuteur principal dans la foulee,
	// ce qui evite un second geste sur le cas courant du premier contact.
	Primary bool
}

// Create inscrit un contact, chez un client ou libre.
func (s *ContactService) Create(ctx context.Context, in CreateContactInput) (CrmContactItem, error) {
	firstname := strings.TrimSpace(in.Firstname)
	lastname := strings.TrimSpace(in.Lastname)

	if firstname == "" && lastname == "" {
		return CrmContactItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"firstname": "Un prénom ou un nom est requis",
		})
	}

	email := strings.TrimSpace(in.Email)
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return CrmContactItem{}, domain.ErrValidation.WithDetails(map[string]any{
				"email": "Adresse e-mail invalide",
			})
		}
	}

	// Le client n'est lu que s'il est designe : un contact libre n'en a pas, et
	// c'est un cas normal, pas une absence a signaler.
	var client *db.Client
	if in.ClientID != nil {
		found, err := s.q.GetClientByID(ctx, *in.ClientID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return CrmContactItem{}, domain.ErrValidation.WithDetails(map[string]any{
					"client_id": "Client introuvable",
				})
			}

			return CrmContactItem{}, fmt.Errorf("lecture du client : %w", err)
		}

		client = &found
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CrmContactItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	var contactEmail *string
	if email != "" {
		contactEmail = &email
	}

	contact, err := qtx.CreateContact(ctx, db.CreateContactParams{
		ClientID:  in.ClientID,
		Firstname: firstname,
		Lastname:  lastname,
		Role:      strings.TrimSpace(in.Role),
		Email:     contactEmail,
		Phone:     strings.TrimSpace(in.Phone),
	})
	if err != nil {
		// Index unique (client_id, lower(email)) : la meme personne ne
		// s'inscrit pas deux fois chez le meme client.
		if isUniqueViolation(err) {
			return CrmContactItem{}, domain.ErrConflict.WithDetails(map[string]any{
				"email": "Ce contact existe déjà chez ce client",
			})
		}

		return CrmContactItem{}, fmt.Errorf("creation du contact : %w", err)
	}

	// Un contact libre ne peut etre principal de personne : la cle etrangere
	// composite exige qu'il appartienne d'abord au client.
	primary := false
	if client != nil {
		// Le tout premier contact devient principal sans qu'on le demande : un
		// client qui n'a qu'un interlocuteur n'a pas de choix a exprimer.
		primary = in.Primary || client.PrimaryContactID == nil

		if primary {
			if err := qtx.SetPrimaryContact(ctx, db.SetPrimaryContactParams{
				ClientID:  *in.ClientID,
				ContactID: &contact.ID,
			}); err != nil {
				return CrmContactItem{}, fmt.Errorf("designation du contact principal : %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CrmContactItem{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	var clientName *string
	if client != nil {
		clientName = &client.Name
	}

	return CrmContactItem{
		ID:         contact.ID,
		ClientID:   contact.ClientID,
		ClientName: clientName,
		Firstname:  contact.Firstname,
		Lastname:   contact.Lastname,
		Role:       contact.Role,
		Email:      contact.Email,
		Phone:      contact.Phone,
		IsPrimary:  primary,
		CreatedAt:  contact.CreatedAt,
	}, nil
}

// ContactOption est une entree d'un menu deroulant de contacts.
//
// `IsFree` distingue les contacts du client de ceux qui n'ont pas encore
// d'entreprise : choisir l'un des seconds le rattache, ce que l'ecran annonce
// avant le clic plutot qu'apres.
type ContactOption struct {
	ContactRef
	IsFree bool `json:"is_free"`
}

// ListOfClient alimente le menu deroulant qui designe le contact principal :
// les contacts du client, puis les contacts libres.
func (s *ContactService) ListOfClient(ctx context.Context, clientID uuid.UUID, search *string) ([]ContactOption, error) {
	rows, err := s.q.ListContactsOfClient(ctx, db.ListContactsOfClientParams{
		ClientID: &clientID,
		Search:   search,
		PageSize: 50,
	})
	if err != nil {
		return nil, fmt.Errorf("contacts du client : %w", err)
	}

	items := make([]ContactOption, 0, len(rows))
	for _, row := range rows {
		items = append(items, ContactOption{
			ContactRef: ContactRef{
				ID:        row.ID,
				Firstname: row.Firstname,
				Lastname:  row.Lastname,
				Role:      row.Role,
				Email:     row.Email,
			},
			IsFree: row.IsFree,
		})
	}

	return items, nil
}

// ListFree alimente le menu deroulant du formulaire de creation d'un client :
// seuls les contacts sans entreprise, qu'un nouveau client peut adopter.
func (s *ContactService) ListFree(ctx context.Context, search *string) ([]ContactRef, error) {
	rows, err := s.q.ListFreeContacts(ctx, db.ListFreeContactsParams{
		Search:   search,
		PageSize: 50,
	})
	if err != nil {
		return nil, fmt.Errorf("contacts libres : %w", err)
	}

	items := make([]ContactRef, 0, len(rows))
	for _, row := range rows {
		items = append(items, ContactRef{
			ID:        row.ID,
			Firstname: row.Firstname,
			Lastname:  row.Lastname,
			Role:      row.Role,
			Email:     row.Email,
		})
	}

	return items, nil
}

// UpdateContactInput decrit les champs modifiables d'un contact.
//
// Le client n'en fait pas partie : rattacher quelqu'un passe par la
// designation du contact principal, seule a savoir tenir la cle etrangere
// composite dans le bon ordre.
type UpdateContactInput struct {
	Firstname string
	Lastname  string
	Role      string
	Email     string
	Phone     string
}

// Update modifie l'identite d'un contact.
func (s *ContactService) Update(ctx context.Context, id uuid.UUID, in UpdateContactInput) (CrmContactItem, error) {
	firstname := strings.TrimSpace(in.Firstname)
	lastname := strings.TrimSpace(in.Lastname)

	if firstname == "" && lastname == "" {
		return CrmContactItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"firstname": "Un prénom ou un nom est requis",
		})
	}

	email := strings.TrimSpace(in.Email)
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return CrmContactItem{}, domain.ErrValidation.WithDetails(map[string]any{
				"email": "Adresse e-mail invalide",
			})
		}
	}

	var contactEmail *string
	if email != "" {
		contactEmail = &email
	}

	row, err := s.q.UpdateContact(ctx, db.UpdateContactParams{
		ID:        id,
		Firstname: firstname,
		Lastname:  lastname,
		Role:      strings.TrimSpace(in.Role),
		Email:     contactEmail,
		Phone:     strings.TrimSpace(in.Phone),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CrmContactItem{}, domain.ErrNotFound
		}
		if isUniqueViolation(err) {
			return CrmContactItem{}, domain.ErrConflict.WithDetails(map[string]any{
				"email": "Un autre contact de ce client porte déjà cette adresse",
			})
		}

		return CrmContactItem{}, fmt.Errorf("modification du contact : %w", err)
	}

	return s.itemOf(ctx, row)
}

// Delete efface un contact.
//
// La designation qui le vise est retiree d'abord : la cle etrangere ne se
// declenche que sur une suppression reelle, et laisserait sinon un client
// pointant vers une ligne morte.
func (s *ContactService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.q.GetContact(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}

		return fmt.Errorf("lecture du contact : %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	if err := qtx.ClearPrimaryContactOf(ctx, &id); err != nil {
		return fmt.Errorf("retrait de la designation : %w", err)
	}

	if err := qtx.SoftDeleteContact(ctx, id); err != nil {
		return fmt.Errorf("suppression du contact : %w", err)
	}

	return tx.Commit(ctx)
}

// itemOf complete une ligne de contact par le nom de son client et son etat de
// contact principal, que la table seule ne porte pas.
func (s *ContactService) itemOf(ctx context.Context, row db.Contact) (CrmContactItem, error) {
	item := CrmContactItem{
		ID:        row.ID,
		ClientID:  row.ClientID,
		Firstname: row.Firstname,
		Lastname:  row.Lastname,
		Role:      row.Role,
		Email:     row.Email,
		Phone:     row.Phone,
		CreatedAt: row.CreatedAt,
	}

	if row.ClientID == nil {
		return item, nil
	}

	client, err := s.q.GetClientByID(ctx, *row.ClientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return item, nil
		}

		return CrmContactItem{}, fmt.Errorf("lecture du client : %w", err)
	}

	item.ClientName = &client.Name
	item.IsPrimary = client.PrimaryContactID != nil && *client.PrimaryContactID == row.ID

	return item, nil
}
