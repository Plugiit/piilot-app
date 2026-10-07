package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	mailer "github.com/plugiit/piilot-app/api/internal/mail"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Nomenclatures des tickets, closes comme en base.
//
// Le CHECK de la table refuserait deja une valeur inconnue, mais avec une
// erreur de contrainte que l'appelant lirait en 500. Les valider ici rend un
// 422 qui nomme le champ fautif.
var (
	ticketTrackers   = map[string]struct{}{"anomalie": {}, "evolution": {}, "assistance": {}}
	ticketPriorities = map[string]struct{}{
		"low": {}, "normal": {}, "high": {}, "urgent": {}, "critical": {},
	}
	ticketStatuses = map[string]struct{}{
		"backlog": {}, "todo": {}, "in_progress": {}, "in_review": {},
		"ready_to_deploy": {}, "done": {}, "annule": {},
	}
)

// TicketProject nomme le projet d'un ticket, sans le decrire.
//
// L'ecran affiche un lien, pas une fiche : l'identifiant et le nom suffisent,
// et renvoyer le projet entier gonflerait la reponse d'autant de fois qu'il y a
// de lignes.
type TicketProject struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// TicketItem est une ligne du tableau « Tickets ».
//
// Les cles de `tracker`, `status` et `priority` sont celles de la base. Leurs
// libelles francais vivent dans le front : traduire ici figerait l'intitule
// dans l'API, et le reformuler demanderait une version d'endpoint.
type TicketItem struct {
	ID uuid.UUID `json:"id"`
	// Numero lisible, unique pour toute l'agence. C'est ce qu'on echange a
	// l'oral : « regarde le #47 ».
	Numero  int64         `json:"numero"`
	Project TicketProject `json:"project"`
	// Assignee est nul quand le ticket est a prendre — un etat normal — et
	// quand le compte a quitte l'agence : le ticket reste, son destinataire
	// non. Le tableau personnel le renvoie aussi, ou il vaut toujours soi :
	// un type unique pour les quatre vues vaut mieux que deux qui divergent.
	Assignee *TicketPerson `json:"assignee"`
	// anomalie, evolution ou assistance.
	Tracker string `json:"tracker"`
	// backlog, todo, in_progress, in_review, ready_to_deploy, done ou annule.
	Status string `json:"status"`
	// low, normal, high, urgent ou critical.
	Priority  string    `json:"priority"`
	Subject   string    `json:"subject"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TicketPage est une page du tableau.
type TicketPage struct {
	Items    []TicketItem `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

// TicketFilters porte ce que la barre d'outils peut demander.
//
// Les trois vues la partagent : changer d'onglet ne remet pas les filtres a
// zero, on regarde le meme jeu de tickets autrement.
type TicketFilters struct {
	// Porte sur le sujet et sur le numero : « 47 » retrouve le ticket #47.
	Search    *string
	Status    *string
	Tracker   *string
	Priority  *string
	ProjectID *uuid.UUID
}

// TicketService sert l'ecran « Tickets ».
type TicketService struct {
	// Le pool sert a inscrire au registre : le message et le changement de
	// statut qu'il annonce vont dans la meme transaction.
	pool *pgxpool.Pool
	q    *db.Queries
	bus  Bus

	// E-mails du portail : au client quand l'agence repond, a l'agence quand
	// le client ecrit.
	baseURL     string
	teamURL     string
	adminURL    string
	mailEnabled bool
}

func NewTicketService(pool *pgxpool.Pool, bus Bus) *TicketService {
	return &TicketService{pool: pool, q: db.New(pool), bus: bus}
}

// SetMail active les e-mails des tickets du portail.
//
// Deux adresses : le portail pour les e-mails au client, le back-office pour
// ceux de l'agence. Elles different quand chaque espace a son domaine.
//
// Les administrateurs font tout leur travail sur le domaine d'administration :
// leurs liens y pointent.
func (s *TicketService) SetMail(clientURL, teamURL, adminURL string, enabled bool) {
	s.baseURL = strings.TrimRight(clientURL, "/")
	s.teamURL = strings.TrimRight(teamURL, "/")
	s.adminURL = strings.TrimRight(adminURL, "/")
	s.mailEnabled = enabled
}

// PortalTicketStatus est le statut d'un ticket dans les mots du client. Les
// etapes internes de l'agence s'y regroupent : « a faire » et « en attente »
// disent la meme chose a qui attend une reponse.
func PortalTicketStatus(status string) string {
	switch status {
	case "backlog", "todo":
		return "Reçue"
	case "in_progress", "in_review":
		return "En cours de traitement"
	case "ready_to_deploy":
		return "Prête à être mise en ligne"
	case "done":
		return "Résolue"
	case "annule":
		return "Fermée"
	default:
		return status
	}
}

// mailClient previent la personne du portail qui a ouvert le ticket : une
// reponse publique, un changement de statut visible, ou les deux.
func (s *TicketService) mailClient(ctx context.Context, q *db.Queries, ticketID uuid.UUID, quote, status string) error {
	if !s.mailEnabled || (quote == "" && status == "") {
		return nil
	}

	info, err := q.GetTicketMailContext(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("lecture du ticket : %w", err)
	}
	if !info.ClientVisible || !info.ReporterIsActiveClient || info.ReporterEmail == nil {
		return nil
	}

	msg, err := mailer.TicketToClient(
		*info.ReporterEmail, firstnameOf(info.ReporterFirstname), info.Numero, info.Subject,
		excerpt(quote), status, fmt.Sprintf("%s/client/tickets/%s", s.baseURL, ticketID),
	)
	if err != nil {
		return err
	}

	return enqueue(ctx, q, msg)
}

// mailTeam previent l'agence d'une demande deposee dans le portail, ou de la
// reponse d'un client.
func (s *TicketService) mailTeam(ctx context.Context, q *db.Queries, ticketID uuid.UUID, created bool, author, quote string) error {
	if !s.mailEnabled {
		return nil
	}

	info, err := q.GetTicketMailContext(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("lecture du ticket : %w", err)
	}
	recipients, err := q.ListTicketTeamMailRecipients(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("destinataires de l'agence : %w", err)
	}

	for _, r := range recipients {
		base := s.teamURL
		if r.Role == "admin" {
			base = s.adminURL
		}
		url := fmt.Sprintf("%s/pm/tickets/%s", base, ticketID)
		msg, err := mailer.TicketToTeam(
			r.Email, r.Firstname, created, info.Numero, info.Subject, info.ProjectName, info.ClientName,
			author, excerpt(quote), url,
		)
		if err != nil {
			return err
		}
		if err := enqueue(ctx, q, msg); err != nil {
			return err
		}
	}

	return nil
}

// ListAssignedTo renvoie une page des tickets confies a quelqu'un.
//
// L'identifiant vient de la session et jamais de la requete : un ecran qui
// accepterait un `assignee_id` en parametre laisserait lire les tickets de
// n'importe qui.
func (s *TicketService) ListAssignedTo(
	ctx context.Context,
	userID uuid.UUID,
	f TicketFilters,
	page, pageSize int,
) (TicketPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	rows, err := s.q.ListTicketsAssignedTo(ctx, db.ListTicketsAssignedToParams{
		AssigneeID: &userID,
		Search:     f.Search,
		Status:     f.Status,
		Tracker:    f.Tracker,
		Priority:   f.Priority,
		ProjectID:  f.ProjectID,
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return TicketPage{}, fmt.Errorf("liste des tickets : %w", err)
	}

	total, err := s.q.CountTicketsAssignedTo(ctx, db.CountTicketsAssignedToParams{
		AssigneeID: &userID,
		Search:     f.Search,
		Status:     f.Status,
		Tracker:    f.Tracker,
		Priority:   f.Priority,
		ProjectID:  f.ProjectID,
	})
	if err != nil {
		return TicketPage{}, fmt.Errorf("compte des tickets : %w", err)
	}

	items := make([]TicketItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, TicketItem{
			ID:        row.ID,
			Numero:    row.Numero,
			Project:   TicketProject{ID: row.ProjectID, Name: row.ProjectName},
			Tracker:   row.Tracker,
			Status:    row.Status,
			Priority:  row.Priority,
			Subject:   row.Subject,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			Assignee: personOfTicket(
				row.AssigneeID, row.AssigneeFirstname, row.AssigneeLastname,
				row.AssigneeAvatarUrl, "",
			),
		})
	}

	return TicketPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// TicketBoard est le kanban des tickets : toutes les cartes, et le drapeau qui
// dit si la borne a coupe.
//
// Bornee et non paginee, comme les kanbans des taches et des clients.
type TicketBoard struct {
	Items []TicketItem `json:"items"`
	// Truncated vaut vrai quand la borne a coupe : l'ecran previent alors que
	// toutes les cartes ne sont pas la, plutot que de mentir par omission.
	Truncated bool `json:"truncated"`
}

// ticketBoardLimit borne le kanban. Au-dela, c'est la vue tableau et ses
// filtres qu'il faut, pas un mur de cartes.
const ticketBoardLimit = 300

// BoardAssignedTo renvoie toutes les cartes du kanban d'une personne.
func (s *TicketService) BoardAssignedTo(
	ctx context.Context,
	userID uuid.UUID,
	f TicketFilters,
) (TicketBoard, error) {
	rows, err := s.q.ListTicketsBoardAssignedTo(ctx, db.ListTicketsBoardAssignedToParams{
		AssigneeID: &userID,
		Search:     f.Search,
		Status:     f.Status,
		Tracker:    f.Tracker,
		Priority:   f.Priority,
		ProjectID:  f.ProjectID,
		PageSize:   ticketBoardLimit + 1,
	})
	if err != nil {
		return TicketBoard{}, fmt.Errorf("kanban des tickets : %w", err)
	}

	board := TicketBoard{Truncated: len(rows) > ticketBoardLimit}
	if board.Truncated {
		rows = rows[:ticketBoardLimit]
	}

	board.Items = make([]TicketItem, 0, len(rows))
	for _, row := range rows {
		board.Items = append(board.Items, TicketItem{
			ID:        row.ID,
			Numero:    row.Numero,
			Project:   TicketProject{ID: row.ProjectID, Name: row.ProjectName},
			Tracker:   row.Tracker,
			Status:    row.Status,
			Priority:  row.Priority,
			Subject:   row.Subject,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			Assignee: personOfTicket(
				row.AssigneeID, row.AssigneeFirstname, row.AssigneeLastname,
				row.AssigneeAvatarUrl, "",
			),
		})
	}

	return board, nil
}

// ListByProject renvoie une page des tickets d'un projet, pour l'onglet
// « Tickets » de sa fiche.
//
// Le projet est un argument et non un filtre : on lit la fiche d'un projet, pas
// une liste qu'on restreindrait ensuite. `f.ProjectID` n'est donc pas lu.
//
// Aucune verification d'existence du projet : un identifiant inconnu rend une
// page vide plutot qu'un 404, et la fiche qui porte cet onglet a deja repondu
// 404 avant de l'afficher. La verifier ici couterait un SELECT par appel pour
// un cas qu'aucun ecran n'atteint.
func (s *TicketService) ListByProject(
	ctx context.Context,
	projectID uuid.UUID,
	f TicketFilters,
	page, pageSize int,
) (TicketPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	rows, err := s.q.ListTicketsByProject(ctx, db.ListTicketsByProjectParams{
		ProjectID:  projectID,
		Search:     f.Search,
		Status:     f.Status,
		Tracker:    f.Tracker,
		Priority:   f.Priority,
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return TicketPage{}, fmt.Errorf("liste des tickets du projet : %w", err)
	}

	total, err := s.q.CountTicketsByProject(ctx, db.CountTicketsByProjectParams{
		ProjectID: projectID,
		Search:    f.Search,
		Status:    f.Status,
		Tracker:   f.Tracker,
		Priority:  f.Priority,
	})
	if err != nil {
		return TicketPage{}, fmt.Errorf("compte des tickets du projet : %w", err)
	}

	items := make([]TicketItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, TicketItem{
			ID:        row.ID,
			Numero:    row.Numero,
			Project:   TicketProject{ID: row.ProjectID, Name: row.ProjectName},
			Tracker:   row.Tracker,
			Status:    row.Status,
			Priority:  row.Priority,
			Subject:   row.Subject,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			Assignee: personOfTicket(
				row.AssigneeID, row.AssigneeFirstname, row.AssigneeLastname,
				row.AssigneeAvatarUrl, "",
			),
		})
	}

	return TicketPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// BoardByProject renvoie toutes les cartes du kanban d'un projet.
//
// Le front les repartit par statut. Pas de regroupement par projet ici, comme
// sur l'ecran « Tickets » : dans un projet, il ne ferait qu'une colonne.
func (s *TicketService) BoardByProject(
	ctx context.Context,
	projectID uuid.UUID,
	f TicketFilters,
) (TicketBoard, error) {
	rows, err := s.q.ListTicketsBoardByProject(ctx, db.ListTicketsBoardByProjectParams{
		ProjectID: projectID,
		Search:    f.Search,
		Status:    f.Status,
		Tracker:   f.Tracker,
		Priority:  f.Priority,
		PageSize:  ticketBoardLimit + 1,
	})
	if err != nil {
		return TicketBoard{}, fmt.Errorf("kanban des tickets du projet : %w", err)
	}

	board := TicketBoard{Truncated: len(rows) > ticketBoardLimit}
	if board.Truncated {
		rows = rows[:ticketBoardLimit]
	}

	board.Items = make([]TicketItem, 0, len(rows))
	for _, row := range rows {
		board.Items = append(board.Items, TicketItem{
			ID:        row.ID,
			Numero:    row.Numero,
			Project:   TicketProject{ID: row.ProjectID, Name: row.ProjectName},
			Tracker:   row.Tracker,
			Status:    row.Status,
			Priority:  row.Priority,
			Subject:   row.Subject,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			Assignee: personOfTicket(
				row.AssigneeID, row.AssigneeFirstname, row.AssigneeLastname,
				row.AssigneeAvatarUrl, "",
			),
		})
	}

	return board, nil
}

// CreateTicketInput decrit un ticket a deposer.
//
// Le statut n'y figure pas : un ticket nait dans le backlog, et laisser
// l'ecran le choisir ferait deposer des demandes deja « terminees ».
type CreateTicketInput struct {
	ProjectID   uuid.UUID
	Subject     string
	Description string
	Tracker     string
	Priority    string
	// A qui le confier. Nul pour le laisser a prendre.
	AssigneeID *uuid.UUID
	// Qui depose. Vient de la session.
	CreatedBy *uuid.UUID
	// Depose depuis le portail : le ticket est visible du client, et l'agence
	// en est prevenue par e-mail. AuthorName nomme la personne dans l'e-mail.
	FromPortal bool
	AuthorName string
}

// Create depose un ticket et le rend sous la forme qu'affiche la liste.
func (s *TicketService) Create(ctx context.Context, in CreateTicketInput) (TicketItem, error) {
	subject := strings.TrimSpace(in.Subject)

	details := map[string]any{}

	if subject == "" {
		details["subject"] = "Le sujet est requis"
	}
	if in.ProjectID == uuid.Nil {
		details["project_id"] = "Le projet est requis"
	}
	if _, ok := ticketTrackers[in.Tracker]; !ok {
		details["tracker"] = "Tracker inconnu"
	}
	if _, ok := ticketPriorities[in.Priority]; !ok {
		details["priority"] = "Priorité inconnue"
	}

	if len(details) > 0 {
		return TicketItem{}, domain.ErrValidation.WithDetails(details)
	}

	// Une transaction pour que le ticket et ses notifications aboutissent
	// ensemble : un ticket depose sans que personne n'en soit prevenu est
	// precisement ce que les notifications doivent eviter.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TicketItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	row, err := q.CreateTicket(ctx, db.CreateTicketParams{
		ProjectID:     in.ProjectID,
		Subject:       subject,
		Description:   strings.TrimSpace(in.Description),
		Tracker:       in.Tracker,
		Status:        "backlog",
		Priority:      in.Priority,
		AssigneeID:    in.AssigneeID,
		CreatedBy:     in.CreatedBy,
		ClientVisible: in.FromPortal,
	})
	if err != nil {
		return TicketItem{}, fmt.Errorf("creation du ticket : %w", err)
	}

	err = s.notifyTicket(ctx, q, ticketNotice{
		TicketID:    row.ID,
		ProjectID:   row.ProjectID,
		Actor:       in.CreatedBy,
		Payload:     ticketPayload(row.Numero, row.Subject, row.ProjectName),
		NewAssignee: in.AssigneeID,
		Kind:        NotifyTicketCreated,
	})
	if err != nil {
		return TicketItem{}, err
	}

	if err := recordProjectEvent(ctx, q, row.ProjectID, InteractionTicketOpened, in.CreatedBy, map[string]any{
		"numero": row.Numero, "title": row.Subject,
	}); err != nil {
		return TicketItem{}, err
	}

	if in.FromPortal {
		if err := s.mailTeam(ctx, q, row.ID, true, in.AuthorName, strings.TrimSpace(in.Description)); err != nil {
			return TicketItem{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return TicketItem{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return TicketItem{
		ID:        row.ID,
		Numero:    row.Numero,
		Project:   TicketProject{ID: row.ProjectID, Name: row.ProjectName},
		Tracker:   row.Tracker,
		Status:    row.Status,
		Priority:  row.Priority,
		Subject:   row.Subject,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// TicketPerson nomme qui a parle ou agi. Nul quand le compte a quitte l'agence :
// le registre garde la trace, il perd seulement son auteur.
type TicketPerson struct {
	ID        uuid.UUID `json:"id"`
	Firstname string    `json:"firstname"`
	Lastname  string    `json:"lastname"`
	Initials  string    `json:"initials"`
	AvatarURL *string   `json:"avatar_url"`
	// admin, team ou client : l'ecran distingue l'agence du client.
	Role string `json:"role"`
}

// TicketEntry est une entree du registre.
//
// Les messages et les evenements y prennent la meme forme parce que l'ecran les
// entrelace sur un seul fil : un changement de statut compte autant qu'une
// phrase dans l'histoire d'un ticket. `Kind` dit laquelle des deux moities
// porte du sens.
type TicketEntry struct {
	ID uuid.UUID `json:"id"`
	// "message" ou "event".
	Kind   string        `json:"kind"`
	At     time.Time     `json:"at"`
	Author *TicketPerson `json:"author"`
	// Sans auteur des l'origine : un geste de Piilot lui-meme — l'integration
	// Git, une tache de fond. Distinct d'un auteur dont le compte a disparu.
	Automatic  bool   `json:"automatic"`
	Body       string `json:"body"`
	IsInternal bool   `json:"is_internal"`
	// Champ modifie et valeurs, pour un evenement : status, priority, tracker
	// ou assignee. Vides pour un message.
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// TicketDetail est tout ce que la fiche d'un ticket affiche, en un appel.
type TicketDetail struct {
	TicketItem
	Description string         `json:"description"`
	Client      *TicketProject `json:"client"`
	Reporter    *TicketPerson  `json:"reporter"`
	// Visible du client dans son portail : le ticket vient de lui, et ce qui
	// n'est pas note interne lui parvient.
	ClientVisible bool `json:"client_visible"`
	// Pieces jointes deposees avec la demande.
	Files []Attachment `json:"files"`
	// Le registre, deja fusionne et trie du plus ancien au plus recent.
	Entries []TicketEntry `json:"entries"`
	// Les pull requests qui le nomment, recues par webhook.
	PullRequests []PullRequest `json:"pull_requests"`
}

// personOfTicket compose une personne a partir des colonnes d'une jointure a
// gauche, qui arrivent nulles quand il n'y a personne a nommer.
func personOfTicket(id *uuid.UUID, firstname, lastname *string, avatar *string, role string) *TicketPerson {
	if id == nil || firstname == nil || lastname == nil {
		return nil
	}

	initials := ""
	if len(*firstname) > 0 {
		initials += strings.ToUpper((*firstname)[:1])
	}
	if len(*lastname) > 0 {
		initials += strings.ToUpper((*lastname)[:1])
	}

	return &TicketPerson{
		ID:        *id,
		Firstname: *firstname,
		Lastname:  *lastname,
		Initials:  initials,
		AvatarURL: avatar,
		Role:      role,
	}
}

// Get renvoie la fiche d'un ticket et son registre.
func (s *TicketService) Get(ctx context.Context, id uuid.UUID) (TicketDetail, error) {
	row, err := s.q.GetTicket(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TicketDetail{}, domain.ErrNotFound
		}

		return TicketDetail{}, fmt.Errorf("lecture du ticket : %w", err)
	}

	detail := TicketDetail{
		TicketItem: TicketItem{
			ID:        row.ID,
			Numero:    row.Numero,
			Project:   TicketProject{ID: row.ProjectID, Name: row.ProjectName},
			Tracker:   row.Tracker,
			Status:    row.Status,
			Priority:  row.Priority,
			Subject:   row.Subject,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		},
		Description: row.Description,
	}

	if row.ClientID != nil && row.ClientName != nil {
		detail.Client = &TicketProject{ID: *row.ClientID, Name: *row.ClientName}
	}

	detail.Assignee = personOfTicket(
		row.AssigneeID, row.AssigneeFirstname, row.AssigneeLastname, row.AssigneeAvatarUrl, "",
	)
	detail.Reporter = personOfTicket(
		row.CreatedBy, row.ReporterFirstname, row.ReporterLastname, row.ReporterAvatarUrl, "",
	)

	entries, err := s.registre(ctx, id)
	if err != nil {
		return TicketDetail{}, err
	}

	detail.Entries = entries
	detail.ClientVisible = row.ClientVisible

	files, err := s.q.ListTicketFiles(ctx, &id)
	if err != nil {
		return TicketDetail{}, fmt.Errorf("pieces jointes du ticket : %w", err)
	}
	detail.Files = make([]Attachment, 0, len(files))
	for _, f := range files {
		detail.Files = append(detail.Files, Attachment{
			ID: f.ID, Filename: f.Filename, ContentType: f.ContentType, SizeBytes: f.SizeBytes, CreatedAt: f.CreatedAt,
		})
	}

	prs, err := s.q.ListPullRequestsOfTicket(ctx, &id)
	if err != nil {
		return TicketDetail{}, fmt.Errorf("pull requests du ticket : %w", err)
	}
	detail.PullRequests = make([]PullRequest, 0, len(prs))
	for _, pr := range prs {
		detail.PullRequests = append(detail.PullRequests, pullRequestOf(pr))
	}

	return detail, nil
}

// registre fusionne les messages et les evenements en un seul fil trie.
//
// Deux requetes plutot qu'une UNION : les deux tables n'ont pas les memes
// colonnes, et les projeter sur une forme commune en SQL rendrait la requete
// illisible pour economiser un aller-retour sur des volumes qui se comptent en
// dizaines de lignes.
func (s *TicketService) registre(ctx context.Context, ticketID uuid.UUID) ([]TicketEntry, error) {
	messages, err := s.q.ListTicketMessages(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("messages du ticket : %w", err)
	}

	events, err := s.q.ListTicketEvents(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("journal du ticket : %w", err)
	}

	entries := make([]TicketEntry, 0, len(messages)+len(events))

	for _, m := range messages {
		role := ""
		if m.AuthorRole != nil {
			role = *m.AuthorRole
		}

		entries = append(entries, TicketEntry{
			ID:         m.ID,
			Kind:       "message",
			At:         m.CreatedAt,
			Author:     personOfTicket(m.AuthorID, m.AuthorFirstname, m.AuthorLastname, m.AuthorAvatarUrl, role),
			Automatic:  m.AuthorID == nil,
			Body:       m.Body,
			IsInternal: m.IsInternal,
		})
	}

	for _, e := range events {
		entries = append(entries, TicketEntry{
			ID:        e.ID,
			Kind:      "event",
			At:        e.CreatedAt,
			Author:    personOfTicket(e.ActorID, e.ActorFirstname, e.ActorLastname, e.ActorAvatarUrl, ""),
			Automatic: e.ActorID == nil,
			Field:     e.Field,
			OldValue:  e.OldValue,
			NewValue:  e.NewValue,
		})
	}

	// Tri stable : a horodatage egal — un message et le changement de statut
	// qu'il porte sont ecrits dans la meme milliseconde — le message passe
	// devant, parce que c'est lui qui explique le mouvement.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].At.Equal(entries[j].At) {
			return entries[i].Kind == "message" && entries[j].Kind == "event"
		}

		return entries[i].At.Before(entries[j].At)
	})

	return entries, nil
}

// PostMessageInput decrit une entree a inscrire au registre.
//
// Les changements y sont facultatifs mais volontairement au meme endroit que le
// message : repondre et faire avancer un ticket sont un seul geste dans la
// vraie vie, et les separer en deux actions est ce qui laisse des tickets
// « A faire » alors qu'ils sont traites.
type PostMessageInput struct {
	Body       string
	IsInternal bool
	AuthorID   *uuid.UUID

	// Nuls pour ne rien changer.
	NewStatus   *string
	NewPriority *string

	// L'assignation a son propre drapeau : nul y veut dire « remettre a
	// prendre », ce qu'un pointeur seul ne saurait pas distinguer de « ne
	// touche pas a l'assignation ».
	ChangeAssignee bool
	NewAssigneeID  *uuid.UUID

	// Ecrit depuis le portail : l'agence est prevenue par e-mail. Sinon, le
	// client l'est d'une reponse publique ou d'un statut qui change a ses yeux.
	FromClient bool
	AuthorName string
}

// nameOfAssignee compose le nom porte par les colonnes de la jointure a gauche.
// Vide quand le ticket n'est confie a personne.
func nameOfAssignee(firstname, lastname *string) string {
	if firstname == nil || lastname == nil {
		return ""
	}

	return strings.TrimSpace(*firstname + " " + *lastname)
}

// PostMessage inscrit un message, et applique les changements qu'il annonce.
//
// Tout va dans la meme transaction : un message inscrit sans le changement
// qu'il annonce, ou l'inverse, laisserait un registre qui ment.
//
// Le journal est tire d'une comparaison entre la fiche d'avant et celle
// d'apres, relues toutes deux dans la transaction. C'est ce qui garantit qu'une
// ligne de journal correspond a un changement reel : demander « passer en cours »
// sur un ticket deja en cours n'ecrit rien.
func (s *TicketService) PostMessage(
	ctx context.Context,
	ticketID uuid.UUID,
	in PostMessageInput,
) (TicketDetail, error) {
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return TicketDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"body": "Le message est requis",
		})
	}

	if in.NewStatus != nil {
		if _, ok := ticketStatuses[*in.NewStatus]; !ok {
			return TicketDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"status": "Statut inconnu",
			})
		}
	}

	if in.NewPriority != nil {
		if _, ok := ticketPriorities[*in.NewPriority]; !ok {
			return TicketDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"priority": "Priorité inconnue",
			})
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TicketDetail{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	// Le verrou d'abord : il met en file les ecritures concurrentes sur ce
	// ticket, pour que la seconde lise le travail de la premiere au lieu de
	// journaliser le meme changement une deuxieme fois.
	if _, err := q.LockTicket(ctx, ticketID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TicketDetail{}, domain.ErrNotFound
		}

		return TicketDetail{}, fmt.Errorf("verrou du ticket : %w", err)
	}

	avant, err := q.GetTicket(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TicketDetail{}, domain.ErrNotFound
		}

		return TicketDetail{}, fmt.Errorf("lecture du ticket : %w", err)
	}

	if _, err := q.CreateTicketMessage(ctx, db.CreateTicketMessageParams{
		TicketID:   ticketID,
		AuthorID:   in.AuthorID,
		Body:       body,
		IsInternal: in.IsInternal,
	}); err != nil {
		return TicketDetail{}, fmt.Errorf("inscription du message : %w", err)
	}

	apresStatus := avant.Status
	var nouvelAssigne *uuid.UUID

	if in.NewStatus != nil || in.NewPriority != nil || in.ChangeAssignee {
		if err := q.UpdateTicketFields(ctx, db.UpdateTicketFieldsParams{
			ID:             ticketID,
			Status:         in.NewStatus,
			Priority:       in.NewPriority,
			ChangeAssignee: in.ChangeAssignee,
			AssigneeID:     in.NewAssigneeID,
		}); err != nil {
			return TicketDetail{}, fmt.Errorf("mise a jour du ticket : %w", err)
		}

		apres, err := q.GetTicket(ctx, ticketID)
		if err != nil {
			return TicketDetail{}, fmt.Errorf("relecture du ticket : %w", err)
		}

		// Les valeurs de l'assigne sont des noms et non des identifiants : le
		// journal fige ce qui etait vrai ce jour-la, et un nom se lit encore
		// quand le compte a disparu.
		changes := []struct{ field, old, new string }{
			{"status", avant.Status, apres.Status},
			{"priority", avant.Priority, apres.Priority},
			{
				"assignee",
				nameOfAssignee(avant.AssigneeFirstname, avant.AssigneeLastname),
				nameOfAssignee(apres.AssigneeFirstname, apres.AssigneeLastname),
			},
		}

		for _, change := range changes {
			if change.old == change.new {
				continue
			}

			if _, err := q.CreateTicketEvent(ctx, db.CreateTicketEventParams{
				TicketID: ticketID,
				ActorID:  in.AuthorID,
				Field:    change.field,
				OldValue: change.old,
				NewValue: change.new,
			}); err != nil {
				return TicketDetail{}, fmt.Errorf("journal du ticket : %w", err)
			}
		}

		apresStatus = apres.Status
		if apres.AssigneeID != nil && (avant.AssigneeID == nil || *avant.AssigneeID != *apres.AssigneeID) {
			nouvelAssigne = apres.AssigneeID
		}
	}

	if in.FromClient {
		if err := s.mailTeam(ctx, q, ticketID, false, in.AuthorName, body); err != nil {
			return TicketDetail{}, err
		}
	} else {
		quote := body
		if in.IsInternal {
			quote = ""
		}
		status := ""
		if PortalTicketStatus(apresStatus) != PortalTicketStatus(avant.Status) {
			status = PortalTicketStatus(apresStatus)
		}
		if err := s.mailClient(ctx, q, ticketID, quote, status); err != nil {
			return TicketDetail{}, err
		}
	}

	// Une seule notification par personne et par geste, la plus parlante : etre
	// designe pour traiter le ticket prime sur le changement de statut, qui
	// prime sur la simple reponse.
	payload := ticketPayload(avant.Numero, avant.Subject, avant.ProjectName)
	payload["excerpt"] = excerpt(body)
	payload["internal"] = in.IsInternal
	kind := NotifyTicketReplied
	if apresStatus != avant.Status {
		kind = NotifyTicketStatusChanged
		payload["from"] = avant.Status
		payload["to"] = apresStatus
	}

	err = s.notifyTicket(ctx, q, ticketNotice{
		TicketID:    ticketID,
		ProjectID:   avant.ProjectID,
		Actor:       in.AuthorID,
		Payload:     payload,
		NewAssignee: nouvelAssigne,
		Kind:        kind,
	})
	if err != nil {
		return TicketDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TicketDetail{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	// La fiche entiere est rendue : l'ecran remplace son etat d'un bloc plutot
	// que d'ajouter la ligne a la main puis de se resynchroniser.
	return s.Get(ctx, ticketID)
}

// Rename change le sujet d'un ticket, et l'inscrit au journal.
//
// Endpoint a part plutot qu'un champ de plus sur l'inscription au registre :
// renommer n'est pas un message, ca se fait d'un clic sur le titre et sans
// rien avoir a dire.
func (s *TicketService) Rename(
	ctx context.Context,
	ticketID uuid.UUID,
	subject string,
	actorID *uuid.UUID,
) (TicketDetail, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return TicketDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"subject": "Le sujet est requis",
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TicketDetail{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	// Le verrou d'abord : il met en file les ecritures concurrentes sur ce
	// ticket, pour que la seconde lise le travail de la premiere au lieu de
	// journaliser le meme changement une deuxieme fois.
	if _, err := q.LockTicket(ctx, ticketID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TicketDetail{}, domain.ErrNotFound
		}

		return TicketDetail{}, fmt.Errorf("verrou du ticket : %w", err)
	}

	avant, err := q.GetTicket(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TicketDetail{}, domain.ErrNotFound
		}

		return TicketDetail{}, fmt.Errorf("lecture du ticket : %w", err)
	}

	// Rien a faire, et surtout rien a journaliser : reouvrir le titre puis le
	// refermer sans y toucher ne doit pas laisser de trace.
	if avant.Subject == subject {
		return s.Get(ctx, ticketID)
	}

	if err := q.UpdateTicketSubject(ctx, db.UpdateTicketSubjectParams{
		ID:      ticketID,
		Subject: subject,
	}); err != nil {
		return TicketDetail{}, fmt.Errorf("renommage du ticket : %w", err)
	}

	if _, err := q.CreateTicketEvent(ctx, db.CreateTicketEventParams{
		TicketID: ticketID,
		ActorID:  actorID,
		Field:    "subject",
		OldValue: avant.Subject,
		NewValue: subject,
	}); err != nil {
		return TicketDetail{}, fmt.Errorf("journal du ticket : %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return TicketDetail{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return s.Get(ctx, ticketID)
}

// ticketNotice decrit un geste pose sur un ticket, pret a etre notifie.
type ticketNotice struct {
	TicketID  uuid.UUID
	ProjectID uuid.UUID
	Actor     *uuid.UUID
	Kind      string
	Payload   map[string]any
	// Personne qui vient d'etre designee pour traiter le ticket : elle recoit
	// « vous a confie » plutot que le genre du geste.
	NewAssignee *uuid.UUID
}

// notifyTicket previent les administrateurs, la personne qui traite le ticket
// et celle qui l'a ouvert. Appelee dans la transaction du geste.
func (s *TicketService) notifyTicket(ctx context.Context, q *db.Queries, n ticketNotice) error {
	actor := uuid.Nil
	if n.Actor != nil {
		actor = *n.Actor
	}

	recipients, err := q.ListTicketNotificationRecipients(ctx, db.ListTicketNotificationRecipientsParams{
		TicketID: n.TicketID,
		ActorID:  actor,
	})
	if err != nil {
		return fmt.Errorf("recherche des destinataires : %w", err)
	}

	base := notice{
		ActorID:   actor,
		Payload:   n.Payload,
		ProjectID: &n.ProjectID,
		TicketID:  &n.TicketID,
	}

	others := make([]uuid.UUID, 0, len(recipients))
	for _, id := range recipients {
		if n.NewAssignee == nil || id != *n.NewAssignee {
			others = append(others, id)
		}
	}

	// La personne designee figure deja parmi les destinataires : la requete lit
	// le ticket dans la transaction, apres sa mise a jour. Le test d'appartenance
	// l'ecarte quand elle ne doit rien recevoir — compte du portail, desactive,
	// ou auteur du geste.
	if n.NewAssignee != nil && slices.Contains(recipients, *n.NewAssignee) {
		assigned := base
		assigned.Kind = NotifyTicketAssigned
		assigned.Recipients = []uuid.UUID{*n.NewAssignee}
		if err := deliver(ctx, q, s.bus, assigned); err != nil {
			return err
		}
	}

	rest := base
	rest.Kind = n.Kind
	rest.Recipients = others

	return deliver(ctx, q, s.bus, rest)
}

// ticketPayload porte de quoi ecrire la phrase sans relire le ticket.
func ticketPayload(numero int64, subject, project string) map[string]any {
	return map[string]any{"numero": numero, "title": subject, "project": project}
}
