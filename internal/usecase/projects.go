package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/repository/db"
)

// dateLayout est le format des dates sans heure echangees avec le front.
//
// Les echeances sont des dates, pas des instants : les rendre en RFC3339
// obligerait chaque ecran a tronquer une heure qui ne veut rien dire, et ferait
// basculer une echeance d'un jour selon le fuseau du navigateur.
const dateLayout = "2006-01-02"

// Person est l'identite reduite affichee partout ou quelqu'un apparait : une
// pastille d'avatar, un nom dans une liste. Distincte de Profile, qui porte les
// permissions de l'appelant et n'a rien a faire dans une liste de projets.
type Person struct {
	ID        uuid.UUID `json:"id"`
	Firstname string    `json:"firstname"`
	Lastname  string    `json:"lastname"`
	// Initiales calculees cote serveur : les ecrans les affichent tous, et
	// chacun les recalculerait a sa facon.
	Initials  string  `json:"initials"`
	AvatarURL *string `json:"avatar_url"`
}

// ProjectListItem est une ligne de la liste des projets. Elle porte exactement
// ce que la ligne affiche, agregats compris — aucun ecran ne complete cette
// reponse par un second appel.
type ProjectListItem struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	ClientID   uuid.UUID `json:"client_id"`
	ClientName string    `json:"client_name"`
	Status     string    `json:"status"`
	Progress   int       `json:"progress"`
	HoursSold  float64   `json:"hours_sold"`
	HoursSpent float64   `json:"hours_spent"`
	StartsOn   *string   `json:"starts_on"`
	DueOn      *string   `json:"due_on"`
	TasksTotal int       `json:"tasks_total"`
	TasksDone  int       `json:"tasks_done"`
	Team       []Person  `json:"team"`
}

// ProjectPage est une page de la liste.
type ProjectPage struct {
	Items    []ProjectListItem `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// ProjectDetail est l'en-tete d'un projet : ce que le chassis affiche, quel que
// soit l'onglet ouvert.
type ProjectDetail struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	ClientID           uuid.UUID `json:"client_id"`
	ClientName         string    `json:"client_name"`
	ClientContactName  string    `json:"client_contact_name"`
	ClientContactRole  string    `json:"client_contact_role"`
	ClientContactEmail *string   `json:"client_contact_email"`
	Status             string    `json:"status"`
	Progress           int       `json:"progress"`
	HoursSold          float64   `json:"hours_sold"`
	HoursSpent         float64   `json:"hours_spent"`
	StartsOn           *string   `json:"starts_on"`
	DueOn              *string   `json:"due_on"`
	TasksTotal         int       `json:"tasks_total"`
	TasksDone          int       `json:"tasks_done"`
	Team               []Person  `json:"team"`
}

// ClientItem est une entree du champ « Client » du formulaire de projet.
type ClientItem struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	ContactName string    `json:"contact_name"`
	ContactRole string    `json:"contact_role"`
}

// ProjectFilters porte ce que la barre de filtres de la liste peut demander.
type ProjectFilters struct {
	Status   *string
	ClientID *uuid.UUID
	Search   *string
	Sort     string
	Dir      string
	Page     int
	PageSize int
}

// CreateProjectInput decrit un projet a creer.
//
// Le client est designe par son identifiant OU par son nom : le formulaire
// propose une liste, mais laisse saisir un nom inconnu plutot que d'imposer un
// detour par un ecran de creation de client qui n'existe pas encore.
type CreateProjectInput struct {
	Name       string
	ClientID   *uuid.UUID
	ClientName string
	Status     string
	Progress   int
	HoursSold  float64
	StartsOn   *time.Time
	DueOn      *time.Time
	TeamIDs    []uuid.UUID
	CreatedBy  uuid.UUID
}

// UpdateProjectInput ne porte que ce qui change : un champ absent garde sa
// valeur. Les pointeurs distinguent « non fourni » de « vide ».
type UpdateProjectInput struct {
	Name          *string
	ClientID      *uuid.UUID
	Status        *string
	Progress      *int
	HoursSold     *float64
	StartsOn      *time.Time
	ClearStartsOn bool
	DueOn         *time.Time
	ClearDueOn    bool
}

// Statuts acceptes, en un seul endroit. Le CHECK de la base dit la meme chose ;
// le refuser ici donne une erreur de validation lisible plutot qu'une violation
// de contrainte remontee en 500.
var projectStatuses = map[string]struct{}{
	"cadrage": {}, "production": {}, "attente": {}, "livre": {},
}

// Colonnes de tri autorisees. La requete construit son ORDER BY a partir de
// cette valeur : la clore ici est ce qui empeche l'ecran de demander n'importe
// quoi.
var projectSorts = map[string]struct{}{
	"due": {}, "name": {}, "progress": {}, "budget": {},
}

// ProjectService porte les projets et leurs clients.
type ProjectService struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewProjectService construit le service. Le pool sert aux creations, qui
// ecrivent le projet et son equipe dans la meme transaction.
func NewProjectService(pool *pgxpool.Pool) *ProjectService {
	return &ProjectService{pool: pool, q: db.New(pool)}
}

// List renvoie une page de la liste des projets.
func (s *ProjectService) List(ctx context.Context, f ProjectFilters) (ProjectPage, error) {
	if _, ok := projectSorts[f.Sort]; !ok {
		f.Sort = "due"
	}
	if f.Dir != "desc" {
		f.Dir = "asc"
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}

	total, err := s.q.CountProjects(ctx, db.CountProjectsParams{
		Status:   f.Status,
		ClientID: f.ClientID,
		Search:   f.Search,
	})
	if err != nil {
		return ProjectPage{}, fmt.Errorf("comptage des projets : %w", err)
	}

	rows, err := s.q.ListProjects(ctx, db.ListProjectsParams{
		Status:     f.Status,
		ClientID:   f.ClientID,
		Search:     f.Search,
		Sort:       f.Sort,
		Dir:        f.Dir,
		PageSize:   int32(f.PageSize),
		PageOffset: int32((f.Page - 1) * f.PageSize),
	})
	if err != nil {
		return ProjectPage{}, fmt.Errorf("lecture des projets : %w", err)
	}

	items := make([]ProjectListItem, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		items = append(items, ProjectListItem{
			ID:         row.ID,
			Name:       row.Name,
			ClientID:   row.ClientID,
			ClientName: row.ClientName,
			Status:     row.Status,
			Progress:   int(row.Progress),
			HoursSold:  row.HoursSold,
			HoursSpent: row.HoursSpent,
			StartsOn:   formatDate(row.StartsOn),
			DueOn:      formatDate(row.DueOn),
			TasksTotal: int(row.TasksTotal),
			TasksDone:  int(row.TasksDone),
			Team:       []Person{},
		})
	}

	// Une seule requete pour les equipes de toute la page, puis repartition en
	// memoire : c'est ce qui evite les vingt allers-retours d'un N+1.
	if len(ids) > 0 {
		members, err := s.q.ListMembersOfProjects(ctx, ids)
		if err != nil {
			return ProjectPage{}, fmt.Errorf("lecture des equipes : %w", err)
		}

		byProject := make(map[uuid.UUID][]Person, len(ids))
		for _, m := range members {
			byProject[m.ProjectID] = append(byProject[m.ProjectID], Person{
				ID:        m.ID,
				Firstname: m.Firstname,
				Lastname:  m.Lastname,
				Initials:  initialsOf(m.Firstname, m.Lastname),
				AvatarURL: m.AvatarUrl,
			})
		}

		for i := range items {
			if team, ok := byProject[items[i].ID]; ok {
				items[i].Team = team
			}
		}
	}

	return ProjectPage{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Get renvoie l'en-tete d'un projet.
func (s *ProjectService) Get(ctx context.Context, id uuid.UUID) (ProjectDetail, error) {
	row, err := s.q.GetProject(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectDetail{}, domain.ErrNotFound
		}
		return ProjectDetail{}, fmt.Errorf("lecture du projet : %w", err)
	}

	members, err := s.q.ListMembersOfProjects(ctx, []uuid.UUID{id})
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("lecture de l'equipe : %w", err)
	}

	team := make([]Person, 0, len(members))
	for _, m := range members {
		team = append(team, Person{
			ID:        m.ID,
			Firstname: m.Firstname,
			Lastname:  m.Lastname,
			Initials:  initialsOf(m.Firstname, m.Lastname),
			AvatarURL: m.AvatarUrl,
		})
	}

	return ProjectDetail{
		ID:                 row.ID,
		Name:               row.Name,
		ClientID:           row.ClientID,
		ClientName:         row.ClientName,
		ClientContactName:  row.ClientContactName,
		ClientContactRole:  row.ClientContactRole,
		ClientContactEmail: row.ClientContactEmail,
		Status:             row.Status,
		Progress:           int(row.Progress),
		HoursSold:          row.HoursSold,
		HoursSpent:         row.HoursSpent,
		StartsOn:           formatDate(row.StartsOn),
		DueOn:              formatDate(row.DueOn),
		TasksTotal:         int(row.TasksTotal),
		TasksDone:          int(row.TasksDone),
		Team:               team,
	}, nil
}

// Create cree un projet, son client si besoin, et son equipe.
//
// Le tout dans une transaction : un projet sans son equipe, ou un client cree
// pour un projet qui echoue ensuite, laisserait la base dans un etat que
// personne n'a demande.
func (s *ProjectService) Create(ctx context.Context, in CreateProjectInput) (ProjectDetail, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"name": "Le nom du projet est requis",
		})
	}

	if in.Status == "" {
		in.Status = "cadrage"
	}
	if _, ok := projectStatuses[in.Status]; !ok {
		return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"status": "Statut inconnu",
		})
	}
	if in.Progress < 0 || in.Progress > 100 {
		return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"progress": "L'avancement va de 0 à 100",
		})
	}
	if in.HoursSold < 0 {
		return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"hours_sold": "Les heures vendues ne peuvent pas être négatives",
		})
	}
	if in.StartsOn != nil && in.DueOn != nil && in.DueOn.Before(*in.StartsOn) {
		return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"due_on": "L'échéance précède la date de début",
		})
	}
	if in.ClientID == nil && strings.TrimSpace(in.ClientName) == "" {
		return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"client": "Un projet appartient à un client",
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	clientID, err := s.resolveClient(ctx, qtx, in.ClientID, in.ClientName)
	if err != nil {
		return ProjectDetail{}, err
	}

	project, err := qtx.CreateProject(ctx, db.CreateProjectParams{
		ClientID:  clientID,
		Name:      name,
		Status:    in.Status,
		Progress:  int16(in.Progress),
		HoursSold: in.HoursSold,
		StartsOn:  in.StartsOn,
		DueOn:     in.DueOn,
		CreatedBy: &in.CreatedBy,
	})
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("creation du projet : %w", err)
	}

	for _, member := range in.TeamIDs {
		if err := qtx.AddProjectMember(ctx, db.AddProjectMemberParams{
			ProjectID: project.ID,
			UserID:    member,
		}); err != nil {
			return ProjectDetail{}, fmt.Errorf("affectation de l'equipe : %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ProjectDetail{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return s.Get(ctx, project.ID)
}

// resolveClient retrouve le client designe, ou le cree a partir de son nom.
//
// La recherche par nom precede la creation : saisir « Novaterre » alors que le
// client existe deja doit le reutiliser, pas creer un doublon que l'index
// unique refuserait de toute facon.
func (s *ProjectService) resolveClient(ctx context.Context, q *db.Queries, id *uuid.UUID, name string) (uuid.UUID, error) {
	if id != nil {
		client, err := q.GetClientByID(ctx, *id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return uuid.Nil, domain.ErrValidation.WithDetails(map[string]any{
					"client": "Client introuvable",
				})
			}
			return uuid.Nil, fmt.Errorf("lecture du client : %w", err)
		}
		return client.ID, nil
	}

	trimmed := strings.TrimSpace(name)

	existing, err := q.GetClientByName(ctx, trimmed)
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("recherche du client : %w", err)
	}

	created, err := q.CreateClient(ctx, db.CreateClientParams{Name: trimmed})
	if err != nil {
		return uuid.Nil, fmt.Errorf("creation du client : %w", err)
	}

	return created.ID, nil
}

// Update modifie un projet.
func (s *ProjectService) Update(ctx context.Context, id uuid.UUID, in UpdateProjectInput) (ProjectDetail, error) {
	if in.Status != nil {
		if _, ok := projectStatuses[*in.Status]; !ok {
			return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"status": "Statut inconnu",
			})
		}
	}
	if in.Progress != nil && (*in.Progress < 0 || *in.Progress > 100) {
		return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"progress": "L'avancement va de 0 à 100",
		})
	}

	var progress *int16
	if in.Progress != nil {
		value := int16(*in.Progress)
		progress = &value
	}

	if _, err := s.q.UpdateProject(ctx, db.UpdateProjectParams{
		ID:            id,
		Name:          in.Name,
		Status:        in.Status,
		Progress:      progress,
		HoursSold:     in.HoursSold,
		ClientID:      in.ClientID,
		StartsOn:      in.StartsOn,
		ClearStartsOn: in.ClearStartsOn,
		DueOn:         in.DueOn,
		ClearDueOn:    in.ClearDueOn,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectDetail{}, domain.ErrNotFound
		}
		return ProjectDetail{}, fmt.Errorf("mise a jour du projet : %w", err)
	}

	return s.Get(ctx, id)
}

// Delete efface un projet logiquement. Ses taches restent en base, rattachees :
// restaurer le projet doit les retrouver.
func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.q.SoftDeleteProject(ctx, id); err != nil {
		return fmt.Errorf("suppression du projet : %w", err)
	}
	return nil
}

// SetTeam remplace l'equipe affectee.
func (s *ProjectService) SetTeam(ctx context.Context, id uuid.UUID, userIDs []uuid.UUID) (ProjectDetail, error) {
	current, err := s.q.ListMembersOfProjects(ctx, []uuid.UUID{id})
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("lecture de l'equipe : %w", err)
	}

	wanted := make(map[uuid.UUID]struct{}, len(userIDs))
	for _, userID := range userIDs {
		wanted[userID] = struct{}{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	for _, member := range current {
		if _, keep := wanted[member.ID]; !keep {
			if err := qtx.RemoveProjectMember(ctx, db.RemoveProjectMemberParams{
				ProjectID: id,
				UserID:    member.ID,
			}); err != nil {
				return ProjectDetail{}, fmt.Errorf("retrait d'un membre : %w", err)
			}
		}
	}

	for _, userID := range userIDs {
		if err := qtx.AddProjectMember(ctx, db.AddProjectMemberParams{
			ProjectID: id,
			UserID:    userID,
		}); err != nil {
			return ProjectDetail{}, fmt.Errorf("ajout d'un membre : %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ProjectDetail{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return s.Get(ctx, id)
}

// ListPeople liste les comptes internes.
//
// Sert les champs d'affectation : equipe d'un projet, assignes d'une tache.
// Les comptes clients en sont exclus — on n'assigne pas une tache de
// production a quelqu'un qui n'a acces qu'au portail.
func (s *ProjectService) ListPeople(ctx context.Context) ([]Person, error) {
	people := make([]Person, 0)

	for _, role := range []string{"admin", "team"} {
		rows, err := s.q.ListUsers(ctx, db.ListUsersParams{
			Role:       &role,
			PageSize:   100,
			PageOffset: 0,
		})
		if err != nil {
			return nil, fmt.Errorf("lecture des comptes : %w", err)
		}

		for _, row := range rows {
			people = append(people, Person{
				ID:        row.ID,
				Firstname: row.Firstname,
				Lastname:  row.Lastname,
				Initials:  initialsOf(row.Firstname, row.Lastname),
				AvatarURL: row.AvatarUrl,
			})
		}
	}

	return people, nil
}

// ListClients sert le champ « Client » du formulaire de projet.
func (s *ProjectService) ListClients(ctx context.Context, search *string, page, pageSize int) ([]ClientItem, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	rows, err := s.q.ListClients(ctx, db.ListClientsParams{
		Search:     search,
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("lecture des clients : %w", err)
	}

	items := make([]ClientItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ClientItem{
			ID:          row.ID,
			Name:        row.Name,
			ContactName: row.ContactName,
			ContactRole: row.ContactRole,
		})
	}

	return items, nil
}

// formatDate rend une date sans heure, ou nil.
func formatDate(value *time.Time) *string {
	if value == nil {
		return nil
	}

	formatted := value.Format(dateLayout)

	return &formatted
}

// initialsOf compose les initiales affichees dans les pastilles.
//
// Une seule lettre quand le nom manque : « M » vaut mieux qu'une pastille vide
// ou qu'un point d'interrogation.
func initialsOf(firstname, lastname string) string {
	var initials strings.Builder

	for _, part := range []string{firstname, lastname} {
		for _, r := range part {
			initials.WriteRune(r)
			break
		}
	}

	return strings.ToUpper(initials.String())
}

// isForeignKeyViolation reconnait une cle etrangere refusee par Postgres.
//
// C'est ce qui distingue « le projet que tu references n'existe pas » — une
// faute de la requete, donc un 422 — d'une panne de base, qui merite un 500.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// Metric est un chiffre du tableau de bord et son evolution.
//
// `Change` est nul quand la periode precedente etait vide : une progression
// depuis zero n'est pas « +100 % », c'est un demarrage, et l'ecran doit pouvoir
// dire qu'il n'y a rien a comparer plutot que d'afficher un pourcentage qui ne
// veut rien dire.
type Metric struct {
	Value  float64  `json:"value"`
	Change *float64 `json:"change"`
}

// DashboardSummary porte les chiffres d'en-tete du module.
//
// Pas de chiffre d'affaires : la facturation ne fait pas partie de ce projet.
// Les heures vendues sont la donnee la plus proche qui lui appartienne.
type DashboardSummary struct {
	Projects  Metric `json:"projects"`
	Clients   Metric `json:"clients"`
	HoursSold Metric `json:"hours_sold"`
}

// Dashboard renvoie les chiffres d'en-tete, en une requete.
func (s *ProjectService) Dashboard(ctx context.Context) (DashboardSummary, error) {
	row, err := s.q.GetDashboardStats(ctx)
	if err != nil {
		return DashboardSummary{}, fmt.Errorf("lecture des agregats : %w", err)
	}

	return DashboardSummary{
		Projects:  metricOf(float64(row.ProjectsTotal), float64(row.ProjectsRecent), float64(row.ProjectsPrevious)),
		Clients:   metricOf(float64(row.ClientsTotal), float64(row.ClientsRecent), float64(row.ClientsPrevious)),
		HoursSold: metricOf(row.HoursTotal, row.HoursRecent, row.HoursPrevious),
	}, nil
}

// metricOf compose un chiffre et sa variation d'une periode a l'autre.
func metricOf(total, recent, previous float64) Metric {
	if previous == 0 {
		return Metric{Value: total}
	}

	change := ((recent - previous) / previous) * 100

	return Metric{Value: total, Change: &change}
}
