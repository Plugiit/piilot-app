package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/middleware"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

// ProjectService est le contrat dont les endpoints de projets ont besoin.
// Declare chez le consommateur, comme AuthService : les handlers restent
// testables sans Postgres.
type ProjectService interface {
	List(ctx context.Context, f usecase.ProjectFilters) (usecase.ProjectPage, error)
	Get(ctx context.Context, id uuid.UUID) (usecase.ProjectDetail, error)
	Create(ctx context.Context, in usecase.CreateProjectInput) (usecase.ProjectDetail, error)
	Update(ctx context.Context, id uuid.UUID, in usecase.UpdateProjectInput) (usecase.ProjectDetail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetTeam(ctx context.Context, id uuid.UUID, userIDs []uuid.UUID) (usecase.ProjectDetail, error)
	ListClients(ctx context.Context, search *string, page, pageSize int) ([]usecase.ClientItem, error)
	ListPeople(ctx context.Context) ([]usecase.Person, error)
	Dashboard(ctx context.Context) (usecase.DashboardSummary, error)
}

// Projects porte les endpoints des projets.
type Projects struct {
	svc ProjectService
}

// NewProjects construit le handler.
func NewProjects(svc ProjectService) *Projects {
	return &Projects{svc: svc}
}

// createProjectRequest est le corps de POST /admin/projects.
//
// Le client se designe par identifiant ou par nom : le formulaire propose une
// liste, mais accepte un nom inconnu plutot que d'exiger un detour par un ecran
// de creation de client.
type createProjectRequest struct {
	Name       string   `json:"name"`
	ClientID   *string  `json:"client_id"`
	ClientName string   `json:"client_name"`
	Status     string   `json:"status"`
	Progress   int      `json:"progress"`
	HoursSold  float64  `json:"hours_sold"`
	StartsOn   *string  `json:"starts_on"`
	DueOn      *string  `json:"due_on"`
	TeamIDs    []string `json:"team_ids"`
}

// updateProjectRequest est le corps de PATCH /admin/projects/:id.
//
// Tous les champs sont optionnels : ce qui n'est pas envoye n'est pas modifie.
// Une date explicitement nulle efface l'echeance, ce qu'un champ absent ne fait
// pas — d'ou le pointeur de pointeur simule par la paire valeur/presence.
type updateProjectRequest struct {
	Name      *string  `json:"name"`
	ClientID  *string  `json:"client_id"`
	Status    *string  `json:"status"`
	Progress  *int     `json:"progress"`
	HoursSold *float64 `json:"hours_sold"`
	StartsOn  *string  `json:"starts_on"`
	DueOn     *string  `json:"due_on"`
}

type setTeamRequest struct {
	UserIDs []string `json:"user_ids"`
}

// List sert la liste paginee des projets.
func (h *Projects) List(c fiber.Ctx) error {
	filters := usecase.ProjectFilters{
		Sort:     c.Query("sort", "due"),
		Dir:      c.Query("dir", "asc"),
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 20),
	}

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filters.Status = &status
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		filters.Search = &search
	}
	if raw := strings.TrimSpace(c.Query("client_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"client_id": "Identifiant invalide"})
		}
		filters.ClientID = &id
	}

	page, err := h.svc.List(c.Context(), filters)
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// Get sert l'en-tete d'un projet.
func (h *Projects) Get(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	project, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(project)
}

// Create cree un projet.
func (h *Projects) Create(c fiber.Ctx) error {
	var req createProjectRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	in := usecase.CreateProjectInput{
		Name:       req.Name,
		ClientName: req.ClientName,
		Status:     req.Status,
		Progress:   req.Progress,
		HoursSold:  req.HoursSold,
		CreatedBy:  actor,
	}

	if req.ClientID != nil && strings.TrimSpace(*req.ClientID) != "" {
		id, err := uuid.Parse(*req.ClientID)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"client_id": "Identifiant invalide"})
		}
		in.ClientID = &id
	}

	starts, err := parseDatePointer(req.StartsOn, "starts_on")
	if err != nil {
		return err
	}
	due, err := parseDatePointer(req.DueOn, "due_on")
	if err != nil {
		return err
	}
	in.StartsOn = starts
	in.DueOn = due

	team, err := parseUUIDs(req.TeamIDs, "team_ids")
	if err != nil {
		return err
	}
	in.TeamIDs = team

	project, err := h.svc.Create(c.Context(), in)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(project)
}

// Update modifie un projet.
func (h *Projects) Update(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req updateProjectRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	in := usecase.UpdateProjectInput{
		Name:      req.Name,
		Status:    req.Status,
		Progress:  req.Progress,
		HoursSold: req.HoursSold,
	}

	if req.ClientID != nil {
		clientID, err := uuid.Parse(*req.ClientID)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"client_id": "Identifiant invalide"})
		}
		in.ClientID = &clientID
	}

	// Une cle presente et nulle efface la date ; une cle absente la laisse.
	// `Bind` ne distingue pas les deux, donc on relit le corps brut.
	body := c.Body()
	if hasJSONKey(body, "starts_on") {
		if req.StartsOn == nil {
			in.ClearStartsOn = true
		} else {
			starts, err := parseDatePointer(req.StartsOn, "starts_on")
			if err != nil {
				return err
			}
			in.StartsOn = starts
		}
	}
	if hasJSONKey(body, "due_on") {
		if req.DueOn == nil {
			in.ClearDueOn = true
		} else {
			due, err := parseDatePointer(req.DueOn, "due_on")
			if err != nil {
				return err
			}
			in.DueOn = due
		}
	}

	project, err := h.svc.Update(c.Context(), id, in)
	if err != nil {
		return err
	}

	return c.JSON(project)
}

// Delete efface un projet.
func (h *Projects) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// SetTeam remplace l'equipe d'un projet.
func (h *Projects) SetTeam(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req setTeamRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	ids, err := parseUUIDs(req.UserIDs, "user_ids")
	if err != nil {
		return err
	}

	project, err := h.svc.SetTeam(c.Context(), id, ids)
	if err != nil {
		return err
	}

	return c.JSON(project)
}

// ListClients sert le champ « Client » du formulaire de projet.
func (h *Projects) ListClients(c fiber.Ctx) error {
	var search *string
	if raw := strings.TrimSpace(c.Query("search")); raw != "" {
		search = &raw
	}

	clients, err := h.svc.ListClients(c.Context(), search, queryInt(c, "page", 1), queryInt(c, "page_size", 50))
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": clients})
}

// Dashboard sert les chiffres d'en-tete du module.
func (h *Projects) Dashboard(c fiber.Ctx) error {
	summary, err := h.svc.Dashboard(c.Context())
	if err != nil {
		return err
	}

	return c.JSON(summary)
}

// ListPeople sert les champs d'affectation.
func (h *Projects) ListPeople(c fiber.Ctx) error {
	people, err := h.svc.ListPeople(c.Context())
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": people})
}

// pathUUID lit un identifiant de l'URL.
func pathUUID(c fiber.Ctx, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return uuid.Nil, domain.ErrValidation.WithDetails(map[string]any{name: "Identifiant invalide"})
	}

	return id, nil
}

// queryInt lit un entier de la chaine de requete, avec sa valeur par defaut.
func queryInt(c fiber.Ctx, name string, fallback int) int {
	raw := c.Query(name)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}

// parseDatePointer lit une date « AAAA-MM-JJ ».
func parseDatePointer(raw *string, field string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}

	value, err := time.Parse("2006-01-02", strings.TrimSpace(*raw))
	if err != nil {
		return nil, domain.ErrValidation.WithDetails(map[string]any{field: "Date attendue au format AAAA-MM-JJ"})
	}

	return &value, nil
}

// parseUUIDs convertit une liste d'identifiants.
func parseUUIDs(raw []string, field string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(raw))

	for _, item := range raw {
		id, err := uuid.Parse(item)
		if err != nil {
			return nil, domain.ErrValidation.WithDetails(map[string]any{field: "Identifiant invalide"})
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// hasJSONKey dit si le corps porte cette cle, quelle que soit sa valeur.
//
// C'est ce qui distingue « echeance non modifiee » (cle absente) de « echeance
// effacee » (cle presente a null) : deballe dans une struct, les deux donnent
// un pointeur nil et deviennent indiscernables.
func hasJSONKey(body []byte, key string) bool {
	var raw map[string]json.RawMessage

	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}

	_, present := raw[key]

	return present
}
