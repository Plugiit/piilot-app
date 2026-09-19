package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// ProjectService est le contrat dont les endpoints de projets ont besoin.
// Declare chez le consommateur, comme AuthService : les handlers restent
// testables sans Postgres.
type ProjectService interface {
	List(ctx context.Context, f usecase.ProjectFilters) (usecase.ProjectPage, error)
	Get(ctx context.Context, id, viewer uuid.UUID) (usecase.ProjectDetail, error)
	Create(ctx context.Context, in usecase.CreateProjectInput) (usecase.ProjectDetail, error)
	Update(ctx context.Context, id, viewer uuid.UUID, in usecase.UpdateProjectInput) (usecase.ProjectDetail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetTeam(ctx context.Context, id, viewer uuid.UUID, userIDs []uuid.UUID) (usecase.ProjectDetail, error)
	AddFile(ctx context.Context, projectID, uploader uuid.UUID, filename, contentType string, content io.Reader) (usecase.Attachment, error)
	OpenFile(ctx context.Context, fileID uuid.UUID) (usecase.Attachment, io.ReadCloser, error)
	DeleteFile(ctx context.Context, fileID uuid.UUID) error
	SetFavorite(ctx context.Context, projectID, userID uuid.UUID, on bool) error
	ListFavorites(ctx context.Context, viewer uuid.UUID) ([]usecase.ProjectShortcut, error)
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
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ClientID    *string  `json:"client_id"`
	ClientName  string   `json:"client_name"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	FigmaURL    string   `json:"figma_url"`
	ProdURL     string   `json:"prod_url"`
	PreprodURL  string   `json:"preprod_url"`
	Progress    int      `json:"progress"`
	HoursSold   float64  `json:"hours_sold"`
	StartsOn    *string  `json:"starts_on"`
	DueOn       *string  `json:"due_on"`
	TeamIDs     []string `json:"team_ids"`
	ServiceIDs  []string `json:"service_ids"`
}

// updateProjectRequest est le corps de PATCH /admin/projects/:id.
//
// Tous les champs sont optionnels : ce qui n'est pas envoye n'est pas modifie.
// Une date explicitement nulle efface l'echeance, ce qu'un champ absent ne fait
// pas — d'ou le pointeur de pointeur simule par la paire valeur/presence.
type updateProjectRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	ClientID    *string  `json:"client_id"`
	Status      *string  `json:"status"`
	Priority    *string  `json:"priority"`
	FigmaURL    *string  `json:"figma_url"`
	ProdURL     *string  `json:"prod_url"`
	PreprodURL  *string  `json:"preprod_url"`
	Progress    *int     `json:"progress"`
	HoursSold   *float64 `json:"hours_sold"`
	IsInternal  *bool    `json:"is_internal"`
	StartsOn    *string  `json:"starts_on"`
	DueOn       *string  `json:"due_on"`
	ServiceIDs  []string `json:"service_ids"`
}

type setTeamRequest struct {
	UserIDs []string `json:"user_ids"`
}

// List sert la liste paginee des projets.
func (h *Projects) List(c fiber.Ctx) error {
	// L'etoile etant personnelle, la liste ne peut pas se lire sans savoir qui
	// la demande : c'est elle qui decide des lignes en tete.
	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	filters := usecase.ProjectFilters{
		Sort:     c.Query("sort", "due"),
		Dir:      c.Query("dir", "asc"),
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 20),
		Viewer:   actor,
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

// Favorites sert les raccourcis de la barre laterale.
func (h *Projects) Favorites(c fiber.Ctx) error {
	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	items, err := h.svc.ListFavorites(c.Context(), actor)
	if err != nil {
		return err
	}

	// Enveloppe plutot que tableau nu : une reponse JSON qui est un tableau ne
	// peut plus rien gagner, et le reste de l'API rend deja `{ items }`.
	return c.JSON(fiber.Map{"items": items})
}

// Get sert l'en-tete d'un projet.
func (h *Projects) Get(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	project, err := h.svc.Get(c.Context(), id, actor)
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
		Name:        req.Name,
		Description: req.Description,
		ClientName:  req.ClientName,
		Status:      req.Status,
		Priority:    req.Priority,
		FigmaURL:    req.FigmaURL,
		ProdURL:     req.ProdURL,
		PreprodURL:  req.PreprodURL,
		Progress:    req.Progress,
		HoursSold:   req.HoursSold,
		CreatedBy:   actor,
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

	services, err := parseUUIDs(req.ServiceIDs, "service_ids")
	if err != nil {
		return err
	}
	in.ServiceIDs = services

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
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		FigmaURL:    req.FigmaURL,
		ProdURL:     req.ProdURL,
		PreprodURL:  req.PreprodURL,
		Progress:    req.Progress,
		HoursSold:   req.HoursSold,
		IsInternal:  req.IsInternal,
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
	// La cle absente laisse les services en place ; presente, elle fixe la
	// liste entiere — vide comprise, qui les detache tous.
	if hasJSONKey(body, "service_ids") {
		services, err := parseUUIDs(req.ServiceIDs, "service_ids")
		if err != nil {
			return err
		}
		in.ServiceIDs = &services
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	project, err := h.svc.Update(c.Context(), id, actor, in)
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

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	project, err := h.svc.SetTeam(c.Context(), id, actor, ids)
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

// UploadFile recoit une piece jointe du projet.
func (h *Projects) UploadFile(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	header, err := c.FormFile("file")
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{
			"file": "Aucun fichier reçu sous le champ « file »",
		})
	}

	content, err := header.Open()
	if err != nil {
		return fmt.Errorf("lecture du fichier envoye : %w", err)
	}
	defer func() { _ = content.Close() }()

	file, err := h.svc.AddFile(
		c.Context(), id, actor,
		header.Filename,
		header.Header.Get("Content-Type"),
		content,
	)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(file)
}

// DownloadFile rend le contenu d'une piece jointe.
//
// Toujours en piece jointe et jamais affichee dans l'onglet : un fichier
// televerse est du contenu que l'API n'a pas ecrit, et le rendre a l'ecran
// depuis le domaine de l'API laisserait un HTML depose executer du script avec
// les cookies de session. `attachment` et `nosniff` ferment les deux portes —
// celle du rendu direct, et celle du navigateur qui devine un type plus
// permissif que celui annonce.
func (h *Projects) DownloadFile(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	file, content, err := h.svc.OpenFile(c.Context(), id)
	if err != nil {
		return err
	}

	// Pas de `defer Close` : le corps est ecrit apres le retour du handler, et
	// fermer ici couperait le flux avant qu'il ne soit lu — la reponse partait
	// vide. C'est Fiber qui referme le lecteur, parce qu'il porte Close.

	c.Set(fiber.HeaderContentType, file.ContentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderContentDisposition, contentDisposition(file.Filename))

	return c.SendStream(content, int(file.SizeBytes))
}

// DeleteFile efface une piece jointe.
func (h *Projects) DeleteFile(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.DeleteFile(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Favorite pose l'etoile sur un projet pour le compte appelant.
func (h *Projects) Favorite(c fiber.Ctx) error { return h.setFavorite(c, true) }

// Unfavorite la retire.
func (h *Projects) Unfavorite(c fiber.Ctx) error { return h.setFavorite(c, false) }

func (h *Projects) setFavorite(c fiber.Ctx, on bool) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	if err := h.svc.SetFavorite(c.Context(), id, actor, on); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// contentDisposition compose l'en-tete de telechargement.
//
// Deux formes, comme le veut la RFC 6266 : une version ASCII pour les clients
// anciens, et `filename*` en UTF-8 pour les noms accentues. Les guillemets et
// les retours a la ligne sont retires de la premiere — laisses tels quels, ils
// permettraient d'injecter un en-tete de plus.
func contentDisposition(filename string) string {
	ascii := make([]rune, 0, len(filename))
	for _, r := range filename {
		switch {
		case r == '"' || r == '\\' || r == '\r' || r == '\n':
			ascii = append(ascii, '_')
		case r < 32 || r > 126:
			ascii = append(ascii, '_')
		default:
			ascii = append(ascii, r)
		}
	}

	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		string(ascii), url.PathEscape(filename))
}
