package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// searchRows borne chaque famille de resultats : la palette montre les
// premiers, pas une liste.
const searchRows = 5

// SearchScope dit quelles familles l'appelant a le droit de voir. Le handler
// le remplit d'apres ses permissions : une famille hors droit est vide, pas
// refusee — la palette reste utilisable pour le reste.
type SearchScope struct {
	Projects bool
	Clients  bool
	Tasks    bool
	Tickets  bool
}

// SearchResult est ce que la palette affiche pour une saisie.
type SearchResult struct {
	Projects []SearchProject `json:"projects"`
	Clients  []SearchClient  `json:"clients"`
	Contacts []SearchContact `json:"contacts"`
	Tasks    []SearchTask    `json:"tasks"`
	Tickets  []SearchTicket  `json:"tickets"`
}

type SearchProject struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	ClientName string    `json:"client_name"`
	LogoURL    *string   `json:"logo_url"`
}

type SearchClient struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Kind   string    `json:"kind"`
}

type SearchContact struct {
	ID         uuid.UUID  `json:"id"`
	Firstname  string     `json:"firstname"`
	Lastname   string     `json:"lastname"`
	Email      *string    `json:"email"`
	ClientID   *uuid.UUID `json:"client_id"`
	ClientName string     `json:"client_name"`
}

type SearchTask struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	ProjectID   uuid.UUID `json:"project_id"`
	ProjectName string    `json:"project_name"`
}

type SearchTicket struct {
	ID          uuid.UUID `json:"id"`
	Numero      int64     `json:"numero"`
	Subject     string    `json:"subject"`
	Status      string    `json:"status"`
	ProjectName string    `json:"project_name"`
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// SearchService sert la palette.
type SearchService struct {
	q *db.Queries
}

func NewSearchService(pool *pgxpool.Pool) *SearchService {
	return &SearchService{q: db.New(pool)}
}

// Search cherche `query` dans chaque famille du perimetre. Une saisie trop
// courte rend un resultat vide : un seul caractere retrouverait tout.
func (s *SearchService) Search(ctx context.Context, query string, scope SearchScope) (SearchResult, error) {
	out := SearchResult{
		Projects: []SearchProject{}, Clients: []SearchClient{}, Contacts: []SearchContact{},
		Tasks: []SearchTask{}, Tickets: []SearchTicket{},
	}

	query = strings.TrimSpace(query)
	// Un numero de ticket se tape tel quel, meme a un chiffre.
	if len([]rune(query)) < 2 && !isNumber(strings.TrimPrefix(query, "#")) {
		return out, nil
	}

	if scope.Projects {
		rows, err := s.q.SearchProjects(ctx, db.SearchProjectsParams{Q: query, MaxRows: searchRows})
		if err != nil {
			return out, fmt.Errorf("recherche des projets : %w", err)
		}
		for _, r := range rows {
			out.Projects = append(out.Projects, SearchProject{
				ID: r.ID, Name: r.Name, Status: r.Status, ClientName: r.ClientName, LogoURL: projectLogoURL(r.LogoKey),
			})
		}
	}

	if scope.Clients {
		clients, err := s.q.SearchClients(ctx, db.SearchClientsParams{Q: query, MaxRows: searchRows})
		if err != nil {
			return out, fmt.Errorf("recherche des clients : %w", err)
		}
		for _, r := range clients {
			out.Clients = append(out.Clients, SearchClient{ID: r.ID, Name: r.Name, Status: r.Status, Kind: r.Kind})
		}

		contacts, err := s.q.SearchContacts(ctx, db.SearchContactsParams{Q: query, MaxRows: searchRows})
		if err != nil {
			return out, fmt.Errorf("recherche des contacts : %w", err)
		}
		for _, r := range contacts {
			out.Contacts = append(out.Contacts, SearchContact{
				ID: r.ID, Firstname: r.Firstname, Lastname: r.Lastname, Email: r.Email,
				ClientID: r.ClientID, ClientName: r.ClientName,
			})
		}
	}

	if scope.Tasks {
		rows, err := s.q.SearchTasks(ctx, db.SearchTasksParams{Q: query, MaxRows: searchRows})
		if err != nil {
			return out, fmt.Errorf("recherche des taches : %w", err)
		}
		for _, r := range rows {
			out.Tasks = append(out.Tasks, SearchTask{
				ID: r.ID, Title: r.Title, Status: r.Status, ProjectID: r.ProjectID, ProjectName: r.ProjectName,
			})
		}
	}

	if scope.Tickets {
		rows, err := s.q.SearchTickets(ctx, db.SearchTicketsParams{Q: strings.TrimPrefix(query, "#"), MaxRows: searchRows})
		if err != nil {
			return out, fmt.Errorf("recherche des tickets : %w", err)
		}
		for _, r := range rows {
			out.Tickets = append(out.Tickets, SearchTicket{
				ID: r.ID, Numero: r.Numero, Subject: r.Subject, Status: r.Status, ProjectName: r.ProjectName,
			})
		}
	}

	return out, nil
}
