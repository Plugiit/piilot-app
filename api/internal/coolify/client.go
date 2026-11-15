package coolify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// timeout borne chaque appel.
	timeout = 20 * time.Second
	// maxBody borne la lecture d'une reponse.
	maxBody = 16 << 20
	// pageSize : deploiements demandes par appel.
	pageSize = 100
	// maxPages borne la pagination : 2 000 deploiements par application.
	maxPages = 20
)

// ErrTooLarge : la reponse depasse maxBody.
var ErrTooLarge = errors.New("réponse de Coolify trop volumineuse")

// NewHTTPClient rend un client HTTP qui ne suit aucune redirection : le jeton
// ne doit pas partir vers une autre adresse que celle enregistree.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Client appelle l'API de Coolify.
type Client struct {
	// BaseURL est l'adresse de l'instance, sans /api/v1.
	BaseURL string
	// Token est le jeton complet, avec son « id| ».
	Token  string
	Client *http.Client
}

var _ Provider = (*Client)(nil)

// ListApplications lit GET /applications.
func (c *Client) ListApplications(ctx context.Context) ([]Application, error) {
	body, err := c.get(ctx, "/applications", nil)
	if err != nil {
		return nil, err
	}
	items, err := listOf(body, "data", "applications")
	if err != nil {
		return nil, err
	}
	apps := make([]Application, 0, len(items))
	for _, raw := range items {
		a := applicationOf(raw)
		if a.UUID == "" {
			continue
		}
		apps = append(apps, a)
	}
	return apps, nil
}

// ListDeployments lit GET /deployments/applications/{uuid}, page par page.
//
// L'ordre de Coolify n'est pas documente : il se lit sur la premiere page.
// Du plus recent au plus ancien, la lecture s'arrete a la premiere page qui ne
// contient plus que des deploiements anterieurs a since. Dans l'ordre inverse,
// ou s'il est indeterminable, toute la liste est lue, a concurrence de
// maxPages : au-dela, la lecture echoue plutot que de rendre une liste dont
// les plus recents manquent peut-etre. Un deploiement sans date valide est
// garde.
func (c *Client) ListDeployments(ctx context.Context, appUUID string, since time.Time) ([]Deployment, error) {
	if strings.TrimSpace(appUUID) == "" {
		return nil, errors.New("identifiant d'application manquant")
	}
	var out []Deployment
	newestFirst := false
	for page := 0; page < maxPages; page++ {
		query := url.Values{}
		query.Set("skip", fmt.Sprint(page*pageSize))
		query.Set("take", fmt.Sprint(pageSize))
		body, err := c.get(ctx, "/deployments/applications/"+url.PathEscape(appUUID), query)
		if err != nil {
			return nil, err
		}
		items, err := listOf(body, "deployments", "data")
		if err != nil {
			return nil, err
		}
		batch := make([]Deployment, 0, len(items))
		for _, raw := range items {
			batch = append(batch, deploymentOf(raw))
		}
		if page == 0 {
			newestFirst = isNewestFirst(batch)
		}
		recent := false
		for _, d := range batch {
			// since filtre sur created_at, pas sur la fin du deploiement : un
			// deploiement cree juste avant since et fini apres est ecarte. Sans
			// effet pratique (un deploiement dure des minutes, since est un debut
			// de mois), donc laisse tel quel.
			if !since.IsZero() && !d.CreatedAt.IsZero() && d.CreatedAt.Before(since) {
				continue
			}
			recent = true
			out = append(out, d)
		}
		if len(items) < pageSize || (newestFirst && !since.IsZero() && !recent) {
			return out, nil
		}
	}
	return nil, fmt.Errorf("plus de %d déploiements pour l’application %s : lecture abandonnée", maxPages*pageSize, appUUID)
}

// isNewestFirst dit si une page est strictement du plus recent au plus
// ancien, d'apres ses dates. Une page trop courte, sans dates ou en desordre
// ne permet pas de conclure.
func isNewestFirst(batch []Deployment) bool {
	var dated []time.Time
	for _, d := range batch {
		if !d.CreatedAt.IsZero() {
			dated = append(dated, d.CreatedAt)
		}
	}
	if len(dated) < 2 {
		return false
	}
	for i := 1; i < len(dated); i++ {
		if dated[i].After(dated[i-1]) {
			return false
		}
	}
	return dated[0].After(dated[len(dated)-1])
}

func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	base = strings.TrimSuffix(base, "/api/v1")
	if base == "" {
		return nil, errors.New("adresse de Coolify manquante")
	}
	target := base + "/api/v1" + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("adresse de Coolify : %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	req.Header.Set("Accept", "application/json")

	client := c.Client
	if client == nil {
		client = NewHTTPClient()
	}
	res, err := client.Do(req)
	if err != nil {
		// L'erreur de transport porte l'URL, jamais le jeton (en-tete).
		return nil, fmt.Errorf("appel de Coolify : %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("lecture de la réponse de Coolify : %w", err)
	}
	if len(body) > maxBody {
		return nil, ErrTooLarge
	}
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		return nil, fmt.Errorf("Coolify redirige (%d) vers une autre adresse : vérifiez l’adresse enregistrée (https, sans redirection)", res.StatusCode)
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case res.StatusCode == http.StatusForbidden:
		if msg := messageOf(body); msg != "" {
			return nil, fmt.Errorf("%w (%s)", ErrForbidden, msg)
		}
		return nil, ErrForbidden
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("Coolify répond %d", res.StatusCode)
	}
	return body, nil
}

// messageOf lit le champ « message » d'une reponse d'erreur, s'il y en a un.
func messageOf(body []byte) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) != nil {
		return ""
	}
	msg := strings.TrimSpace(e.Message)
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

// listOf lit une liste d'objets : un tableau JSON, ou un objet qui le porte
// sous l'une des cles donnees.
func listOf(body []byte, keys ...string) ([]map[string]any, error) {
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, errors.New("réponse inattendue : l’adresse ne pointe pas vers l’API de Coolify")
	}
	var list []any
	switch v := decoded.(type) {
	case []any:
		list = v
	case map[string]any:
		found := false
		for _, k := range keys {
			if l, ok := v[k].([]any); ok {
				list, found = l, true
				break
			}
		}
		if !found {
			return nil, errors.New("réponse inattendue : liste introuvable dans la réponse de Coolify")
		}
	default:
		return nil, errors.New("réponse inattendue de Coolify")
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func applicationOf(m map[string]any) Application {
	a := Application{UUID: str(m, "uuid"), Name: str(m, "name"), FQDN: str(m, "fqdn")}
	env, _ := m["environment"].(map[string]any)
	a.EnvironmentName = firstNonEmpty(str(m, "environment_name"), str(env, "name"))
	project, _ := env["project"].(map[string]any)
	a.ProjectName = firstNonEmpty(str(m, "project_name"), str(project, "name"))
	return a
}

func deploymentOf(m map[string]any) Deployment {
	return Deployment{
		UUID:          firstNonEmpty(str(m, "deployment_uuid"), str(m, "uuid")),
		Status:        str(m, "status"),
		CommitSHA:     str(m, "git_commit_sha"),
		CommitMessage: str(m, "commit_message"),
		CreatedAt:     when(m, "created_at"),
		UpdatedAt:     when(m, "updated_at"),
	}
}

// str lit une chaine ; tout autre type, ou une cle absente, donne « ».
func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

var timeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999-07",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
}

// when lit une date ; l'heure d'une date sans fuseau est lue en UTC.
func when(m map[string]any, key string) time.Time {
	s := str(m, key)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
