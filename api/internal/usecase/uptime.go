package usecase

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/uptime"
)

// UptimeService est le module de disponibilite : il releve un service de
// surveillance, en tient l'historique par jour et les incidents, et relie
// ses sites aux projets.
//
// Aucun ecran n'appelle le service de surveillance : la releve est une
// tache de fond, le bouton « Tester » ne fait que la demander.
type UptimeService struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	env      config.UptimeEnv
	sealKey  []byte
	hookBase string
	client   *http.Client
	log      *slog.Logger
}

// ErrUptimeForbidden : le jeton du webhook ne correspond pas.
var ErrUptimeForbidden = &domain.Error{Status: http.StatusUnauthorized, Code: "INVALID_TOKEN", Message: "Jeton du webhook invalide"}

const (
	// uptimeProvider : le seul fournisseur pour l'instant.
	uptimeProvider = "kuma"
	// uptimeWindowDays : la fenetre de disponibilite affichee.
	uptimeWindowDays = 30
	// maxProjectSites borne les sites d'un projet.
	maxProjectSites = 10
)

// NewUptimeService construit le service.
func NewUptimeService(pool *pgxpool.Pool, env config.UptimeEnv, secret []byte, hookBase string, log *slog.Logger) *UptimeService {
	return &UptimeService{
		pool: pool, q: db.New(pool), env: env, sealKey: security.DeriveKey(secret, "uptime-api-key"),
		hookBase: strings.TrimRight(hookBase, "/"), client: &http.Client{Timeout: 20 * time.Second}, log: log,
	}
}

// --- Modules -----------------------------------------------------------------

// ModuleView est une carte de la page des modules.
type ModuleView struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	// off, pending (active, pas encore releve), ok ou error.
	State string `json:"state"`
}

// Modules liste les modules activables. Un seul aujourd'hui.
func (s *UptimeService) Modules(ctx context.Context) ([]ModuleView, error) {
	row, err := s.q.GetUptimeSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("lecture du module : %w", err)
	}
	return []ModuleView{{
		Code: "uptime", Name: "Disponibilité des sites", Provider: "Uptime Kuma",
		Description: "Relève l’état des sites surveillés par Uptime Kuma, en garde l’historique et les incidents, et les relie aux projets.",
		Enabled:     row.Enabled, State: moduleState(row),
	}}, nil
}

func moduleState(row db.UptimeSetting) string {
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

// UptimeSettingsView est l'ecran du module. La cle d'API n'en sort jamais.
type UptimeSettingsView struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`
	BaseURLFromEnv bool   `json:"base_url_from_env"`
	APIKeySet      bool   `json:"api_key_set"`
	APIKeyFromEnv  bool   `json:"api_key_from_env"`
	PollMinutes    int    `json:"poll_minutes"`
	// L'adresse a coller dans Kuma comme notification « Webhook ».
	WebhookURL string           `json:"webhook_url"`
	Status     UptimeStatusView `json:"status"`
}

// UptimeStatusView dit ou en est la releve.
type UptimeStatusView struct {
	State         string     `json:"state"`
	LastPollAt    *time.Time `json:"last_poll_at"`
	LastPollError *string    `json:"last_poll_error"`
	PollPending   bool       `json:"poll_pending"`
	WebhookLastAt *time.Time `json:"webhook_last_at"`
	Monitors      int        `json:"monitors"`
	Up            int        `json:"up"`
	Down          int        `json:"down"`
}

// UptimeSettingsInput est ce que l'ecran enregistre. Une cle nulle garde
// celle qui est en base ; une chaine vide l'efface.
type UptimeSettingsInput struct {
	Enabled     bool
	BaseURL     string
	APIKey      *string
	PollMinutes int
}

// Settings rend l'ecran du module.
func (s *UptimeService) Settings(ctx context.Context) (UptimeSettingsView, error) {
	row, err := s.q.GetUptimeSettings(ctx)
	if err != nil {
		return UptimeSettingsView{}, fmt.Errorf("lecture du module : %w", err)
	}
	secret, err := s.webhookSecret(ctx, row)
	if err != nil {
		return UptimeSettingsView{}, err
	}
	counts, err := s.q.CountUptimeMonitorsByState(ctx)
	if err != nil {
		return UptimeSettingsView{}, fmt.Errorf("compte des sites : %w", err)
	}

	baseURL, _ := s.connection(row)
	return UptimeSettingsView{
		Enabled: row.Enabled, Provider: row.Provider, BaseURL: baseURL, BaseURLFromEnv: s.env.KumaURL != "",
		APIKeySet: s.env.KumaAPIKey != "" || len(row.ApiKeyEnc) > 0, APIKeyFromEnv: s.env.KumaAPIKey != "",
		PollMinutes: int(row.PollMinutes),
		WebhookURL:  fmt.Sprintf("%s/api/v1/hooks/uptime/%s?token=%s", s.hookBase, row.Provider, url.QueryEscape(secret)),
		Status: UptimeStatusView{
			State: moduleState(row), LastPollAt: row.LastPollAt, LastPollError: row.LastPollError,
			PollPending: row.PollRequestedAt != nil, WebhookLastAt: row.WebhookLastAt,
			Monitors: int(counts.Present), Up: int(counts.Up), Down: int(counts.Down),
		},
	}, nil
}

// UpdateSettings enregistre les reglages. Ce que l'environnement fixe n'est
// pas modifiable ici.
func (s *UptimeService) UpdateSettings(ctx context.Context, in UptimeSettingsInput) (UptimeSettingsView, error) {
	details := map[string]any{}
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	in.BaseURL = strings.TrimSuffix(in.BaseURL, "/metrics")
	if s.env.KumaURL != "" {
		in.BaseURL = ""
	} else if in.BaseURL != "" {
		u, err := url.Parse(in.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			details["base_url"] = "Une adresse complète : https://kuma.agence.fr"
		}
	}
	if in.PollMinutes == 0 {
		in.PollMinutes = 2
	}
	if in.PollMinutes < 1 || in.PollMinutes > 5 {
		details["poll_minutes"] = "Entre 1 et 5 minutes"
	}
	if in.Enabled && s.env.KumaURL == "" && in.BaseURL == "" {
		details["base_url"] = "L’adresse de Kuma est requise pour activer le module"
	}
	if len(details) > 0 {
		return UptimeSettingsView{}, domain.ErrValidation.WithDetails(details)
	}

	params := db.UpdateUptimeSettingsParams{Enabled: in.Enabled, BaseUrl: in.BaseURL, PollMinutes: int32(in.PollMinutes)}
	if in.APIKey != nil && s.env.KumaAPIKey == "" {
		params.SetApiKey = true
		if key := strings.TrimSpace(*in.APIKey); key != "" {
			sealed, err := security.Seal(s.sealKey, key)
			if err != nil {
				return UptimeSettingsView{}, fmt.Errorf("chiffrement de la clé : %w", err)
			}
			params.ApiKeyEnc = sealed
		}
	}
	if err := s.q.UpdateUptimeSettings(ctx, params); err != nil {
		return UptimeSettingsView{}, fmt.Errorf("enregistrement du module : %w", err)
	}
	// Des reglages qui changent meritent une releve tout de suite : l'ecran
	// dit aussitot si l'adresse et la cle conviennent.
	if err := s.q.RequestUptimePoll(ctx); err != nil {
		return UptimeSettingsView{}, fmt.Errorf("demande de releve : %w", err)
	}
	return s.Settings(ctx)
}

// RotateWebhookSecret tire un nouveau secret : le webhook configure dans
// Kuma avec l'ancien cesse d'etre accepte.
func (s *UptimeService) RotateWebhookSecret(ctx context.Context) (UptimeSettingsView, error) {
	raw, _, err := security.NewOneTimeToken()
	if err != nil {
		return UptimeSettingsView{}, err
	}
	if err := s.q.SetUptimeWebhookSecret(ctx, raw); err != nil {
		return UptimeSettingsView{}, fmt.Errorf("enregistrement du secret : %w", err)
	}
	return s.Settings(ctx)
}

// RequestPoll demande une releve immediate : c'est le « Tester la
// connexion » de l'ecran. La tache de fond la fait dans les quinze
// secondes, et l'ecran lit son resultat.
func (s *UptimeService) RequestPoll(ctx context.Context) (UptimeSettingsView, error) {
	if err := s.q.RequestUptimePoll(ctx); err != nil {
		return UptimeSettingsView{}, fmt.Errorf("demande de releve : %w", err)
	}
	return s.Settings(ctx)
}

func (s *UptimeService) webhookSecret(ctx context.Context, row db.UptimeSetting) (string, error) {
	if row.WebhookSecret != "" {
		return row.WebhookSecret, nil
	}
	raw, _, err := security.NewOneTimeToken()
	if err != nil {
		return "", err
	}
	if err := s.q.SetUptimeWebhookSecret(ctx, raw); err != nil {
		return "", fmt.Errorf("enregistrement du secret : %w", err)
	}
	return raw, nil
}

// connection rend l'adresse et la cle a utiliser, l'environnement d'abord.
func (s *UptimeService) connection(row db.UptimeSetting) (string, string) {
	baseURL := row.BaseUrl
	if s.env.KumaURL != "" {
		baseURL = strings.TrimSuffix(strings.TrimRight(s.env.KumaURL, "/"), "/metrics")
	}
	key := s.env.KumaAPIKey
	if key == "" && len(row.ApiKeyEnc) > 0 {
		if opened, err := security.Open(s.sealKey, row.ApiKeyEnc); err == nil {
			key = opened
		}
	}
	return baseURL, key
}

// --- Releve ------------------------------------------------------------------

// PollDue dit s'il est temps de relever : l'intervalle ecoule pour un module
// actif, ou une demande de l'ecran — active ou non, pour tester avant
// d'activer.
func (s *UptimeService) PollDue(ctx context.Context) bool {
	row, err := s.q.GetUptimeSettings(ctx)
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

// Poll releve le service et met l'historique a jour. Une releve ratee
// compte un controle manquant par site, et son erreur s'affiche a l'ecran.
func (s *UptimeService) Poll(ctx context.Context) error {
	return s.pollAt(ctx, time.Now())
}

func (s *UptimeService) pollAt(ctx context.Context, now time.Time) error {
	row, err := s.q.GetUptimeSettings(ctx)
	if err != nil {
		return fmt.Errorf("lecture du module : %w", err)
	}
	day := agencyToday(now)

	baseURL, key := s.connection(row)
	if baseURL == "" {
		return s.recordPoll(ctx, errors.New("adresse de Kuma manquante"))
	}
	if s.env.KumaAPIKey == "" && len(row.ApiKeyEnc) > 0 && key == "" {
		return s.recordPoll(ctx, errors.New("clé d’API illisible : JWT_SECRET a changé, ressaisissez-la"))
	}

	provider := &uptime.Kuma{BaseURL: baseURL, APIKey: key, Client: s.client}
	monitors, err := provider.ListMonitors(ctx)
	if err != nil {
		if row.Enabled {
			if berr := s.q.BumpUptimeMissing(ctx, db.BumpUptimeMissingParams{Day: day, Provider: uptimeProvider}); berr != nil {
				s.log.Warn("controles manquants non comptes", "err", berr)
			}
		}
		return s.recordPoll(ctx, err)
	}

	if err := s.store(ctx, monitors, day, row.Enabled); err != nil {
		return s.recordPoll(ctx, err)
	}
	return s.recordPoll(ctx, nil)
}

// store ecrit une releve : les sites, et, module actif, un controle de plus
// par site dans l'agregat du jour et les incidents deduits.
func (s *UptimeService) store(ctx context.Context, monitors []uptime.Monitor, day time.Time, enabled bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	ids := make([]uuid.UUID, 0, len(monitors))
	statuses := make([]int32, 0, len(monitors))
	pings := make([]int32, 0, len(monitors))
	for _, m := range monitors {
		id, err := q.UpsertUptimeMonitor(ctx, db.UpsertUptimeMonitorParams{
			Provider: uptimeProvider, ExternalKey: m.Key, Name: m.Name, Kind: m.Type, Url: m.URL, Hostname: m.Hostname,
			Status: int16(m.Status), ResponseMs: int32Ptr(m.ResponseMS), CertDays: int32Ptr(m.CertDays), CertValid: m.CertValid,
		})
		if err != nil {
			return fmt.Errorf("site %s : %w", m.Name, err)
		}
		ids = append(ids, id)
		statuses = append(statuses, int32(m.Status))
		ping := int32(-1)
		if m.ResponseMS != nil {
			ping = int32(*m.ResponseMS)
		}
		pings = append(pings, ping)
	}
	if err := q.MarkUptimeMonitorsAbsent(ctx, db.MarkUptimeMonitorsAbsentParams{Provider: uptimeProvider, Seen: ids}); err != nil {
		return fmt.Errorf("sites disparus : %w", err)
	}

	if enabled {
		if len(ids) > 0 {
			if err := q.BumpUptimeDaily(ctx, db.BumpUptimeDailyParams{Day: day, MonitorIds: ids, Statuses: statuses, Pings: pings}); err != nil {
				return fmt.Errorf("agregat du jour : %w", err)
			}
		}
		// Les incidents deduits de l'etat : le webhook, s'il est branche, les
		// precise (heure exacte, raison) ; sans lui, ils existent quand meme.
		if err := q.OpenIncidentsFromPoll(ctx, uptimeProvider); err != nil {
			return fmt.Errorf("incidents ouverts : %w", err)
		}
		if err := q.CloseIncidentsFromPoll(ctx, uptimeProvider); err != nil {
			return fmt.Errorf("incidents clos : %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (s *UptimeService) recordPoll(ctx context.Context, pollErr error) error {
	var msg *string
	if pollErr != nil {
		text := pollErr.Error()
		if errors.Is(pollErr, uptime.ErrUnauthorized) {
			text = "Kuma refuse la clé d’API : vérifiez-la dans Paramètres → Clés API de Kuma"
		}
		msg = &text
	}
	if err := s.q.RecordUptimePoll(ctx, msg); err != nil {
		return fmt.Errorf("enregistrement de la releve : %w", err)
	}
	return pollErr
}

func int32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	n := int32(*v)
	return &n
}

// --- Webhook -----------------------------------------------------------------

// UptimeWebhookResult dit ce que le webhook a fait de l'evenement.
type UptimeWebhookResult struct {
	// recorded, ignored (essai, ou site pas encore releve).
	Outcome string `json:"outcome"`
}

// Webhook recoit un changement d'etat pousse par le service. Le jeton de
// l'adresse fait office de preuve.
func (s *UptimeService) Webhook(ctx context.Context, provider, token string, body []byte) (UptimeWebhookResult, error) {
	row, err := s.q.GetUptimeSettings(ctx)
	if err != nil {
		return UptimeWebhookResult{}, fmt.Errorf("lecture du module : %w", err)
	}
	if row.WebhookSecret == "" || subtle.ConstantTimeCompare([]byte(row.WebhookSecret), []byte(token)) != 1 {
		return UptimeWebhookResult{}, ErrUptimeForbidden
	}
	if provider != row.Provider {
		return UptimeWebhookResult{}, domain.ErrNotFound
	}
	if !row.Enabled {
		return UptimeWebhookResult{Outcome: "ignored"}, nil
	}

	ev, ok, err := (&uptime.Kuma{}).ParseEvent(body)
	if err != nil {
		return UptimeWebhookResult{}, domain.ErrValidation.WithMessage(err.Error())
	}
	if err := s.q.TouchUptimeWebhook(ctx); err != nil {
		return UptimeWebhookResult{}, fmt.Errorf("horodatage du webhook : %w", err)
	}
	if !ok {
		return UptimeWebhookResult{Outcome: "ignored"}, nil
	}

	monitor, err := s.q.FindUptimeMonitorByKeys(ctx, db.FindUptimeMonitorByKeysParams{Provider: uptimeProvider, Keys: ev.Keys})
	if errors.Is(err, pgx.ErrNoRows) {
		// Un site cree apres la derniere releve : la prochaine le prendra.
		return UptimeWebhookResult{Outcome: "ignored"}, nil
	}
	if err != nil {
		return UptimeWebhookResult{}, fmt.Errorf("site de l'evenement : %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return UptimeWebhookResult{}, fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if err := q.SetUptimeMonitorStatus(ctx, db.SetUptimeMonitorStatusParams{ID: monitor.ID, Status: int16(ev.Status), At: ev.At}); err != nil {
		return UptimeWebhookResult{}, fmt.Errorf("etat du site : %w", err)
	}
	switch ev.Status {
	case uptime.Down:
		err = q.OpenUptimeIncident(ctx, db.OpenUptimeIncidentParams{MonitorID: monitor.ID, StartedAt: ev.At, Message: truncate(ev.Message, 500)})
	case uptime.Up:
		err = q.CloseUptimeIncident(ctx, db.CloseUptimeIncidentParams{MonitorID: monitor.ID, EndedAt: &ev.At})
	}
	if err != nil {
		return UptimeWebhookResult{}, fmt.Errorf("incident : %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return UptimeWebhookResult{}, err
	}
	return UptimeWebhookResult{Outcome: "recorded"}, nil
}

// --- Sites et incidents --------------------------------------------------------

// UptimeMonitorView est un site et sa disponibilite sur trente jours.
type UptimeMonitorView struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	URL      string    `json:"url"`
	Hostname string    `json:"hostname"`
	// up, down, pending ou maintenance.
	Status          string    `json:"status"`
	ResponseMS      *int32    `json:"response_ms"`
	CertDays        *int32    `json:"cert_days"`
	CertValid       *bool     `json:"cert_valid"`
	Present         bool      `json:"present"`
	StatusChangedAt time.Time `json:"status_changed_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	// Disponibilite sur trente jours, en pourcentage ; nulle sans controle
	// concluant. Les releves ratees n'y entrent pas.
	Uptime30d *float64 `json:"uptime_30d"`
	AvgPing   *int     `json:"avg_ping_30d"`
	Projects  []string `json:"projects"`
}

// UptimeMonitorPage est une page de sites.
type UptimeMonitorPage struct {
	Items []UptimeMonitorView `json:"items"`
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
}

var uptimeStatusName = map[int16]string{0: "down", 1: "up", 2: "pending", 3: "maintenance"}

// uptimeRatio : up / (up + down), en pourcentage a deux decimales.
func uptimeRatio(up, down int32) *float64 {
	if up+down == 0 {
		return nil
	}
	v := math.Round(float64(up)/float64(up+down)*10000) / 100
	return &v
}

// Monitors rend une page de sites, filtree par nom ou adresse.
func (s *UptimeService) Monitors(ctx context.Context, query string, page, pageSize int) (UptimeMonitorPage, error) {
	page, pageSize = clampPage(page, pageSize)
	query = strings.TrimSpace(query)
	total, err := s.q.CountUptimeMonitors(ctx, query)
	if err != nil {
		return UptimeMonitorPage{}, fmt.Errorf("compte des sites : %w", err)
	}
	rows, err := s.q.ListUptimeMonitors(ctx, db.ListUptimeMonitorsParams{
		Since: agencyToday(time.Now()).AddDate(0, 0, -(uptimeWindowDays - 1)), Q: query,
		Lim: int32(pageSize), Off: int32((page - 1) * pageSize),
	})
	if err != nil {
		return UptimeMonitorPage{}, fmt.Errorf("liste des sites : %w", err)
	}
	out := UptimeMonitorPage{Items: make([]UptimeMonitorView, 0, len(rows)), Total: int(total), Page: page}
	for _, r := range rows {
		v := UptimeMonitorView{
			ID: r.ID, Name: r.Name, Kind: r.Kind, URL: r.Url, Hostname: r.Hostname, Status: uptimeStatusName[r.Status],
			ResponseMS: r.ResponseMs, CertDays: r.CertDays, CertValid: r.CertValid, Present: r.Present,
			StatusChangedAt: r.StatusChangedAt, LastSeenAt: r.LastSeenAt,
			Uptime30d: uptimeRatio(r.WindowUp, r.WindowDown), Projects: r.Projects,
		}
		if r.WindowPingCount > 0 {
			avg := int(r.WindowPingSum / int64(r.WindowPingCount))
			v.AvgPing = &avg
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

// UptimeIncidentView est une panne.
type UptimeIncidentView struct {
	ID          uuid.UUID  `json:"id"`
	MonitorID   uuid.UUID  `json:"monitor_id"`
	MonitorName string     `json:"monitor_name"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	// Duree en secondes, nulle tant que la panne dure.
	DurationSeconds *int32 `json:"duration_seconds"`
	Message         string `json:"message"`
	Source          string `json:"source"`
}

// UptimeIncidentPage est une page d'incidents.
type UptimeIncidentPage struct {
	Items []UptimeIncidentView `json:"items"`
	Total int                  `json:"total"`
	Page  int                  `json:"page"`
}

// Incidents rend une page d'incidents, tous sites confondus.
func (s *UptimeService) Incidents(ctx context.Context, page, pageSize int) (UptimeIncidentPage, error) {
	return s.incidents(ctx, nil, page, pageSize)
}

func (s *UptimeService) incidents(ctx context.Context, monitorIDs []uuid.UUID, page, pageSize int) (UptimeIncidentPage, error) {
	page, pageSize = clampPage(page, pageSize)
	total, err := s.q.CountUptimeIncidents(ctx, monitorIDs)
	if err != nil {
		return UptimeIncidentPage{}, fmt.Errorf("compte des incidents : %w", err)
	}
	rows, err := s.q.ListUptimeIncidents(ctx, db.ListUptimeIncidentsParams{MonitorIds: monitorIDs, Lim: int32(pageSize), Off: int32((page - 1) * pageSize)})
	if err != nil {
		return UptimeIncidentPage{}, fmt.Errorf("liste des incidents : %w", err)
	}
	out := UptimeIncidentPage{Items: make([]UptimeIncidentView, 0, len(rows)), Total: int(total), Page: page}
	for _, r := range rows {
		out.Items = append(out.Items, UptimeIncidentView{
			ID: r.ID, MonitorID: r.MonitorID, MonitorName: r.MonitorName, StartedAt: r.StartedAt, EndedAt: r.EndedAt,
			DurationSeconds: r.DurationSeconds, Message: r.Message, Source: r.Source,
		})
	}
	return out, nil
}

// --- Projets -----------------------------------------------------------------

// ProjectUptimeView est l'onglet « Disponibilite » d'un projet.
type ProjectUptimeView struct {
	// Module inactif : l'ecran le dit, et propose d'aller l'activer.
	ModuleEnabled bool                `json:"module_enabled"`
	Sites         []ProjectUptimeSite `json:"sites"`
	// Les cinq derniers incidents de ses sites.
	Incidents []UptimeIncidentView `json:"incidents"`
}

// ProjectUptimeSite est un site du projet.
type ProjectUptimeSite struct {
	// prod ou preprod.
	Environment     string    `json:"environment"`
	MonitorID       uuid.UUID `json:"monitor_id"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	Status          string    `json:"status"`
	ResponseMS      *int32    `json:"response_ms"`
	CertDays        *int32    `json:"cert_days"`
	CertValid       *bool     `json:"cert_valid"`
	Present         bool      `json:"present"`
	StatusChangedAt time.Time `json:"status_changed_at"`
	Uptime30d       *float64  `json:"uptime_30d"`
	// Trente jours, du plus ancien a aujourd'hui : la disponibilite du jour,
	// nulle sans controle concluant.
	Days []UptimeDay `json:"days"`
}

// UptimeDay est une case de la frise.
type UptimeDay struct {
	Day    string   `json:"day"`
	Uptime *float64 `json:"uptime"`
}

// ProjectSiteInput est un lien demande par l'ecran.
type ProjectSiteInput struct {
	MonitorID   uuid.UUID
	Environment string
}

// ProjectUptime rend l'onglet d'un projet.
func (s *UptimeService) ProjectUptime(ctx context.Context, projectID uuid.UUID) (ProjectUptimeView, error) {
	exists, err := s.q.UptimeProjectExists(ctx, projectID)
	if err != nil {
		return ProjectUptimeView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ProjectUptimeView{}, domain.ErrNotFound
	}
	settings, err := s.q.GetUptimeSettings(ctx)
	if err != nil {
		return ProjectUptimeView{}, fmt.Errorf("lecture du module : %w", err)
	}

	today := agencyToday(time.Now())
	since := today.AddDate(0, 0, -(uptimeWindowDays - 1))
	rows, err := s.q.ListProjectUptimeMonitors(ctx, db.ListProjectUptimeMonitorsParams{ProjectID: projectID, Since: since})
	if err != nil {
		return ProjectUptimeView{}, fmt.Errorf("sites du projet : %w", err)
	}
	days, err := s.q.ListProjectUptimeDays(ctx, db.ListProjectUptimeDaysParams{ProjectID: projectID, Since: since})
	if err != nil {
		return ProjectUptimeView{}, fmt.Errorf("frise du projet : %w", err)
	}
	byDay := map[uuid.UUID]map[string]*float64{}
	for _, d := range days {
		if byDay[d.MonitorID] == nil {
			byDay[d.MonitorID] = map[string]*float64{}
		}
		byDay[d.MonitorID][d.Day.Format(dateLayout)] = uptimeRatio(d.ChecksUp, d.ChecksDown)
	}

	out := ProjectUptimeView{ModuleEnabled: settings.Enabled, Sites: make([]ProjectUptimeSite, 0, len(rows)), Incidents: []UptimeIncidentView{}}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		site := ProjectUptimeSite{
			Environment: r.Environment, MonitorID: r.ID, Name: r.Name, URL: r.Url, Status: uptimeStatusName[r.Status],
			ResponseMS: r.ResponseMs, CertDays: r.CertDays, CertValid: r.CertValid, Present: r.Present,
			StatusChangedAt: r.StatusChangedAt, Uptime30d: uptimeRatio(r.WindowUp, r.WindowDown),
			Days: make([]UptimeDay, 0, uptimeWindowDays),
		}
		for d := since; !d.After(today); d = d.AddDate(0, 0, 1) {
			key := d.Format(dateLayout)
			site.Days = append(site.Days, UptimeDay{Day: key, Uptime: byDay[r.ID][key]})
		}
		out.Sites = append(out.Sites, site)
		ids = append(ids, r.ID)
	}

	if len(ids) > 0 {
		page, err := s.incidents(ctx, ids, 1, 5)
		if err != nil {
			return ProjectUptimeView{}, err
		}
		out.Incidents = page.Items
	}
	return out, nil
}

// SetProjectSites remplace les sites d'un projet. Une liste vide les
// detache tous : le lien est facultatif.
func (s *UptimeService) SetProjectSites(ctx context.Context, projectID uuid.UUID, sites []ProjectSiteInput) (ProjectUptimeView, error) {
	exists, err := s.q.UptimeProjectExists(ctx, projectID)
	if err != nil {
		return ProjectUptimeView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ProjectUptimeView{}, domain.ErrNotFound
	}
	if len(sites) > maxProjectSites {
		return ProjectUptimeView{}, domain.ErrValidation.WithDetails(map[string]any{"sites": fmt.Sprintf("%d sites au plus", maxProjectSites)})
	}

	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(sites))
	for _, site := range sites {
		if site.Environment != "prod" && site.Environment != "preprod" {
			return ProjectUptimeView{}, domain.ErrValidation.WithDetails(map[string]any{"environment": "prod ou preprod"})
		}
		if seen[site.MonitorID] {
			return ProjectUptimeView{}, domain.ErrValidation.WithDetails(map[string]any{"sites": "Un site ne se lie qu’une fois"})
		}
		seen[site.MonitorID] = true
		ids = append(ids, site.MonitorID)
	}
	if len(ids) > 0 {
		found, err := s.q.UptimeMonitorsExist(ctx, ids)
		if err != nil {
			return ProjectUptimeView{}, fmt.Errorf("verification des sites : %w", err)
		}
		if int(found) != len(ids) {
			return ProjectUptimeView{}, domain.ErrValidation.WithDetails(map[string]any{"sites": "Site inconnu"})
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectUptimeView{}, fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.DeleteProjectUptimeMonitors(ctx, projectID); err != nil {
		return ProjectUptimeView{}, fmt.Errorf("liens actuels : %w", err)
	}
	for _, site := range sites {
		if err := q.InsertProjectUptimeMonitor(ctx, db.InsertProjectUptimeMonitorParams{
			ProjectID: projectID, MonitorID: site.MonitorID, Environment: site.Environment,
		}); err != nil {
			return ProjectUptimeView{}, fmt.Errorf("lien au site : %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectUptimeView{}, err
	}
	return s.ProjectUptime(ctx, projectID)
}

// PortalUptimeSite est la disponibilite d'un site de production d'un projet
// du client, sur trente jours, pour le portail.
type PortalUptimeSite struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	// Nul tant qu'aucun controle n'a abouti sur la periode.
	Uptime *float64 `json:"uptime_30d"`
}

// PortalUptime rend la disponibilite des sites de production du client : vide
// quand le module est coupe ou qu'aucun site n'est lie.
func (s *UptimeService) PortalUptime(ctx context.Context, userID uuid.UUID) ([]PortalUptimeSite, error) {
	out := []PortalUptimeSite{}
	clientID, err := s.q.PortalClientOf(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && clientID == nil) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lecture du compte : %w", err)
	}
	since := agencyToday(time.Now()).AddDate(0, 0, -(uptimeWindowDays - 1))
	rows, err := s.q.PortalUptimeSites(ctx, db.PortalUptimeSitesParams{Since: since, ClientID: *clientID})
	if err != nil {
		return nil, fmt.Errorf("sites surveilles du client : %w", err)
	}
	for _, r := range rows {
		out = append(out, PortalUptimeSite{
			ID: r.ID, ProjectID: r.ProjectID, Name: r.Name, URL: r.Url, Status: uptimeStatusName[r.Status],
			Uptime: uptimeRatio(int32(r.ChecksUp), int32(r.ChecksDown)),
		})
	}
	return out, nil
}
