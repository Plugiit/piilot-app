package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// ContactRef est le contact principal tel qu'une ligne de client le montre :
// de quoi nommer l'interlocuteur, rien de plus. La fiche complete vit dans le
// module Contacts.
type ContactRef struct {
	ID        uuid.UUID `json:"id"`
	Firstname string    `json:"firstname"`
	Lastname  string    `json:"lastname"`
	Role      string    `json:"role"`
	Email     *string   `json:"email"`
}

// CrmClientItem est une ligne du tableau « Clients », et une carte du kanban.
type CrmClientItem struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// Etape du pipeline commercial : lead, devis, actif, veille ou perdu.
	Status string `json:"status"`
	// Nul tant que le client n'a designe personne — un client tout juste
	// inscrit est dans ce cas.
	PrimaryContact *ContactRef `json:"primary_contact"`
	// Qui suit ce client dans l'agence. Nul quand personne n'est designe, ou
	// quand la personne a quitte l'agence.
	AccountManager *Person `json:"account_manager"`
	ContactsCount  int     `json:"contacts_count"`
	// Coordonnees de l'entreprise. Vides plutot que nulles : une adresse non
	// renseignee est un cas normal.
	Website    string `json:"website"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`
	Country    string `json:"country"`
	Siret      string `json:"siret"`
	VatNumber  string `json:"vat_number"`
	// Agregats lus en base, jamais recomptes ici : ce sont des colonnes tenues
	// par declencheur.
	ProjectsActive int       `json:"projects_active"`
	PortalUsers    int       `json:"portal_users"`
	CreatedAt      time.Time `json:"created_at"`
	// Entree dans l'etape courante, tenue par declencheur. C'est de quoi le
	// kanban tire l'anciennete d'une carte : l'ecart se calcule a l'affichage,
	// une duree renvoyee ici serait fausse des la seconde suivante.
	StatusChangedAt time.Time `json:"status_changed_at"`
}

// CrmClientPage est une page de la liste.
type CrmClientPage struct {
	Items    []CrmClientItem `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// CrmClientFilters porte ce que la barre d'outils peut demander.
type CrmClientFilters struct {
	Search *string
	// Etape du pipeline, et personne qui suit le client. Nuls quand le filtre
	// n'est pas pose.
	Status    *string
	ManagerID *uuid.UUID
	// HasPortal vaut nil quand le filtre n'est pas pose : true ne garde que les
	// clients qui ont au moins un compte, false que ceux qui n'en ont aucun.
	HasPortal *bool
	Sort      string
	Dir       string
	Page      int
	PageSize  int
}

// crmClientSorts clot les colonnes triables. La requete ouvre un ORDER BY
// parametre ; sans cette liste, l'ecran pourrait y faire passer n'importe quoi.
var crmClientSorts = map[string]struct{}{
	"name":     {},
	"projects": {},
	"created":  {},
}

// ClientService sert les ecrans du module CRM.
type ClientService struct {
	// Le pool sert aux creations, qui ecrivent le client et son premier
	// contact dans la meme transaction.
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewClientService(pool *pgxpool.Pool) *ClientService {
	return &ClientService{pool: pool, q: db.New(pool)}
}

// List renvoie une page du tableau des clients.
func (s *ClientService) List(ctx context.Context, f CrmClientFilters) (CrmClientPage, error) {
	if _, ok := crmClientSorts[f.Sort]; !ok {
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

	rows, err := s.q.ListCrmClients(ctx, db.ListCrmClientsParams{
		Search:     f.Search,
		Status:     f.Status,
		ManagerID:  f.ManagerID,
		HasPortal:  f.HasPortal,
		Sort:       f.Sort,
		Dir:        f.Dir,
		PageSize:   int32(f.PageSize),
		PageOffset: int32((f.Page - 1) * f.PageSize),
	})
	if err != nil {
		return CrmClientPage{}, fmt.Errorf("liste des clients : %w", err)
	}

	total, err := s.q.CountCrmClients(ctx, db.CountCrmClientsParams{
		Search:    f.Search,
		Status:    f.Status,
		ManagerID: f.ManagerID,
		HasPortal: f.HasPortal,
	})
	if err != nil {
		return CrmClientPage{}, fmt.Errorf("compte des clients : %w", err)
	}

	items := make([]CrmClientItem, 0, len(rows))
	for _, row := range rows {
		item := CrmClientItem{
			ID:              row.ID,
			Name:            row.Name,
			Status:          row.Status,
			ContactsCount:   int(row.ContactsCount),
			Website:         row.Website,
			Phone:           row.Phone,
			Address:         row.Address,
			PostalCode:      row.PostalCode,
			City:            row.City,
			Country:         row.Country,
			Siret:           row.Siret,
			VatNumber:       row.VatNumber,
			ProjectsActive:  int(row.ProjectsActive),
			PortalUsers:     int(row.PortalUsers),
			CreatedAt:       row.CreatedAt,
			StatusChangedAt: row.StatusChangedAt,
		}

		// Les jointures sont a gauche : sans contact principal ni chargé de
		// compte, leurs colonnes arrivent vides et il n'y a personne a nommer.
		if row.ContactID != nil {
			item.PrimaryContact = &ContactRef{
				ID:        *row.ContactID,
				Firstname: row.ContactFirstname,
				Lastname:  row.ContactLastname,
				Role:      row.ContactRole,
				Email:     row.ContactEmail,
			}
		}

		if row.AccountManagerID != nil && row.ManagerFirstname != nil {
			item.AccountManager = personOf(
				*row.AccountManagerID, *row.ManagerFirstname, *row.ManagerLastname, row.ManagerAvatarUrl,
			)
		}

		items = append(items, item)
	}

	return CrmClientPage{
		Items:    items,
		Total:    total,
		Page:     f.Page,
		PageSize: f.PageSize,
	}, nil
}

// CreateClientInput decrit un client a creer.
//
// Le contact est facultatif et se designe par identifiant, parmi les contacts
// libres : creer une personne se fait depuis l'ecran Contacts, pas ici. Un
// formulaire qui sait a la fois choisir et creer finit par faire les deux mal.
type CreateClientInput struct {
	Name      string
	ContactID *uuid.UUID
}

// Create inscrit un client, et adopte le contact libre qu'on lui designe.
func (s *ClientService) Create(ctx context.Context, in CreateClientInput) (CrmClientItem, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return CrmClientItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"name": "Le nom du client est requis",
		})
	}

	// Recherche prealable pour rendre un message utile plutot que de laisser
	// remonter une violation d'index. La verification ne suffit pas pour
	// autant : deux creations simultanees passeraient toutes deux ici, d'ou le
	// rattrapage sur l'erreur d'unicite plus bas.
	if _, err := s.q.GetClientByName(ctx, name); err == nil {
		return CrmClientItem{}, domain.ErrConflict.WithDetails(map[string]any{
			"name": "Un client porte déjà ce nom",
		})
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CrmClientItem{}, fmt.Errorf("recherche du client : %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CrmClientItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	client, err := qtx.CreateClient(ctx, name)
	if err != nil {
		if isUniqueViolation(err) {
			return CrmClientItem{}, domain.ErrConflict.WithDetails(map[string]any{
				"name": "Un client porte déjà ce nom",
			})
		}

		return CrmClientItem{}, fmt.Errorf("creation du client : %w", err)
	}

	item := itemOfClient(client)

	if in.ContactID != nil {
		// Le rattachement ne touche que les contacts libres : zero ligne
		// signifie que la personne a ete prise entre-temps, ou n'existe pas.
		contact, err := qtx.AttachContactToClient(ctx, db.AttachContactToClientParams{
			ClientID:  &client.ID,
			ContactID: *in.ContactID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return CrmClientItem{}, domain.ErrValidation.WithDetails(map[string]any{
					"contact_id": "Ce contact n'est plus disponible",
				})
			}

			return CrmClientItem{}, fmt.Errorf("rattachement du contact : %w", err)
		}

		// Rattache, il peut devenir principal : la cle etrangere composite
		// l'exigeait dans cet ordre.
		if err := qtx.SetPrimaryContact(ctx, db.SetPrimaryContactParams{
			ClientID:  client.ID,
			ContactID: &contact.ID,
		}); err != nil {
			return CrmClientItem{}, fmt.Errorf("designation du contact principal : %w", err)
		}

		item.ContactsCount = 1
		item.PrimaryContact = &ContactRef{
			ID:        contact.ID,
			Firstname: contact.Firstname,
			Lastname:  contact.Lastname,
			Role:      contact.Role,
			Email:     contact.Email,
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CrmClientItem{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return item, nil
}

// SetPrimaryContact designe l'interlocuteur principal d'un client. Un contact
// nul retire la designation sans supprimer personne.
func (s *ClientService) SetPrimaryContact(ctx context.Context, clientID uuid.UUID, contactID *uuid.UUID) error {
	// La requete ne touche aucune ligne quand le client est introuvable comme
	// quand le contact n'est pas le sien : les deux cas se disent au client de
	// la meme facon, une designation qu'on ne peut pas honorer.
	tag, err := s.pool.Exec(ctx, `
		UPDATE clients
		SET primary_contact_id = $2, updated_at = now()
		WHERE id = $1
		  AND deleted_at IS NULL
		  AND ($2::uuid IS NULL OR EXISTS (
		      SELECT 1 FROM contacts ct
		      WHERE ct.id = $2::uuid AND ct.client_id = $1 AND ct.deleted_at IS NULL
		  ))`, clientID, contactID)
	if err != nil {
		return fmt.Errorf("designation du contact principal : %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrValidation.WithDetails(map[string]any{
			"primary_contact_id": "Ce contact n'appartient pas à ce client",
		})
	}

	return nil
}

// ClientProject est une ligne de la liste des projets sur la fiche d'un client.
// Reduite a ce que la fiche montre : le tableau complet vit dans le module PM.
type ClientProject struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Progress int       `json:"progress"`
	DueOn    *string   `json:"due_on"`
}

// PortalAccount est un compte de portail rattache au client.
type PortalAccount struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Firstname   string     `json:"firstname"`
	Lastname    string     `json:"lastname"`
	AvatarURL   *string    `json:"avatar_url"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

// CrmClientDetail est tout ce que la fiche d'un client affiche.
//
// Un seul appel la remplit : ses contacts, ses projets et ses comptes de
// portail voyagent avec l'en-tete plutot que par trois requetes que l'ecran
// enchainerait.
type CrmClientDetail struct {
	CrmClientItem
	Contacts []CrmContactItem `json:"contacts"`
	Projects []ClientProject  `json:"projects"`
	Accounts []PortalAccount  `json:"accounts"`
}

// Combien de lignes une fiche montre au plus, par bloc. Au-dela, l'ecran du
// module concerne prend le relais avec sa pagination.
const clientDetailLimit = 50

// Get renvoie la fiche d'un client.
func (s *ClientService) Get(ctx context.Context, id uuid.UUID) (CrmClientDetail, error) {
	row, err := s.q.GetCrmClient(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CrmClientDetail{}, domain.ErrNotFound
		}

		return CrmClientDetail{}, fmt.Errorf("lecture du client : %w", err)
	}

	detail := CrmClientDetail{
		CrmClientItem: CrmClientItem{
			ID:              row.ID,
			Name:            row.Name,
			Status:          row.Status,
			ContactsCount:   int(row.ContactsCount),
			Website:         row.Website,
			Phone:           row.Phone,
			Address:         row.Address,
			PostalCode:      row.PostalCode,
			City:            row.City,
			Country:         row.Country,
			Siret:           row.Siret,
			VatNumber:       row.VatNumber,
			ProjectsActive:  int(row.ProjectsActive),
			PortalUsers:     int(row.PortalUsers),
			CreatedAt:       row.CreatedAt,
			StatusChangedAt: row.StatusChangedAt,
		},
	}

	if row.AccountManagerID != nil && row.ManagerFirstname != nil {
		detail.AccountManager = personOf(
			*row.AccountManagerID, *row.ManagerFirstname, *row.ManagerLastname, row.ManagerAvatarUrl,
		)
	}

	if row.ContactID != nil {
		detail.PrimaryContact = &ContactRef{
			ID:        *row.ContactID,
			Firstname: row.ContactFirstname,
			Lastname:  row.ContactLastname,
			Role:      row.ContactRole,
			Email:     row.ContactEmail,
		}
	}

	contacts, err := s.q.ListCrmContacts(ctx, db.ListCrmContactsParams{
		ClientID: &id,
		Sort:     "name",
		Dir:      "asc",
		PageSize: clientDetailLimit,
	})
	if err != nil {
		return CrmClientDetail{}, fmt.Errorf("contacts du client : %w", err)
	}

	detail.Contacts = make([]CrmContactItem, 0, len(contacts))
	for _, c := range contacts {
		detail.Contacts = append(detail.Contacts, CrmContactItem{
			ID:         c.ID,
			ClientID:   c.ClientID,
			ClientName: c.ClientName,
			Firstname:  c.Firstname,
			Lastname:   c.Lastname,
			Role:       c.Role,
			Email:      c.Email,
			Phone:      c.Phone,
			IsPrimary:  c.IsPrimary,
			CreatedAt:  c.CreatedAt,
		})
	}

	projects, err := s.q.ListProjectsOfClient(ctx, db.ListProjectsOfClientParams{
		ClientID: id,
		PageSize: clientDetailLimit,
	})
	if err != nil {
		return CrmClientDetail{}, fmt.Errorf("projets du client : %w", err)
	}

	detail.Projects = make([]ClientProject, 0, len(projects))
	for _, p := range projects {
		item := ClientProject{
			ID:       p.ID,
			Name:     p.Name,
			Status:   p.Status,
			Progress: int(p.Progress),
		}

		if p.DueOn != nil {
			due := p.DueOn.Format(dateLayout)
			item.DueOn = &due
		}

		detail.Projects = append(detail.Projects, item)
	}

	accounts, err := s.q.ListPortalUsersOfClient(ctx, db.ListPortalUsersOfClientParams{
		ClientID: &id,
		PageSize: clientDetailLimit,
	})
	if err != nil {
		return CrmClientDetail{}, fmt.Errorf("comptes du client : %w", err)
	}

	detail.Accounts = make([]PortalAccount, 0, len(accounts))
	for _, a := range accounts {
		detail.Accounts = append(detail.Accounts, PortalAccount{
			ID:          a.ID,
			Email:       a.Email,
			Firstname:   a.Firstname,
			Lastname:    a.Lastname,
			AvatarURL:   a.AvatarUrl,
			LastLoginAt: a.LastLoginAt,
		})
	}

	return detail, nil
}

// clientStatuses clot les etapes du pipeline. Meme liste que le CHECK en base :
// celui-ci refuserait de toute facon une valeur inconnue, mais avec une erreur
// de contrainte la ou l'ecran merite un message par champ.
var clientStatuses = map[string]struct{}{
	"lead":   {},
	"devis":  {},
	"actif":  {},
	"veille": {},
	"perdu":  {},
}

// UpdateClientInput decrit la fiche modifiable d'un client.
//
// Tous les champs voyagent ensemble : la requete les ecrit tous, et n'en
// envoyer qu'une partie effacerait le reste. Le contact principal et les
// compteurs n'y sont pas — ils ont leur propre route, ou leur declencheur.
type UpdateClientInput struct {
	Name             string
	Status           string
	AccountManagerID *uuid.UUID
	Website          string
	Phone            string
	Address          string
	PostalCode       string
	City             string
	Country          string
	Siret            string
	VatNumber        string
}

// Update modifie la fiche d'un client.
func (s *ClientService) Update(ctx context.Context, id uuid.UUID, in UpdateClientInput) (CrmClientItem, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return CrmClientItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"name": "Le nom du client est requis",
		})
	}

	if in.Status == "" {
		in.Status = "lead"
	}
	if _, ok := clientStatuses[in.Status]; !ok {
		return CrmClientItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"status": "Statut inconnu",
		})
	}

	// Un client peut garder son nom : c'est une modification qui ne change rien,
	// pas un conflit. Sans ce test, renommer « Novaterre » en « Novaterre »
	// echouerait contre son propre enregistrement.
	if existing, err := s.q.GetClientByName(ctx, name); err == nil && existing.ID != id {
		return CrmClientItem{}, domain.ErrConflict.WithDetails(map[string]any{
			"name": "Un client porte déjà ce nom",
		})
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CrmClientItem{}, fmt.Errorf("recherche du client : %w", err)
	}

	// Le chargé de compte est un membre de l'agence. La colonne accepterait un
	// compte client — aucune contrainte de base ne sait interroger une autre
	// table — donc la verification se fait ici.
	if in.AccountManagerID != nil {
		manager, err := s.q.GetUserByID(ctx, *in.AccountManagerID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return CrmClientItem{}, domain.ErrValidation.WithDetails(map[string]any{
					"account_manager_id": "Compte introuvable",
				})
			}

			return CrmClientItem{}, fmt.Errorf("lecture du chargé de compte : %w", err)
		}

		if manager.Role == "client" {
			return CrmClientItem{}, domain.ErrValidation.WithDetails(map[string]any{
				"account_manager_id": "Un compte client ne peut pas suivre un client",
			})
		}
	}

	row, err := s.q.UpdateClient(ctx, db.UpdateClientParams{
		ID:               id,
		Name:             name,
		Status:           in.Status,
		AccountManagerID: in.AccountManagerID,
		Website:          strings.TrimSpace(in.Website),
		Phone:            strings.TrimSpace(in.Phone),
		Address:          strings.TrimSpace(in.Address),
		PostalCode:       strings.TrimSpace(in.PostalCode),
		City:             strings.TrimSpace(in.City),
		Country:          strings.TrimSpace(in.Country),
		Siret:            strings.TrimSpace(in.Siret),
		VatNumber:        strings.TrimSpace(in.VatNumber),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CrmClientItem{}, domain.ErrNotFound
		}
		if isUniqueViolation(err) {
			return CrmClientItem{}, domain.ErrConflict.WithDetails(map[string]any{
				"name": "Un client porte déjà ce nom",
			})
		}

		return CrmClientItem{}, fmt.Errorf("modification du client : %w", err)
	}

	return itemOfClient(row), nil
}

// MoveStatus deplace une carte du kanban.
//
// Distincte d'Update : glisser une carte ne doit pas reecrire les coordonnees
// de l'entreprise avec ce que l'ecran avait en memoire.
func (s *ClientService) MoveStatus(ctx context.Context, id uuid.UUID, status string) (CrmClientItem, error) {
	if _, ok := clientStatuses[status]; !ok {
		return CrmClientItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"status": "Statut inconnu",
		})
	}

	row, err := s.q.MoveClientStatus(ctx, db.MoveClientStatusParams{ID: id, Status: status})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CrmClientItem{}, domain.ErrNotFound
		}

		return CrmClientItem{}, fmt.Errorf("deplacement du client : %w", err)
	}

	return itemOfClient(row), nil
}

// itemOfClient rend la ligne telle que les ecrans l'attendent, sans le contact
// principal ni le chargé de compte : une ecriture ne rend que ce qu'elle a
// touche, et l'ecran relit la liste.
func itemOfClient(row db.Client) CrmClientItem {
	return CrmClientItem{
		ID:              row.ID,
		Name:            row.Name,
		Status:          row.Status,
		Website:         row.Website,
		Phone:           row.Phone,
		Address:         row.Address,
		PostalCode:      row.PostalCode,
		City:            row.City,
		Country:         row.Country,
		Siret:           row.Siret,
		VatNumber:       row.VatNumber,
		ProjectsActive:  int(row.ProjectsActive),
		PortalUsers:     int(row.PortalUsers),
		CreatedAt:       row.CreatedAt,
		StatusChangedAt: row.StatusChangedAt,
	}
}

// Delete efface un client, si rien ne s'y rattache encore.
//
// Un client qui a des projets ne part pas : ses projets porteraient un
// commanditaire invisible, et la liste des projets les afficherait quand meme
// puisqu'elle ne filtre pas sur l'etat du client. Meme raisonnement pour les
// comptes de portail, qui ne sauraient plus quoi montrer.
//
// Les contacts, eux, suivent : ils n'existaient que pour ce client.
func (s *ClientService) Delete(ctx context.Context, id uuid.UUID) error {
	client, err := s.q.GetClientByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}

		return fmt.Errorf("lecture du client : %w", err)
	}

	projects, err := s.q.CountProjectsOfClient(ctx, id)
	if err != nil {
		return fmt.Errorf("projets du client : %w", err)
	}

	if projects > 0 {
		return domain.ErrConflict.WithDetails(map[string]any{
			"projects": fmt.Sprintf("Ce client porte %d projet(s) : supprimez-les d'abord", projects),
		})
	}

	if client.PortalUsers > 0 {
		return domain.ErrConflict.WithDetails(map[string]any{
			"portal_users": "Des comptes de portail sont rattachés à ce client",
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	if err := qtx.SoftDeleteContactsOfClient(ctx, &id); err != nil {
		return fmt.Errorf("suppression des contacts : %w", err)
	}

	if err := qtx.SoftDeleteClient(ctx, id); err != nil {
		return fmt.Errorf("suppression du client : %w", err)
	}

	return tx.Commit(ctx)
}

// personOf compose l'identite reduite d'un membre de l'agence, initiales
// comprises — les ecrans les affichent tous et les recalculeraient chacun a sa
// facon.
func personOf(id uuid.UUID, firstname, lastname string, avatar *string) *Person {
	initials := ""
	if r := []rune(firstname); len(r) > 0 {
		initials += strings.ToUpper(string(r[0]))
	}
	if r := []rune(lastname); len(r) > 0 {
		initials += strings.ToUpper(string(r[0]))
	}

	return &Person{
		ID:        id,
		Firstname: firstname,
		Lastname:  lastname,
		Initials:  initials,
		AvatarURL: avatar,
	}
}

// CrmClientBoard est le kanban commercial : toutes les cartes, et le drapeau
// qui dit si la borne a ete atteinte.
//
// Bornee et non paginee, comme le tableau des taches : un kanban se lit en
// entier ou pas du tout.
type CrmClientBoard struct {
	Items []CrmClientItem `json:"items"`
	// Truncated vaut vrai quand la borne a coupe : l'ecran previent alors que
	// toutes les cartes ne sont pas la, plutot que de mentir par omission.
	Truncated bool `json:"truncated"`
}

// boardLimit borne le kanban. Une agence qui depasse ce chiffre a besoin des
// filtres, pas d'un mur de cartes.
const boardLimit = 300

// Board renvoie toutes les cartes du kanban.
func (s *ClientService) Board(ctx context.Context, f CrmClientFilters) (CrmClientBoard, error) {
	rows, err := s.q.ListClientsBoard(ctx, db.ListClientsBoardParams{
		Search:    f.Search,
		ManagerID: f.ManagerID,
		PageSize:  boardLimit + 1,
	})
	if err != nil {
		return CrmClientBoard{}, fmt.Errorf("kanban des clients : %w", err)
	}

	board := CrmClientBoard{Truncated: len(rows) > boardLimit}
	if board.Truncated {
		rows = rows[:boardLimit]
	}

	board.Items = make([]CrmClientItem, 0, len(rows))
	for _, row := range rows {
		item := CrmClientItem{
			ID:              row.ID,
			Name:            row.Name,
			Status:          row.Status,
			ContactsCount:   int(row.ContactsCount),
			Website:         row.Website,
			Phone:           row.Phone,
			Address:         row.Address,
			PostalCode:      row.PostalCode,
			City:            row.City,
			Country:         row.Country,
			Siret:           row.Siret,
			VatNumber:       row.VatNumber,
			ProjectsActive:  int(row.ProjectsActive),
			PortalUsers:     int(row.PortalUsers),
			CreatedAt:       row.CreatedAt,
			StatusChangedAt: row.StatusChangedAt,
		}

		if row.ContactID != nil {
			item.PrimaryContact = &ContactRef{
				ID:        *row.ContactID,
				Firstname: row.ContactFirstname,
				Lastname:  row.ContactLastname,
				Role:      row.ContactRole,
				Email:     row.ContactEmail,
			}
		}

		if row.AccountManagerID != nil && row.ManagerFirstname != nil {
			item.AccountManager = personOf(
				*row.AccountManagerID, *row.ManagerFirstname, *row.ManagerLastname, row.ManagerAvatarUrl,
			)
		}

		board.Items = append(board.Items, item)
	}

	return board, nil
}
