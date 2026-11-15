package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/coolify"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/zone"
)

// CoolifyService est le module de deploiements : il releve les applications
// d'une instance Coolify, que les projets pourront ensuite lier.
//
// Aucun ecran n'appelle Coolify : la releve est une tache de fond, le bouton
// « Tester » ne fait que la demander.
type CoolifyService struct {
	pool        *pgxpool.Pool
	q           *db.Queries
	env         config.CoolifyEnv
	sealKey     []byte
	client      *http.Client
	newProvider func(baseURL, token string) coolify.Provider
	log         *slog.Logger
}

const (
	coolifyDefaultPollMinutes = 60
	coolifyMinPollMinutes     = 15
	coolifyMaxPollMinutes     = 1440
)

// NewCoolifyService construit le service.
func NewCoolifyService(pool *pgxpool.Pool, env config.CoolifyEnv, secret []byte, log *slog.Logger) *CoolifyService {
	s := &CoolifyService{
		pool: pool, q: db.New(pool), env: env, sealKey: security.DeriveKey(secret, "coolify-api-token"),
		client: coolify.NewHTTPClient(), log: log,
	}
	s.newProvider = func(baseURL, token string) coolify.Provider {
		return &coolify.Client{BaseURL: baseURL, Token: token, Client: s.client}
	}
	return s
}

// SetProviderFactory remplace la fabrique du client Coolify : les tests y
// branchent un faux fournisseur.
func (s *CoolifyService) SetProviderFactory(f func(baseURL, token string) coolify.Provider) {
	s.newProvider = f
}

// --- Modules -----------------------------------------------------------------

// Modules rend la carte « Deploiements ».
func (s *CoolifyService) Modules(ctx context.Context) ([]ModuleView, error) {
	row, err := s.q.GetCoolifySettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("lecture du module : %w", err)
	}
	return []ModuleView{{
		Code: "deployments", Name: "Déploiements", Provider: "Coolify",
		Description: "Relève les applications d’une instance Coolify, pour que le rapport mensuel s’appuie sur les déploiements réellement faits.",
		Enabled:     row.Enabled, State: coolifyState(row),
	}}, nil
}

func coolifyState(row db.CoolifySetting) string {
	switch {
	case !row.Enabled:
		return "off"
	case row.LastPollError != nil:
		return "error"
	case row.LastPollAt == nil:
		return "pending"
	default:
		return "ok"
	}
}

// --- Reglages ----------------------------------------------------------------

// CoolifySettingsView est l'ecran du module. Le jeton n'en sort jamais.
type CoolifySettingsView struct {
	Enabled         bool              `json:"enabled"`
	BaseURL         string            `json:"base_url"`
	BaseURLFromEnv  bool              `json:"base_url_from_env"`
	APITokenSet     bool              `json:"api_token_set"`
	APITokenFromEnv bool              `json:"api_token_from_env"`
	PollMinutes     int               `json:"poll_minutes"`
	Status          CoolifyStatusView `json:"status"`
}

// CoolifyStatusView dit ou en est la releve.
type CoolifyStatusView struct {
	State         string     `json:"state"`
	LastPollAt    *time.Time `json:"last_poll_at"`
	LastPollError *string    `json:"last_poll_error"`
	PollPending   bool       `json:"poll_pending"`
	Applications  int        `json:"applications"`
}

// CoolifySettingsInput est ce que l'ecran enregistre. Un jeton nul garde
// celui qui est en base ; une chaine vide l'efface.
type CoolifySettingsInput struct {
	Enabled     bool
	BaseURL     string
	APIToken    *string
	PollMinutes int
}

// Settings rend l'ecran du module.
func (s *CoolifyService) Settings(ctx context.Context) (CoolifySettingsView, error) {
	row, err := s.q.GetCoolifySettings(ctx)
	if err != nil {
		return CoolifySettingsView{}, fmt.Errorf("lecture du module : %w", err)
	}
	counts, err := s.q.CountCoolifyApplications(ctx)
	if err != nil {
		return CoolifySettingsView{}, fmt.Errorf("compte des applications : %w", err)
	}
	baseURL, _ := s.connection(row)
	return CoolifySettingsView{
		Enabled: row.Enabled, BaseURL: baseURL, BaseURLFromEnv: s.env.URL != "",
		APITokenSet: s.env.APIToken != "" || len(row.ApiTokenEnc) > 0, APITokenFromEnv: s.env.APIToken != "",
		PollMinutes: int(row.PollMinutes),
		Status: CoolifyStatusView{
			State: coolifyState(row), LastPollAt: row.LastPollAt, LastPollError: row.LastPollError,
			PollPending: row.PollRequestedAt != nil, Applications: int(counts.Present),
		},
	}, nil
}

// normalizeCoolifyURL retire la barre finale et le /api/v1 que l'on colle
// parfois avec l'adresse.
func normalizeCoolifyURL(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	raw = strings.TrimSuffix(raw, "/api/v1")
	return strings.TrimRight(raw, "/")
}

// sameCoolifyHost compare les hotes (avec port) de deux adresses.
func sameCoolifyHost(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return false
	}
	return strings.EqualFold(ua.Host, ub.Host)
}

// UpdateSettings enregistre les reglages. Ce que l'environnement fixe n'est
// pas modifiable ici.
func (s *CoolifyService) UpdateSettings(ctx context.Context, in CoolifySettingsInput) (CoolifySettingsView, error) {
	details := map[string]any{}
	in.BaseURL = normalizeCoolifyURL(in.BaseURL)
	if s.env.URL != "" {
		in.BaseURL = ""
	} else if in.BaseURL != "" {
		u, err := url.Parse(in.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			details["base_url"] = "Une adresse complète : https://coolify.agence.fr"
		}
	}
	if len(details) == 0 && in.BaseURL != "" && s.env.URL == "" && s.env.APIToken == "" && in.APIToken == nil {
		// Le jeton enregistre appartient a l'ancienne adresse : un autre hote ne
		// doit pas le recevoir sans que l'on ait ressaisi un jeton.
		row, err := s.q.GetCoolifySettings(ctx)
		if err != nil {
			return CoolifySettingsView{}, fmt.Errorf("lecture du module : %w", err)
		}
		if len(row.ApiTokenEnc) > 0 && row.BaseUrl != "" && !sameCoolifyHost(row.BaseUrl, in.BaseURL) {
			details["api_token"] = "L’adresse a changé d’hôte : ressaisissez le jeton d’API pour cette instance"
		}
	}
	if in.PollMinutes == 0 {
		in.PollMinutes = coolifyDefaultPollMinutes
	}
	if in.PollMinutes < coolifyMinPollMinutes || in.PollMinutes > coolifyMaxPollMinutes {
		details["poll_minutes"] = "Entre 15 minutes et 24 heures (1440 minutes)"
	}
	if in.Enabled && s.env.URL == "" && in.BaseURL == "" {
		details["base_url"] = "L’adresse de Coolify est requise pour activer le module"
	}
	if len(details) > 0 {
		return CoolifySettingsView{}, domain.ErrValidation.WithDetails(details)
	}

	params := db.UpdateCoolifySettingsParams{Enabled: in.Enabled, BaseUrl: in.BaseURL, PollMinutes: int32(in.PollMinutes)}
	if in.APIToken != nil && s.env.APIToken == "" {
		params.SetApiToken = true
		if token := strings.TrimSpace(*in.APIToken); token != "" {
			sealed, err := security.Seal(s.sealKey, token)
			if err != nil {
				return CoolifySettingsView{}, fmt.Errorf("chiffrement du jeton : %w", err)
			}
			params.ApiTokenEnc = sealed
		}
	}
	if err := s.q.UpdateCoolifySettings(ctx, params); err != nil {
		return CoolifySettingsView{}, fmt.Errorf("enregistrement du module : %w", err)
	}
	// Des reglages qui changent meritent une releve tout de suite : l'ecran
	// dit aussitot si l'adresse et le jeton conviennent.
	if err := s.q.RequestCoolifyPoll(ctx); err != nil {
		return CoolifySettingsView{}, fmt.Errorf("demande de releve : %w", err)
	}
	return s.Settings(ctx)
}

// RequestPoll demande une releve immediate : c'est le « Tester la
// connexion » de l'ecran. La tache de fond la fait dans les quinze
// secondes, et l'ecran lit son resultat.
func (s *CoolifyService) RequestPoll(ctx context.Context) (CoolifySettingsView, error) {
	if err := s.q.RequestCoolifyPoll(ctx); err != nil {
		return CoolifySettingsView{}, fmt.Errorf("demande de releve : %w", err)
	}
	return s.Settings(ctx)
}

// connection rend l'adresse et le jeton a utiliser, l'environnement d'abord.
func (s *CoolifyService) connection(row db.CoolifySetting) (string, string) {
	baseURL := row.BaseUrl
	if s.env.URL != "" {
		baseURL = normalizeCoolifyURL(s.env.URL)
	}
	token := s.env.APIToken
	if token == "" && len(row.ApiTokenEnc) > 0 {
		if opened, err := security.Open(s.sealKey, row.ApiTokenEnc); err == nil {
			token = opened
		}
	}
	return baseURL, token
}

// --- Applications ------------------------------------------------------------

// CoolifyApplicationView est une application Coolify telle que la derniere
// releve l'a vue.
type CoolifyApplicationView struct {
	ID              uuid.UUID `json:"id"`
	UUID            string    `json:"uuid"`
	Name            string    `json:"name"`
	FQDN            string    `json:"fqdn"`
	EnvironmentName string    `json:"environment_name"`
	ProjectName     string    `json:"project_name"`
	Present         bool      `json:"present"`
	LastSeenAt      time.Time `json:"last_seen_at"`
}

// Applications liste les applications relevees, sans appeler Coolify.
func (s *CoolifyService) Applications(ctx context.Context) ([]CoolifyApplicationView, error) {
	rows, err := s.q.ListCoolifyApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("lecture des applications : %w", err)
	}
	out := make([]CoolifyApplicationView, 0, len(rows))
	for _, r := range rows {
		out = append(out, CoolifyApplicationView{
			ID: r.ID, UUID: r.Uuid, Name: r.Name, FQDN: r.Fqdn, EnvironmentName: r.EnvironmentName,
			ProjectName: r.ProjectName, Present: r.Present, LastSeenAt: r.LastSeenAt,
		})
	}
	return out, nil
}

// --- Releve ------------------------------------------------------------------

// PollDue dit s'il est temps de relever : l'intervalle ecoule pour un module
// actif, ou une demande de l'ecran — active ou non, pour tester avant
// d'activer.
func (s *CoolifyService) PollDue(ctx context.Context) bool {
	row, err := s.q.GetCoolifySettings(ctx)
	if err != nil {
		return false
	}
	if row.PollRequestedAt != nil {
		return true
	}
	if !row.Enabled {
		return false
	}
	return row.LastPollAt == nil || time.Since(*row.LastPollAt) >= time.Duration(row.PollMinutes)*time.Minute
}

// Poll releve la liste des applications. L'erreur d'une releve ratee
// s'affiche aussi a l'ecran.
func (s *CoolifyService) Poll(ctx context.Context) error {
	row, err := s.q.GetCoolifySettings(ctx)
	if err != nil {
		return fmt.Errorf("lecture du module : %w", err)
	}
	baseURL, token := s.connection(row)
	switch {
	case baseURL == "":
		return s.recordPoll(ctx, errors.New("adresse de Coolify manquante"))
	case token == "" && s.env.APIToken == "" && len(row.ApiTokenEnc) > 0:
		return s.recordPoll(ctx, errors.New("jeton illisible : JWT_SECRET a changé, ressaisissez-le"))
	case token == "":
		return s.recordPoll(ctx, errors.New("jeton d’API de Coolify manquant"))
	}

	provider := s.newProvider(baseURL, token)
	apps, err := provider.ListApplications(ctx)
	if err != nil {
		return s.recordPoll(ctx, err)
	}
	if err := s.storeApplications(ctx, apps); err != nil {
		return s.recordPoll(ctx, err)
	}
	// Les deploiements ne se lisent que module actif : un « Tester » avant
	// activation ne verifie que la connexion.
	if row.Enabled {
		if err := s.pollDeployments(ctx, provider, time.Now()); err != nil {
			return s.recordPoll(ctx, err)
		}
	}
	return s.recordPoll(ctx, nil)
}

// storeApplications ecrit une releve : les applications vues, et absentes
// celles qui ne le sont plus.
func (s *CoolifyService) storeApplications(ctx context.Context, apps []coolify.Application) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	seen := make([]uuid.UUID, 0, len(apps))
	for _, a := range apps {
		id, err := q.UpsertCoolifyApplication(ctx, db.UpsertCoolifyApplicationParams{
			Uuid: a.UUID, Name: coolifyName(a), Fqdn: a.FQDN, EnvironmentName: a.EnvironmentName, ProjectName: a.ProjectName,
		})
		if err != nil {
			return fmt.Errorf("enregistrement d'une application : %w", err)
		}
		seen = append(seen, id)
	}
	if err := q.MarkCoolifyApplicationsAbsent(ctx, seen); err != nil {
		return fmt.Errorf("applications absentes : %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("validation : %w", err)
	}
	return nil
}

// coolifyName garde l'application lisible quand Coolify ne donne pas de nom.
func coolifyName(a coolify.Application) string {
	if a.Name != "" {
		return a.Name
	}
	return a.UUID
}

// coolifyRecordTimeout borne l'ecriture du resultat d'une releve.
const coolifyRecordTimeout = 10 * time.Second

// coolifyAppBudget borne la lecture des deploiements d'une application : une
// application lente ne prive pas les suivantes de tout le temps du job.
const coolifyAppBudget = 2 * time.Minute

// recordPoll ecrit le resultat de la releve. Il s'ecrit avec un contexte
// detache : quand le delai de la releve est depasse, son contexte est expire
// et l'ecriture echouerait, laissant la demande en place (la releve repartirait
// a chaque tick sans rien afficher).
func (s *CoolifyService) recordPoll(ctx context.Context, pollErr error) error {
	var msg *string
	if pollErr != nil {
		text := pollErr.Error()
		switch {
		case errors.Is(pollErr, coolify.ErrUnauthorized):
			text = "Coolify refuse le jeton : vérifiez-le (Keys & Tokens → API tokens), avec son préfixe « 1| »"
		case errors.Is(pollErr, context.DeadlineExceeded):
			text = "la relève a dépassé son délai : Coolify met trop de temps à répondre"
			pollErr = errors.New(text)
		}
		msg = &text
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), coolifyRecordTimeout)
	defer cancel()
	if err := s.q.RecordCoolifyPoll(wctx, msg); err != nil {
		return fmt.Errorf("enregistrement de la releve : %w", err)
	}
	return pollErr
}

// --- Deploiements ------------------------------------------------------------

// deploymentsLookbackMonths : la releve relit le mois courant et les deux
// precedents. Un rapport en retard se genere dans ce delai ; au-dela, ce qui
// n'a pas ete releve ne l'est plus.
const deploymentsLookbackMonths = 2

// coolifySuccessStatuses sont les statuts qui disent qu'un deploiement a
// abouti, en minuscules. Coolify ne documente pas ses statuts : « finished »
// est le seul observe, et c'est ici qu'on en ajoute un.
var coolifySuccessStatuses = map[string]bool{"finished": true}

// coolifyIgnoredStatuses sont les statuts connus qui ne sont pas un succes :
// ils ne s'ecrivent pas et ne font pas de bruit. Tout autre statut est signale
// dans les logs.
var coolifyIgnoredStatuses = map[string]bool{
	"queued": true, "in_progress": true, "failed": true, "cancelled": true, "cancelled-by-user": true, "canceled": true,
}

// Ce que le rapport dit de l'environnement d'une liaison.
var coolifyEnvironmentLabel = map[string]string{"prod": "production", "preprod": "préproduction"}

// pollDeployments lit les deploiements des applications liees a un projet et
// n'ecrit que les succes. Une application en erreur n'arrete pas les autres,
// mais la releve est alors en erreur : le rapport le saura et se retiendra.
func (s *CoolifyService) pollDeployments(ctx context.Context, provider coolify.Provider, now time.Time) error {
	links, err := s.q.ListCoolifyLinks(ctx)
	if err != nil {
		return fmt.Errorf("applications liées : %w", err)
	}
	byApp := map[string][]db.ListCoolifyLinksRow{}
	var order []string
	for _, l := range links {
		if _, ok := byApp[l.ApplicationUuid]; !ok {
			order = append(order, l.ApplicationUuid)
		}
		byApp[l.ApplicationUuid] = append(byApp[l.ApplicationUuid], l)
	}

	loc := zone.Location()
	local := now.In(loc)
	since := time.Date(local.Year(), local.Month()-deploymentsLookbackMonths, 1, 0, 0, 0, 0, loc)

	var errs []error
	unknown := map[string]bool{}
	for _, appUUID := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		appCtx, cancel := context.WithTimeout(ctx, coolifyAppBudget)
		deployments, err := provider.ListDeployments(appCtx, appUUID, since)
		cancel()
		if err != nil {
			if errors.Is(err, coolify.ErrUnauthorized) || errors.Is(err, coolify.ErrForbidden) {
				return err
			}
			errs = append(errs, fmt.Errorf("application %s : %w", appUUID, err))
			continue
		}
		for _, d := range deployments {
			status := strings.ToLower(strings.TrimSpace(d.Status))
			if !coolifySuccessStatuses[status] {
				if !coolifyIgnoredStatuses[status] && !unknown[status] {
					unknown[status] = true
					s.log.Warn("statut de déploiement Coolify inconnu, non retenu", "status", d.Status, "application", appUUID)
				}
				continue
			}
			row, ok := coolifyDeploymentRow(appUUID, d)
			if !ok {
				s.log.Warn("déploiement Coolify sans date ni identifiant, non retenu", "application", appUUID)
				continue
			}
			for _, l := range byApp[appUUID] {
				if err := s.q.InsertCoolifyDeployment(ctx, db.InsertCoolifyDeploymentParams{
					ProjectID: l.ProjectID, Name: row.name, Url: firstFQDN(l.Fqdn), Environment: coolifyEnvironmentLabel[l.Environment],
					DeployedAt: row.at, ExternalID: row.externalID,
				}); err != nil {
					return fmt.Errorf("enregistrement d'un déploiement : %w", err)
				}
			}
		}
	}
	return errors.Join(errs...)
}

type coolifyDeploymentFields struct {
	name, externalID string
	at               time.Time
}

// coolifyDeploymentRow tire d'un deploiement ce que Piilot garde : l'heure de
// la mise en ligne (la fin du deploiement), un nom lisible, et un identifiant
// stable. Sans identifiant fourni, il se deduit de l'application et de l'heure
// de creation ; sans date, le deploiement n'est pas retenu.
func coolifyDeploymentRow(appUUID string, d coolify.Deployment) (coolifyDeploymentFields, bool) {
	at := d.UpdatedAt
	if at.IsZero() {
		at = d.CreatedAt
	}
	if at.IsZero() {
		return coolifyDeploymentFields{}, false
	}
	id := d.UUID
	if id == "" {
		if d.CreatedAt.IsZero() {
			return coolifyDeploymentFields{}, false
		}
		id = fmt.Sprintf("%s@%d", appUUID, d.CreatedAt.Unix())
	}
	name := strings.TrimSpace(strings.SplitN(strings.TrimSpace(d.CommitMessage), "\n", 2)[0])
	if r := []rune(name); len(r) > 120 {
		name = string(r[:120])
	}
	if name == "" {
		name = d.CommitSHA
		if len(name) > 8 {
			name = name[:8]
		}
	}
	if name == "" {
		name = "Déploiement"
	}
	return coolifyDeploymentFields{name: name, externalID: id, at: at}, true
}

// firstFQDN : Coolify separe parfois plusieurs domaines par des virgules.
func firstFQDN(fqdn string) string {
	first, _, _ := strings.Cut(fqdn, ",")
	return strings.TrimSpace(first)
}

// --- Projet ------------------------------------------------------------------

// maxProjectCoolifyApps borne les applications d'un projet.
const maxProjectCoolifyApps = 10

// ProjectCoolifyApp est une application liee a un projet.
type ProjectCoolifyApp struct {
	ApplicationUUID string `json:"application_uuid"`
	// prod ou preprod.
	Environment string `json:"environment"`
	Name        string `json:"name"`
	FQDN        string `json:"fqdn"`
	// Faux : la derniere releve ne l'a plus vue chez Coolify.
	Present bool `json:"present"`
}

// ProjectCoolifyDeployment est un deploiement releve pour un projet.
type ProjectCoolifyDeployment struct {
	Name        string    `json:"name"`
	Environment string    `json:"environment"`
	URL         string    `json:"url"`
	DeployedAt  time.Time `json:"deployed_at"`
}

// ProjectCoolifyView est la carte « Deploiements » d'un projet.
type ProjectCoolifyView struct {
	// Module inactif : l'ecran le dit, et propose d'aller l'activer.
	ModuleEnabled bool                       `json:"module_enabled"`
	Applications  []ProjectCoolifyApp        `json:"applications"`
	Deployments   []ProjectCoolifyDeployment `json:"deployments"`
	LastPollAt    *time.Time                 `json:"last_poll_at"`
	LastPollError *string                    `json:"last_poll_error"`
}

// ProjectCoolifyInput est une application a lier.
type ProjectCoolifyInput struct {
	ApplicationUUID string
	Environment     string
}

// ProjectCoolify rend la carte d'un projet, sans appeler Coolify.
func (s *CoolifyService) ProjectCoolify(ctx context.Context, projectID uuid.UUID) (ProjectCoolifyView, error) {
	exists, err := s.q.CoolifyProjectExists(ctx, projectID)
	if err != nil {
		return ProjectCoolifyView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ProjectCoolifyView{}, domain.ErrNotFound
	}
	settings, err := s.q.GetCoolifySettings(ctx)
	if err != nil {
		return ProjectCoolifyView{}, fmt.Errorf("lecture du module : %w", err)
	}
	apps, err := s.q.ListProjectCoolifyApplications(ctx, projectID)
	if err != nil {
		return ProjectCoolifyView{}, fmt.Errorf("applications du projet : %w", err)
	}
	rows, err := s.q.ListCoolifyDeploymentsOfProject(ctx, db.ListCoolifyDeploymentsOfProjectParams{ProjectID: projectID, MaxRows: 5})
	if err != nil {
		return ProjectCoolifyView{}, fmt.Errorf("déploiements du projet : %w", err)
	}
	out := ProjectCoolifyView{
		ModuleEnabled: settings.Enabled, Applications: make([]ProjectCoolifyApp, 0, len(apps)),
		Deployments: make([]ProjectCoolifyDeployment, 0, len(rows)), LastPollAt: settings.LastPollAt, LastPollError: settings.LastPollError,
	}
	for _, a := range apps {
		out.Applications = append(out.Applications, ProjectCoolifyApp{
			ApplicationUUID: a.ApplicationUuid, Environment: a.Environment, Name: a.Name, FQDN: a.Fqdn, Present: a.Present,
		})
	}
	for _, r := range rows {
		out.Deployments = append(out.Deployments, ProjectCoolifyDeployment{Name: r.Name, Environment: r.Environment, URL: r.Url, DeployedAt: r.DeployedAt})
	}
	return out, nil
}

// SetProjectApplications remplace les applications d'un projet. Une liste
// vide les detache toutes : le lien est facultatif. Une releve est demandee,
// pour que les deploiements arrivent sans attendre l'intervalle.
func (s *CoolifyService) SetProjectApplications(ctx context.Context, projectID uuid.UUID, apps []ProjectCoolifyInput) (ProjectCoolifyView, error) {
	exists, err := s.q.CoolifyProjectExists(ctx, projectID)
	if err != nil {
		return ProjectCoolifyView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ProjectCoolifyView{}, domain.ErrNotFound
	}
	if len(apps) > maxProjectCoolifyApps {
		return ProjectCoolifyView{}, domain.ErrValidation.WithDetails(map[string]any{"applications": fmt.Sprintf("%d applications au plus", maxProjectCoolifyApps)})
	}
	seen := map[string]bool{}
	uuids := make([]string, 0, len(apps))
	for i := range apps {
		apps[i].ApplicationUUID = strings.TrimSpace(apps[i].ApplicationUUID)
		a := apps[i]
		if a.Environment != "prod" && a.Environment != "preprod" {
			return ProjectCoolifyView{}, domain.ErrValidation.WithDetails(map[string]any{"environment": "prod ou preprod"})
		}
		if a.ApplicationUUID == "" || len(a.ApplicationUUID) > 100 {
			return ProjectCoolifyView{}, domain.ErrValidation.WithDetails(map[string]any{"applications": "Application invalide"})
		}
		if seen[a.ApplicationUUID] {
			return ProjectCoolifyView{}, domain.ErrValidation.WithDetails(map[string]any{"applications": "Une application ne se lie qu’une fois"})
		}
		seen[a.ApplicationUUID] = true
		uuids = append(uuids, a.ApplicationUUID)
	}
	if len(uuids) > 0 {
		found, err := s.q.CountCoolifyApplicationsByUUID(ctx, uuids)
		if err != nil {
			return ProjectCoolifyView{}, fmt.Errorf("vérification des applications : %w", err)
		}
		if int(found) != len(uuids) {
			return ProjectCoolifyView{}, domain.ErrValidation.WithDetails(map[string]any{"applications": "Application inconnue"})
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectCoolifyView{}, fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.DeleteProjectCoolifyApplications(ctx, projectID); err != nil {
		return ProjectCoolifyView{}, fmt.Errorf("liens actuels : %w", err)
	}
	for _, a := range apps {
		if err := q.InsertProjectCoolifyApplication(ctx, db.InsertProjectCoolifyApplicationParams{
			ProjectID: projectID, ApplicationUuid: a.ApplicationUUID, Environment: a.Environment,
		}); err != nil {
			return ProjectCoolifyView{}, fmt.Errorf("lien à l'application : %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectCoolifyView{}, err
	}
	if len(apps) > 0 {
		if err := s.q.RequestCoolifyPollIfNone(ctx); err != nil {
			return ProjectCoolifyView{}, fmt.Errorf("demande de relève : %w", err)
		}
	}
	return s.ProjectCoolify(ctx, projectID)
}
