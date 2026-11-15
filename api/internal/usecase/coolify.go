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
		client: &http.Client{Timeout: 20 * time.Second}, log: log,
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

	apps, err := s.newProvider(baseURL, token).ListApplications(ctx)
	if err != nil {
		return s.recordPoll(ctx, err)
	}
	if err := s.storeApplications(ctx, apps); err != nil {
		return s.recordPoll(ctx, err)
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

func (s *CoolifyService) recordPoll(ctx context.Context, pollErr error) error {
	var msg *string
	if pollErr != nil {
		text := pollErr.Error()
		if errors.Is(pollErr, coolify.ErrUnauthorized) {
			text = "Coolify refuse le jeton : vérifiez-le (Keys & Tokens → API tokens), avec son préfixe « 1| »"
		}
		msg = &text
	}
	if err := s.q.RecordCoolifyPoll(ctx, msg); err != nil {
		return fmt.Errorf("enregistrement de la releve : %w", err)
	}
	return pollErr
}
