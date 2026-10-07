package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// PortalProject est une carte de « Mes projets ».
type PortalProject struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	LogoURL     *string   `json:"logo_url"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Progress    int       `json:"progress"`
	StartsOn    *string   `json:"starts_on"`
	DueOn       *string   `json:"due_on"`
	// Livrables qui attendent la reponse du client.
	DeliverablesPending int `json:"deliverables_pending"`
	// Prochain jalon non atteint ; nul quand il n'y en a plus.
	NextMilestone  *PortalMilestoneRef `json:"next_milestone"`
	LastActivityAt time.Time           `json:"last_activity_at"`
}

// PortalMilestoneRef nomme un jalon et son echeance.
type PortalMilestoneRef struct {
	Title string  `json:"title"`
	DueOn *string `json:"due_on"`
}

// PortalProjectList est l'accueil du portail.
type PortalProjectList struct {
	Items []PortalProject `json:"items"`
}

// PortalDeliverable est un livrable tel que le client le voit dans un projet.
type PortalDeliverable struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	// en_attente, valide ou retours : un brouillon n'est jamais montre.
	Status      string     `json:"status"`
	Version     int        `json:"version"`
	URL         string     `json:"url"`
	FileID      *uuid.UUID `json:"file_id"`
	SubmittedAt time.Time  `json:"submitted_at"`
	DecidedAt   *time.Time `json:"decided_at"`
	Feedback    string     `json:"feedback"`
	Milestone   *string    `json:"milestone"`
}

// PortalFile est un fichier partage avec le client.
type PortalFile struct {
	ID          uuid.UUID `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
}

// PortalProjectDetail est la page d'un projet dans le portail.
type PortalProjectDetail struct {
	ID                  uuid.UUID           `json:"id"`
	Name                string              `json:"name"`
	LogoURL             *string             `json:"logo_url"`
	Description         string              `json:"description"`
	ClientName          string              `json:"client_name"`
	Status              string              `json:"status"`
	Progress            int                 `json:"progress"`
	StartsOn            *string             `json:"starts_on"`
	DueOn               *string             `json:"due_on"`
	DeliverablesPending int                 `json:"deliverables_pending"`
	Milestones          []Milestone         `json:"milestones"`
	Deliverables        []PortalDeliverable `json:"deliverables"`
	Files               []PortalFile        `json:"files"`
	// La derniere mise en ligne connue, nulle s'il n'y en a pas eu.
	LastDeployment *Deployment `json:"last_deployment"`
	// Ce qui vient : le prochain jalon, ce qui attend le client, ou en est
	// l'equipe.
	NextStep PortalNextStep `json:"next_step"`
	// Le charge de compte d'abord, puis l'equipe du projet.
	Team []PortalContact `json:"team"`
}

// PortalNextStep est l'encart « Prochaine etape » d'un projet.
type PortalNextStep struct {
	// Prochain jalon non atteint, nul quand tout est atteint ou rien pose.
	Milestone *Milestone `json:"milestone"`
	// Livrables qui attendent la reponse du client.
	DeliverablesPending int `json:"deliverables_pending"`
	// Ou en est l'equipe : taches terminees sur le total, compteurs tenus par
	// les declencheurs.
	TasksDone  int `json:"tasks_done"`
	TasksTotal int `json:"tasks_total"`
}

// PortalContact est un interlocuteur du client sur un projet.
type PortalContact struct {
	ID        uuid.UUID `json:"id"`
	Firstname string    `json:"firstname"`
	Lastname  string    `json:"lastname"`
	Initials  string    `json:"initials"`
	AvatarURL *string   `json:"avatar_url"`
	// Charge de compte du client, ou membre de l'equipe du projet.
	Role       string `json:"role"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	BookingURL string `json:"booking_url"`
}

// PortalVersion est une version dans le fil d'un livrable.
type PortalVersion struct {
	Numero      int        `json:"numero"`
	URL         string     `json:"url"`
	FileID      *uuid.UUID `json:"file_id"`
	SubmittedAt time.Time  `json:"submitted_at"`
	SubmittedBy string     `json:"submitted_by"`
	Decision    string     `json:"decision"`
	DecidedAt   *time.Time `json:"decided_at"`
	DecidedBy   string     `json:"decided_by"`
	Feedback    string     `json:"feedback"`
}

// PortalDeliverableDetail est la page d'un livrable dans le portail.
type PortalDeliverableDetail struct {
	ID          uuid.UUID      `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Project     DeliverableRef `json:"project"`
	Milestone   *string        `json:"milestone"`
	// Etat de la version courante.
	Status string `json:"status"`
	// Du plus recent au plus ancien : la version courante en tete.
	Versions []PortalVersion `json:"versions"`
}

// PortalService sert le portail client.
//
// Chaque methode recoit l'identifiant de l'appelant et rien d'autre ne decide
// de ce qu'il voit : les requetes partent de son compte et de son client. Un
// identifiant qui appartient a un autre client rend ErrNotFound, comme un
// identifiant qui n'existe pas — le portail ne dit pas ce qu'il cache.
type PortalService struct {
	q            *db.Queries
	files        storage.Store
	maxFile      int64
	deliverables *DeliverableService
	tickets      *TicketService
	// Cle des liens de reponse envoyes par e-mail ; vide, aucun lien n'est
	// accepte.
	linkKey []byte
}

// NewPortalService construit le service. Les livrables et les tickets passent
// par les services du back-office : une reponse du client suit les memes
// notifications, le meme journal et les memes e-mails que celle de l'agence.
func NewPortalService(
	pool *pgxpool.Pool,
	files storage.Store,
	maxFile int64,
	deliverables *DeliverableService,
	tickets *TicketService,
) *PortalService {
	return &PortalService{q: db.New(pool), files: files, maxFile: maxFile, deliverables: deliverables, tickets: tickets}
}

// Projects rend les projets du client de l'appelant.
func (s *PortalService) Projects(ctx context.Context, userID uuid.UUID) (PortalProjectList, error) {
	rows, err := s.q.PortalListProjects(ctx, userID)
	if err != nil {
		return PortalProjectList{}, fmt.Errorf("lecture des projets : %w", err)
	}

	items := make([]PortalProject, 0, len(rows))
	for _, row := range rows {
		var next *PortalMilestoneRef
		if row.NextMilestoneTitle != "" {
			next = &PortalMilestoneRef{Title: row.NextMilestoneTitle, DueOn: formatDate(row.NextMilestoneDueOn)}
		}

		items = append(items, PortalProject{
			ID:                  row.ID,
			Name:                row.Name,
			LogoURL:             projectLogoURL(row.LogoKey),
			Description:         row.Description,
			Status:              row.Status,
			Progress:            int(row.Progress),
			StartsOn:            formatDate(row.StartsOn),
			DueOn:               formatDate(row.DueOn),
			DeliverablesPending: int(row.DeliverablesPending),
			NextMilestone:       next,
			LastActivityAt:      row.LastActivityAt,
		})
	}

	return PortalProjectList{Items: items}, nil
}

// Project rend un projet du client de l'appelant, ses jalons, ses livrables
// soumis et ses fichiers partages.
func (s *PortalService) Project(ctx context.Context, userID, projectID uuid.UUID) (PortalProjectDetail, error) {
	row, err := s.q.PortalGetProject(ctx, db.PortalGetProjectParams{UserID: userID, ProjectID: projectID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PortalProjectDetail{}, domain.ErrNotFound
		}

		return PortalProjectDetail{}, fmt.Errorf("lecture du projet : %w", err)
	}

	// Le projet est celui de l'appelant : ce qui suit se lit par son
	// identifiant, deja verifie.
	milestones, err := s.q.ListProjectMilestones(ctx, projectID)
	if err != nil {
		return PortalProjectDetail{}, fmt.Errorf("lecture des jalons : %w", err)
	}
	milestoneDeliverables, err := s.q.ListMilestoneDeliverables(ctx, projectID)
	if err != nil {
		return PortalProjectDetail{}, fmt.Errorf("lecture des livrables des jalons : %w", err)
	}
	deliverables, err := s.q.PortalListDeliverables(ctx, projectID)
	if err != nil {
		return PortalProjectDetail{}, fmt.Errorf("lecture des livrables : %w", err)
	}
	files, err := s.q.PortalListSharedFiles(ctx, &projectID)
	if err != nil {
		return PortalProjectDetail{}, fmt.Errorf("lecture des fichiers : %w", err)
	}
	team, err := s.q.PortalListProjectTeam(ctx, projectID)
	if err != nil {
		return PortalProjectDetail{}, fmt.Errorf("lecture des interlocuteurs : %w", err)
	}

	today := agencyToday(time.Now())
	out := PortalProjectDetail{
		ID:                  row.ID,
		Name:                row.Name,
		LogoURL:             projectLogoURL(row.LogoKey),
		Description:         row.Description,
		ClientName:          row.ClientName,
		Status:              row.Status,
		Progress:            int(row.Progress),
		StartsOn:            formatDate(row.StartsOn),
		DueOn:               formatDate(row.DueOn),
		DeliverablesPending: int(row.DeliverablesPending),
		Milestones:          make([]Milestone, 0, len(milestones)),
		Deliverables:        make([]PortalDeliverable, 0, len(deliverables)),
		Files:               make([]PortalFile, 0, len(files)),
		NextStep: PortalNextStep{
			DeliverablesPending: int(row.DeliverablesPending),
			TasksDone:           int(row.TasksDone),
			TasksTotal:          int(row.TasksTotal),
		},
		Team: make([]PortalContact, 0, len(team)),
	}

	for _, m := range team {
		person := personOf(m.ID, m.Firstname, m.Lastname, m.AvatarUrl)
		role := "Équipe du projet"
		if m.IsAccountManager {
			role = "Chargé·e de compte"
		}
		out.Team = append(out.Team, PortalContact{
			ID: m.ID, Firstname: m.Firstname, Lastname: m.Lastname, Initials: person.Initials, AvatarURL: person.AvatarURL,
			Role: role, Email: m.Email, Phone: m.Phone, BookingURL: m.BookingUrl,
		})
	}

	index := make(map[uuid.UUID]int, len(milestones))
	for i, m := range milestones {
		out.Milestones = append(out.Milestones, milestoneOf(m, today))
		index[m.ID] = i
	}
	// Le prochain jalon : le premier non atteint dans l'ordre de la frise,
	// qu'il soit date ou non, en retard ou a venir.
	for i := range out.Milestones {
		if out.Milestones[i].State != "done" {
			next := out.Milestones[i]
			out.NextStep.Milestone = &next
			break
		}
	}
	// Sous chaque jalon, les seuls livrables soumis : un brouillon reste
	// interne, meme rattache.
	for _, d := range milestoneDeliverables {
		if d.MilestoneID == nil || d.Status == "brouillon" {
			continue
		}
		if i, ok := index[*d.MilestoneID]; ok {
			out.Milestones[i].Deliverables = append(out.Milestones[i].Deliverables, MilestoneDeliverable{
				ID: d.ID, Title: d.Title, Status: d.Status,
			})
		}
	}

	for _, d := range deliverables {
		out.Deliverables = append(out.Deliverables, PortalDeliverable{
			ID:          d.ID,
			Title:       d.Title,
			Description: d.Description,
			Status:      d.Status,
			Version:     int(d.VersionNumero),
			URL:         d.VersionUrl,
			FileID:      d.AttachmentID,
			SubmittedAt: d.SubmittedAt,
			DecidedAt:   d.DecidedAt,
			Feedback:    d.Feedback,
			Milestone:   d.MilestoneTitle,
		})
	}

	for _, f := range files {
		out.Files = append(out.Files, PortalFile{
			ID: f.ID, Filename: f.Filename, ContentType: f.ContentType, SizeBytes: f.SizeBytes, CreatedAt: f.CreatedAt,
		})
	}

	deployments, err := s.q.ListDeploymentsOfProject(ctx, db.ListDeploymentsOfProjectParams{ProjectID: projectID, MaxRows: 1})
	if err != nil {
		return PortalProjectDetail{}, fmt.Errorf("mises en ligne : %w", err)
	}
	if len(deployments) > 0 {
		d := deploymentOf(deployments[0])
		out.LastDeployment = &d
	}

	return out, nil
}

// Deliverable rend un livrable soumis d'un projet du client, et tout son fil.
func (s *PortalService) Deliverable(ctx context.Context, userID, deliverableID uuid.UUID) (PortalDeliverableDetail, error) {
	row, err := s.q.PortalGetDeliverable(ctx, db.PortalGetDeliverableParams{UserID: userID, DeliverableID: deliverableID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PortalDeliverableDetail{}, domain.ErrNotFound
		}

		return PortalDeliverableDetail{}, fmt.Errorf("lecture du livrable : %w", err)
	}

	versions, err := s.q.ListDeliverableVersions(ctx, deliverableID)
	if err != nil {
		return PortalDeliverableDetail{}, fmt.Errorf("lecture des versions : %w", err)
	}

	out := PortalDeliverableDetail{
		ID:          row.ID,
		Title:       row.Title,
		Description: row.Description,
		Project:     DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
		Milestone:   row.MilestoneTitle,
		Versions:    make([]PortalVersion, 0, len(versions)),
	}

	for i := len(versions) - 1; i >= 0; i-- {
		v := versions[i]
		if v.ID == *row.CurrentVersionID {
			out.Status = v.Decision
		}
		out.Versions = append(out.Versions, PortalVersion{
			Numero:      int(v.Numero),
			URL:         v.Url,
			FileID:      v.AttachmentID,
			SubmittedAt: v.SubmittedAt,
			SubmittedBy: firstnameOf(v.SubmitterFirstname),
			Decision:    v.Decision,
			DecidedAt:   v.DecidedAt,
			DecidedBy:   firstnameOf(v.DeciderFirstname),
			Feedback:    v.Feedback,
		})
	}

	return out, nil
}

// Decide enregistre la reponse du client sur la version courante.
//
// Des retours sans un mot ne servent a rien a l'equipe : le portail demande
// de dire ce qui doit changer.
func (s *PortalService) Decide(
	ctx context.Context,
	userID, deliverableID uuid.UUID,
	decision, feedback string,
) (PortalDeliverableDetail, error) {
	if _, err := s.q.PortalGetDeliverable(ctx, db.PortalGetDeliverableParams{
		UserID: userID, DeliverableID: deliverableID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PortalDeliverableDetail{}, domain.ErrNotFound
		}

		return PortalDeliverableDetail{}, fmt.Errorf("lecture du livrable : %w", err)
	}

	if decision == "retours" && strings.TrimSpace(feedback) == "" {
		return PortalDeliverableDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"feedback": "Dites-nous ce qui doit changer",
		})
	}

	if _, err := s.deliverables.Decide(ctx, deliverableID, decision, feedback, &userID); err != nil {
		return PortalDeliverableDetail{}, err
	}

	return s.Deliverable(ctx, userID, deliverableID)
}

// OpenFile ouvre un fichier que l'appelant peut telecharger.
func (s *PortalService) OpenFile(ctx context.Context, userID, fileID uuid.UUID) (Attachment, io.ReadCloser, error) {
	row, err := s.q.PortalGetFile(ctx, db.PortalGetFileParams{UserID: userID, FileID: fileID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Attachment{}, nil, domain.ErrNotFound
		}

		return Attachment{}, nil, fmt.Errorf("lecture du fichier : %w", err)
	}

	content, err := s.files.Open(row.StorageKey)
	if err != nil {
		return Attachment{}, nil, fmt.Errorf("ouverture du fichier : %w", err)
	}

	return Attachment{
		ID: row.ID, Filename: row.Filename, ContentType: row.ContentType, SizeBytes: row.SizeBytes, CreatedAt: row.CreatedAt,
	}, content, nil
}

func firstnameOf(name *string) string {
	if name == nil {
		return ""
	}

	return *name
}
