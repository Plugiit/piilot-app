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

// Decisions qu'un client peut rendre, closes comme en base.
//
// « en_attente » n'en fait pas partie : c'est l'etat de depart d'une version,
// pas une reponse qu'on rend. Revenir en arriere sur une decision n'est pas
// prevu — la trace est ce que le module garde.
var deliverableDecisions = map[string]bool{"valide": true, "retours": true}

// DeliverableRef nomme un projet ou un client sans le decrire : la ligne
// affiche un lien, pas une fiche.
type DeliverableRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// DeliverableVersion est la version courante d'un livrable, telle que la ligne
// l'affiche.
type DeliverableVersion struct {
	ID     uuid.UUID `json:"id"`
	Numero int32     `json:"numero"`
	// Lien vers la ou le livrable vit : preproduction, maquette. Vide quand la
	// version porte un fichier depose.
	URL          string     `json:"url"`
	AttachmentID *uuid.UUID `json:"attachment_id"`
	// Date de soumission, que l'ecran compare a maintenant. Une duree renvoyee
	// ici serait fausse des la seconde suivante.
	SubmittedAt time.Time  `json:"submitted_at"`
	Submitter   *Person    `json:"submitter"`
	DecidedAt   *time.Time `json:"decided_at"`
	Feedback    string     `json:"feedback"`
}

// DeliverableItem est une ligne de l'ecran « Livrables ».
type DeliverableItem struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
	// brouillon, en_attente, valide ou retours. Aucune colonne ne le porte :
	// c'est la decision de la version courante, et l'absence de version vaut
	// brouillon.
	Status  string         `json:"status"`
	Project DeliverableRef `json:"project"`
	Client  DeliverableRef `json:"client"`
	// Nul tant qu'aucune version n'a ete soumise.
	Version   *DeliverableVersion `json:"version"`
	CreatedAt time.Time           `json:"created_at"`
}

// DeliverablePage est une page de l'ecran.
type DeliverablePage struct {
	Items    []DeliverableItem `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// DeliverableFilters porte ce que la barre d'outils peut demander.
type DeliverableFilters struct {
	Search    *string
	Status    *string
	ProjectID *uuid.UUID
}

// DeliverableEntry est une version dans le fil d'un livrable.
type DeliverableEntry struct {
	ID           uuid.UUID  `json:"id"`
	Numero       int32      `json:"numero"`
	URL          string     `json:"url"`
	AttachmentID *uuid.UUID `json:"attachment_id"`
	SubmittedAt  time.Time  `json:"submitted_at"`
	Submitter    *Person    `json:"submitter"`
	Decision     string     `json:"decision"`
	DecidedAt    *time.Time `json:"decided_at"`
	Decider      *Person    `json:"decider"`
	// admin, team ou client : dit si le client a tranche lui-meme, ou si
	// l'agence a enregistre une reponse recue ailleurs. Vide sans decision.
	DeciderRole string `json:"decider_role"`
	Feedback    string `json:"feedback"`
}

// DeliverableService sert l'ecran « Livrables ».
type DeliverableService struct {
	// Le pool sert aux transactions : numeroter une version et la designer
	// courante ne peuvent pas se faire en deux temps.
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewDeliverableService(pool *pgxpool.Pool) *DeliverableService {
	return &DeliverableService{pool: pool, q: db.New(pool)}
}

// personOfDeliverable compose une personne a partir des colonnes d'une jointure
// a gauche, nulles quand il n'y a personne a nommer.
func personOfDeliverable(id *uuid.UUID, firstname, lastname, avatar *string) *Person {
	if id == nil {
		return nil
	}

	person := Person{ID: *id, AvatarURL: avatar}
	if firstname != nil {
		person.Firstname = *firstname
	}
	if lastname != nil {
		person.Lastname = *lastname
	}
	person.Initials = initialsOf(person.Firstname, person.Lastname)

	return &person
}

// List renvoie une page de l'ecran « Livrables ».
func (s *DeliverableService) List(
	ctx context.Context,
	f DeliverableFilters,
	page, pageSize int,
) (DeliverablePage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	rows, err := s.q.ListDeliverables(ctx, db.ListDeliverablesParams{
		Search:     f.Search,
		Status:     f.Status,
		ProjectID:  f.ProjectID,
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return DeliverablePage{}, fmt.Errorf("liste des livrables : %w", err)
	}

	total, err := s.q.CountDeliverables(ctx, db.CountDeliverablesParams{
		Search:    f.Search,
		Status:    f.Status,
		ProjectID: f.ProjectID,
	})
	if err != nil {
		return DeliverablePage{}, fmt.Errorf("compte des livrables : %w", err)
	}

	items := make([]DeliverableItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, DeliverableItem{
			ID:        row.ID,
			Title:     row.Title,
			Status:    row.Status,
			Project:   DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
			Client:    DeliverableRef{ID: row.ClientID, Name: row.ClientName},
			CreatedAt: row.CreatedAt,
			Version: versionOf(
				row.VersionID, row.VersionNumero, row.VersionUrl, row.AttachmentID,
				row.SubmittedAt, row.DecidedAt, row.Feedback,
				row.SubmitterID, row.SubmitterFirstname, row.SubmitterLastname,
				row.SubmitterAvatarUrl,
			),
		})
	}

	return DeliverablePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// versionOf compose la version courante, nulle quand le livrable n'en a pas
// encore : toutes les colonnes de la jointure arrivent alors vides ensemble.
func versionOf(
	id *uuid.UUID,
	numero *int32,
	url *string,
	attachmentID *uuid.UUID,
	submittedAt *time.Time,
	decidedAt *time.Time,
	feedback *string,
	submitterID *uuid.UUID,
	submitterFirstname, submitterLastname, submitterAvatar *string,
) *DeliverableVersion {
	if id == nil || numero == nil || submittedAt == nil {
		return nil
	}

	version := DeliverableVersion{
		ID:           *id,
		Numero:       *numero,
		AttachmentID: attachmentID,
		SubmittedAt:  *submittedAt,
		DecidedAt:    decidedAt,
		Submitter: personOfDeliverable(
			submitterID, submitterFirstname, submitterLastname, submitterAvatar,
		),
	}
	if url != nil {
		version.URL = *url
	}
	if feedback != nil {
		version.Feedback = *feedback
	}

	return &version
}

// CreateDeliverableInput decrit un livrable a deposer.
//
// Le livrable et sa premiere version arrivent ensemble : un livrable sans rien
// a montrer n'est pas quelque chose qu'on depose, c'est une ligne vide.
type CreateDeliverableInput struct {
	ProjectID   uuid.UUID
	Title       string
	Description string
	URL         string
	CreatedBy   *uuid.UUID
}

// Create depose un livrable et sa premiere version.
func (s *DeliverableService) Create(
	ctx context.Context,
	in CreateDeliverableInput,
) (DeliverableItem, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return DeliverableItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"title": "Le titre est requis",
		})
	}

	url := strings.TrimSpace(in.URL)
	if url == "" {
		return DeliverableItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"url": "Le lien est requis",
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeliverableItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	id, err := q.CreateDeliverable(ctx, db.CreateDeliverableParams{
		ProjectID:   in.ProjectID,
		Title:       title,
		Description: strings.TrimSpace(in.Description),
		CreatedBy:   in.CreatedBy,
	})
	if err != nil {
		// Un projet qui n'existe pas est une faute de la requete, pas une
		// panne : la cle etrangere le dit, on le rend en 422.
		if isForeignKeyViolation(err) {
			return DeliverableItem{}, domain.ErrValidation.WithDetails(map[string]any{
				"project_id": "Ce projet n'existe pas",
			})
		}

		return DeliverableItem{}, fmt.Errorf("creation du livrable : %w", err)
	}

	if err := submitVersion(ctx, q, id, url, in.CreatedBy); err != nil {
		return DeliverableItem{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return DeliverableItem{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return s.Get(ctx, id)
}

// Submit ajoute une version a un livrable existant.
func (s *DeliverableService) Submit(
	ctx context.Context,
	deliverableID uuid.UUID,
	url string,
	submittedBy *uuid.UUID,
) (DeliverableItem, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return DeliverableItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"url": "Le lien est requis",
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeliverableItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	if _, err := q.LockDeliverable(ctx, deliverableID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DeliverableItem{}, domain.ErrNotFound
		}

		return DeliverableItem{}, fmt.Errorf("verrou du livrable : %w", err)
	}

	if err := submitVersion(ctx, q, deliverableID, url, submittedBy); err != nil {
		return DeliverableItem{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return DeliverableItem{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return s.Get(ctx, deliverableID)
}

// submitVersion numerote, insere, puis designe la version courante.
//
// Les trois gestes vont ensemble : une version inseree sans etre designee
// n'apparaitrait nulle part, et un numero pris hors du verrou pourrait etre
// attribue deux fois.
func submitVersion(
	ctx context.Context,
	q *db.Queries,
	deliverableID uuid.UUID,
	url string,
	submittedBy *uuid.UUID,
) error {
	numero, err := q.NextDeliverableVersionNumero(ctx, deliverableID)
	if err != nil {
		return fmt.Errorf("numero de version : %w", err)
	}

	versionID, err := q.CreateDeliverableVersion(ctx, db.CreateDeliverableVersionParams{
		DeliverableID: deliverableID,
		Numero:        numero,
		Url:           url,
		SubmittedBy:   submittedBy,
	})
	if err != nil {
		return fmt.Errorf("soumission de la version : %w", err)
	}

	err = q.SetDeliverableCurrentVersion(ctx, db.SetDeliverableCurrentVersionParams{
		ID:        deliverableID,
		VersionID: &versionID,
	})
	if err != nil {
		return fmt.Errorf("version courante : %w", err)
	}

	return nil
}

// Decide enregistre la reponse du client sur la version courante.
//
// Sur la version courante et non sur une version choisie par l'appelant : on
// repond a ce qui est sur la table, et laisser designer une version ancienne
// permettrait de reecrire une decision rendue.
func (s *DeliverableService) Decide(
	ctx context.Context,
	deliverableID uuid.UUID,
	decision, feedback string,
	decidedBy *uuid.UUID,
) (DeliverableItem, error) {
	if !deliverableDecisions[decision] {
		return DeliverableItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"decision": "Decision inconnue : valide ou retours",
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeliverableItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	locked, err := q.LockDeliverable(ctx, deliverableID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DeliverableItem{}, domain.ErrNotFound
		}

		return DeliverableItem{}, fmt.Errorf("verrou du livrable : %w", err)
	}

	// Rien a trancher tant que rien n'a ete soumis.
	if locked.CurrentVersionID == nil {
		return DeliverableItem{}, domain.ErrValidation.WithDetails(map[string]any{
			"decision": "Ce livrable n'a pas de version soumise",
		})
	}

	err = q.DecideDeliverableVersion(ctx, db.DecideDeliverableVersionParams{
		ID:        *locked.CurrentVersionID,
		Decision:  decision,
		DecidedBy: decidedBy,
		Feedback:  strings.TrimSpace(feedback),
	})
	if err != nil {
		return DeliverableItem{}, fmt.Errorf("decision : %w", err)
	}

	if err := q.TouchDeliverable(ctx, deliverableID); err != nil {
		return DeliverableItem{}, fmt.Errorf("mise a jour du livrable : %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return DeliverableItem{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return s.Get(ctx, deliverableID)
}

// Get renvoie un livrable et sa version courante.
func (s *DeliverableService) Get(
	ctx context.Context,
	id uuid.UUID,
) (DeliverableItem, error) {
	row, err := s.q.GetDeliverable(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DeliverableItem{}, domain.ErrNotFound
		}

		return DeliverableItem{}, fmt.Errorf("lecture du livrable : %w", err)
	}

	return DeliverableItem{
		ID:        row.ID,
		Title:     row.Title,
		Status:    row.Status,
		Project:   DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
		Client:    DeliverableRef{ID: row.ClientID, Name: row.ClientName},
		CreatedAt: row.CreatedAt,
		Version: versionOf(
			row.VersionID, row.VersionNumero, row.VersionUrl, row.AttachmentID,
			row.SubmittedAt, row.DecidedAt, row.Feedback,
			row.SubmitterID, row.SubmitterFirstname, row.SubmitterLastname,
			row.SubmitterAvatarUrl,
		),
	}, nil
}

// Versions renvoie le fil complet d'un livrable.
func (s *DeliverableService) Versions(
	ctx context.Context,
	deliverableID uuid.UUID,
) ([]DeliverableEntry, error) {
	rows, err := s.q.ListDeliverableVersions(ctx, deliverableID)
	if err != nil {
		return nil, fmt.Errorf("versions du livrable : %w", err)
	}

	entries := make([]DeliverableEntry, 0, len(rows))
	for _, row := range rows {
		entry := DeliverableEntry{
			ID:           row.ID,
			Numero:       row.Numero,
			URL:          row.Url,
			AttachmentID: row.AttachmentID,
			SubmittedAt:  row.SubmittedAt,
			Decision:     row.Decision,
			DecidedAt:    row.DecidedAt,
			Feedback:     row.Feedback,
			Submitter: personOfDeliverable(
				row.SubmitterID, row.SubmitterFirstname, row.SubmitterLastname,
				row.SubmitterAvatarUrl,
			),
			Decider: personOfDeliverable(
				row.DeciderID, row.DeciderFirstname, row.DeciderLastname,
				row.DeciderAvatarUrl,
			),
		}
		if row.DeciderRole != nil {
			entry.DeciderRole = *row.DeciderRole
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
