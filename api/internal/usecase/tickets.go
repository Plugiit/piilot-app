package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
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
}

func NewTicketService(pool *pgxpool.Pool) *TicketService {
	return &TicketService{pool: pool, q: db.New(pool)}
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

	row, err := s.q.CreateTicket(ctx, db.CreateTicketParams{
		ProjectID:   in.ProjectID,
		Subject:     subject,
		Description: strings.TrimSpace(in.Description),
		Tracker:     in.Tracker,
		Status:      "backlog",
		Priority:    in.Priority,
		AssigneeID:  in.AssigneeID,
		CreatedBy:   in.CreatedBy,
	})
	if err != nil {
		return TicketItem{}, fmt.Errorf("creation du ticket : %w", err)
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
	Kind       string        `json:"kind"`
	At         time.Time     `json:"at"`
	Author     *TicketPerson `json:"author"`
	Body       string        `json:"body"`
	IsInternal bool          `json:"is_internal"`
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
	// Le registre, deja fusionne et trie du plus ancien au plus recent.
	Entries []TicketEntry `json:"entries"`
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
			Body:       m.Body,
			IsInternal: m.IsInternal,
		})
	}

	for _, e := range events {
		entries = append(entries, TicketEntry{
			ID:       e.ID,
			Kind:     "event",
			At:       e.CreatedAt,
			Author:   personOfTicket(e.ActorID, e.ActorFirstname, e.ActorLastname, e.ActorAvatarUrl, ""),
			Field:    e.Field,
			OldValue: e.OldValue,
			NewValue: e.NewValue,
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
