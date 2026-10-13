package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/analytics"
	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/zone"
)

// AnalyticsService est le module de statistiques : il lit un service de
// mesure d'audience (Umami), en garde les vues affichees dans un cache local
// que remplit une tache de fond, et relie ses sites aux projets.
//
// Aucun ecran n'appelle le service. Une vue absente du cache — une periode
// personnalisee, une periode jamais demandee — devient une demande que la
// tache sert dans les secondes qui suivent ; l'ecran attend en relisant.
type AnalyticsService struct {
	pool    *pgxpool.Pool
	q       *db.Queries
	env     config.AnalyticsEnv
	sealKey []byte
	client  *http.Client
	log     *slog.Logger

	// Le fournisseur garde son jeton en memoire d'une releve a l'autre ; il
	// se reconstruit quand les reglages changent.
	mu          sync.Mutex
	umami       *analytics.Umami
	fingerprint string
	now         func() time.Time
}

const (
	analyticsProvider = "umami"
	// Les vues lourdes ou figees se rafraichissent une fois par jour.
	analyticsSlowTTL = 24 * time.Hour
	// Une vue courante qu'un ecran a lue dans ce delai reste tenue a jour ;
	// au-dela, elle attend qu'on la redemande.
	analyticsReadWindow = 3 * 24 * time.Hour
	// Vues rafraichies par passage de la tache : la charge sur le service
	// s'etale au lieu de tomber d'un coup.
	analyticsRefreshPerTick = 6
	// Une periode personnalisee couvre deux ans au plus.
	analyticsMaxMonths = 24
	// Demandes de periodes personnalisees par compte et par heure.
	analyticsRequestsPerHour = 20
	// Lignes par classement.
	analyticsTopLimit = 10
	// La synchronisation des sites, faute de demande, une fois par heure.
	analyticsCheckEvery = time.Hour
)

// NewAnalyticsService construit le service.
func NewAnalyticsService(pool *pgxpool.Pool, env config.AnalyticsEnv, secret []byte, log *slog.Logger) *AnalyticsService {
	return &AnalyticsService{
		pool: pool, q: db.New(pool), env: env, sealKey: security.DeriveKey(secret, "analytics-secrets"),
		client: &http.Client{Timeout: 20 * time.Second}, log: log, now: time.Now,
	}
}

// --- Modules -----------------------------------------------------------------

// Modules rend la carte du module pour la page des modules.
func (s *AnalyticsService) Modules(ctx context.Context) ([]ModuleView, error) {
	row, err := s.q.GetAnalyticsSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("lecture du module : %w", err)
	}
	return []ModuleView{{
		Code: "analytics", Name: "Statistiques des sites", Provider: "Umami",
		Description: "Montre aux clients l’audience de leur site : visiteurs, pages vues, sources, pays et appareils, lus dans Umami.",
		Enabled:     row.Enabled, State: analyticsState(row),
	}}, nil
}

func analyticsState(row db.AnalyticsSetting) string {
	switch {
	case !row.Enabled:
		return "off"
	case row.LastCheckError != nil:
		return "error"
	case row.LastCheckAt == nil:
		return "pending"
	default:
		return "ok"
	}
}

// --- Reglages ----------------------------------------------------------------

// AnalyticsSettingsView est l'ecran du module. Aucun secret n'en sort.
type AnalyticsSettingsView struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`
	BaseURLFromEnv bool   `json:"base_url_from_env"`
	// login ou api_key.
	AuthMode    string `json:"auth_mode"`
	AuthFromEnv bool   `json:"auth_from_env"`
	Username    string `json:"username"`
	PasswordSet bool   `json:"password_set"`
	APIKeySet   bool   `json:"api_key_set"`
	// Version lue ou choisie : 0 inconnue, 2 ou 3.
	APIVersion    int                 `json:"api_version"`
	VersionPinned bool                `json:"version_pinned"`
	CacheMinutes  int                 `json:"cache_minutes"`
	Status        AnalyticsStatusView `json:"status"`
}

// AnalyticsStatusView dit ou en est le module.
type AnalyticsStatusView struct {
	State          string     `json:"state"`
	LastCheckAt    *time.Time `json:"last_check_at"`
	LastCheckError *string    `json:"last_check_error"`
	LastCheckNote  *string    `json:"last_check_note"`
	CheckPending   bool       `json:"check_pending"`
	Websites       int        `json:"websites"`
	Linked         int        `json:"linked"`
}

// AnalyticsSettingsInput est ce que l'ecran enregistre. Un secret nul garde
// celui en base ; une chaine vide l'efface. APIVersion a 0 laisse la
// detection faire ; 2 ou 3 la fige.
type AnalyticsSettingsInput struct {
	Enabled      bool
	BaseURL      string
	AuthMode     string
	Username     string
	Password     *string
	APIKey       *string
	APIVersion   int
	CacheMinutes int
}

// Settings rend l'ecran du module.
func (s *AnalyticsService) Settings(ctx context.Context) (AnalyticsSettingsView, error) {
	row, err := s.q.GetAnalyticsSettings(ctx)
	if err != nil {
		return AnalyticsSettingsView{}, fmt.Errorf("lecture du module : %w", err)
	}
	counts, err := s.q.CountAnalyticsWebsitesPresent(ctx)
	if err != nil {
		return AnalyticsSettingsView{}, fmt.Errorf("compte des sites : %w", err)
	}
	c := s.connection(row)
	return AnalyticsSettingsView{
		Enabled: row.Enabled, Provider: row.Provider, BaseURL: c.baseURL, BaseURLFromEnv: s.env.UmamiURL != "",
		AuthMode: c.mode, AuthFromEnv: s.env.AuthFromEnv(), Username: c.username,
		PasswordSet: s.env.UmamiPassword != "" || len(row.PasswordEnc) > 0, APIKeySet: s.env.UmamiAPIKey != "" || len(row.ApiKeyEnc) > 0,
		APIVersion: int(row.ApiVersion), VersionPinned: row.VersionPinned, CacheMinutes: int(row.CacheMinutes),
		Status: AnalyticsStatusView{
			State: analyticsState(row), LastCheckAt: row.LastCheckAt, LastCheckError: row.LastCheckError,
			LastCheckNote: row.LastCheckNote, CheckPending: row.CheckRequestedAt != nil,
			Websites: int(counts.Present), Linked: int(counts.Linked),
		},
	}, nil
}

// UpdateSettings enregistre les reglages, puis demande une verification.
func (s *AnalyticsService) UpdateSettings(ctx context.Context, in AnalyticsSettingsInput) (AnalyticsSettingsView, error) {
	row, err := s.q.GetAnalyticsSettings(ctx)
	if err != nil {
		return AnalyticsSettingsView{}, fmt.Errorf("lecture du module : %w", err)
	}
	details := map[string]any{}

	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if s.env.UmamiURL != "" {
		in.BaseURL = ""
	} else if in.BaseURL != "" {
		u, err := url.Parse(in.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			details["base_url"] = "Une adresse complète : https://stats.agence.fr, ou https://api.umami.is/v1 pour Umami Cloud"
		}
	}
	if s.env.AuthFromEnv() {
		in.AuthMode, in.Username, in.Password, in.APIKey = row.AuthMode, row.Username, nil, nil
	}
	if in.AuthMode != "login" && in.AuthMode != "api_key" {
		details["auth_mode"] = "login ou api_key"
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.APIVersion != 0 && in.APIVersion != 2 && in.APIVersion != 3 {
		details["api_version"] = "Automatique, 2 ou 3"
	}
	if in.CacheMinutes == 0 {
		in.CacheMinutes = 10
	}
	if in.CacheMinutes < 5 || in.CacheMinutes > 15 {
		details["cache_minutes"] = "Entre 5 et 15 minutes"
	}
	if in.Enabled && s.env.UmamiURL == "" && in.BaseURL == "" {
		details["base_url"] = "L’adresse d’Umami est requise pour activer le module"
	}
	if in.Enabled && !s.env.AuthFromEnv() && in.AuthMode == "login" && in.Username == "" {
		details["username"] = "L’identifiant est requis"
	}
	if len(details) > 0 {
		return AnalyticsSettingsView{}, domain.ErrValidation.WithDetails(details)
	}

	params := db.UpdateAnalyticsSettingsParams{
		Enabled: in.Enabled, BaseUrl: in.BaseURL, AuthMode: in.AuthMode, Username: in.Username,
		VersionPinned: in.APIVersion != 0, ApiVersion: int16(in.APIVersion), CacheMinutes: int32(in.CacheMinutes),
		// Une autre adresse, un autre compte : l'ancien jeton ne vaut plus.
		ResetToken: in.BaseURL != row.BaseUrl || in.Username != row.Username || in.AuthMode != row.AuthMode || in.Password != nil || in.APIKey != nil,
	}
	seal := func(v *string) ([]byte, bool, error) {
		if v == nil {
			return nil, false, nil
		}
		if strings.TrimSpace(*v) == "" {
			return nil, true, nil
		}
		sealed, err := security.Seal(s.sealKey, *v)
		return sealed, true, err
	}
	if params.PasswordEnc, params.SetPassword, err = seal(in.Password); err != nil {
		return AnalyticsSettingsView{}, fmt.Errorf("chiffrement du mot de passe : %w", err)
	}
	if params.ApiKeyEnc, params.SetApiKey, err = seal(in.APIKey); err != nil {
		return AnalyticsSettingsView{}, fmt.Errorf("chiffrement de la clé : %w", err)
	}
	if err := s.q.UpdateAnalyticsSettings(ctx, params); err != nil {
		return AnalyticsSettingsView{}, fmt.Errorf("enregistrement du module : %w", err)
	}
	if err := s.q.RequestAnalyticsCheck(ctx); err != nil {
		return AnalyticsSettingsView{}, fmt.Errorf("demande de verification : %w", err)
	}
	return s.Settings(ctx)
}

// RequestCheck demande une verification : le « Tester la connexion » de
// l'ecran, servi par la tache de fond.
func (s *AnalyticsService) RequestCheck(ctx context.Context) (AnalyticsSettingsView, error) {
	if err := s.q.RequestAnalyticsCheck(ctx); err != nil {
		return AnalyticsSettingsView{}, fmt.Errorf("demande de verification : %w", err)
	}
	return s.Settings(ctx)
}

// connection est l'acces au service, l'environnement d'abord.
type connection struct {
	baseURL, mode, username, password, apiKey string
}

func (s *AnalyticsService) connection(row db.AnalyticsSetting) connection {
	c := connection{baseURL: row.BaseUrl, mode: row.AuthMode, username: row.Username}
	if s.env.UmamiURL != "" {
		c.baseURL = strings.TrimRight(s.env.UmamiURL, "/")
	}
	if s.env.AuthFromEnv() {
		c.username, c.password, c.apiKey, c.mode = s.env.UmamiUsername, s.env.UmamiPassword, s.env.UmamiAPIKey, "login"
		if s.env.UmamiAPIKey != "" {
			c.mode = "api_key"
		}
		return c
	}
	open := func(sealed []byte) string {
		if len(sealed) == 0 {
			return ""
		}
		v, err := security.Open(s.sealKey, sealed)
		if err != nil {
			return ""
		}
		return v
	}
	if c.mode == "api_key" {
		c.apiKey = open(row.ApiKeyEnc)
	} else {
		c.password = open(row.PasswordEnc)
	}
	return c
}

// provider rend le fournisseur, reconstruit quand la connexion change.
func (s *AnalyticsService) provider(row db.AnalyticsSetting) (*analytics.Umami, error) {
	c := s.connection(row)
	if c.baseURL == "" {
		return nil, errors.New("adresse d’Umami manquante")
	}
	if c.mode == "api_key" && c.apiKey == "" {
		return nil, errors.New("clé d’API manquante ou illisible : ressaisissez-la")
	}
	if c.mode == "login" && (c.username == "" || c.password == "") {
		return nil, errors.New("identifiant ou mot de passe manquant ou illisible : ressaisissez-les")
	}

	sum := sha256.Sum256([]byte(strings.Join([]string{c.baseURL, c.mode, c.username, c.password, c.apiKey, fmt.Sprint(row.VersionPinned, row.ApiVersion)}, "\x00")))
	fingerprint := hex.EncodeToString(sum[:])

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.umami != nil && s.fingerprint == fingerprint {
		return s.umami, nil
	}
	u := &analytics.Umami{
		BaseURL: c.baseURL, Username: c.username, Password: c.password, APIKey: c.apiKey,
		Version: int(row.ApiVersion), Pinned: row.VersionPinned, Client: s.client,
		OnVersion: s.onVersion, OnToken: s.onToken,
	}
	if c.mode == "api_key" {
		u.Username, u.Password = "", ""
	} else {
		u.APIKey = ""
		if len(row.TokenEnc) > 0 {
			if token, err := security.Open(s.sealKey, row.TokenEnc); err == nil {
				u.SetToken(token)
			}
		}
	}
	s.umami, s.fingerprint = u, fingerprint
	return u, nil
}

// onVersion consigne une version qui change : la lecture s'est adaptee, et
// l'ecran le dit.
func (s *AnalyticsService) onVersion(previous, detected int) {
	// La premiere detection n'a rien a signaler : la version s'affiche deja.
	// Un changement, si : l'instance a ete mise a jour.
	var note *string
	if previous != 0 {
		text := fmt.Sprintf("Umami est passé de %d.x à %d.x : la lecture s’est adaptée d’elle-même.", previous, detected)
		note = &text
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.q.NoteAnalyticsCheck(ctx, db.NoteAnalyticsCheckParams{Note: note, ApiVersion: int16(detected)}); err != nil {
		s.log.Warn("version d'Umami non consignee", "error", err)
	}
}

// onToken garde le jeton, chiffre, pour le prochain demarrage.
func (s *AnalyticsService) onToken(token string) {
	sealed, err := security.Seal(s.sealKey, token)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.q.SetAnalyticsToken(ctx, sealed); err != nil {
		s.log.Warn("jeton d'Umami non garde", "error", err)
	}
}

// --- Periodes -----------------------------------------------------------------

// analyticsPeriod est une periode du selecteur, calculee dans le fuseau de
// l'agence.
type analyticsPeriod struct {
	Key   string
	Range analytics.Range
	Unit  analytics.Unit
	From  time.Time // premier jour
	To    time.Time // dernier jour
}

var standardPeriods = []string{"7d", "30d", "month", "prev_month", "12m"}

// periodOf calcule une periode. Une periode personnalisee s'ecrit
// « custom:2026-01-01:2026-03-31 », bornes comprises.
func periodOf(key string, now time.Time) (analyticsPeriod, error) {
	loc := zone.Location()
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	p := analyticsPeriod{Key: key, Unit: analytics.Day, To: today}

	switch {
	case key == "7d":
		p.From = today.AddDate(0, 0, -6)
	case key == "30d":
		p.From = today.AddDate(0, 0, -29)
	case key == "month":
		p.From = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
	case key == "prev_month":
		first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		p.From, p.To = first.AddDate(0, -1, 0), first.AddDate(0, 0, -1)
	case key == "12m":
		p.From = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, -11, 0)
		p.Unit = analytics.Month
	case strings.HasPrefix(key, "custom:"):
		parts := strings.Split(key, ":")
		if len(parts) != 3 {
			return p, errors.New("période illisible")
		}
		from, err1 := time.ParseInLocation(dateLayout, parts[1], loc)
		to, err2 := time.ParseInLocation(dateLayout, parts[2], loc)
		if err1 != nil || err2 != nil {
			return p, errors.New("dates illisibles : AAAA-MM-JJ")
		}
		if to.Before(from) {
			return p, errors.New("la fin précède le début")
		}
		if to.After(today) {
			return p, errors.New("la période ne peut pas finir après aujourd’hui")
		}
		if from.Before(to.AddDate(0, -analyticsMaxMonths, 0)) {
			return p, fmt.Errorf("%d mois au plus", analyticsMaxMonths)
		}
		p.From, p.To = from, to
		switch days := int(to.Sub(from).Hours()/24) + 1; {
		case days <= 2:
			p.Unit = analytics.Hour
		case days > 92:
			p.Unit = analytics.Month
		}
	default:
		return p, errors.New("période inconnue")
	}

	end := p.To.AddDate(0, 0, 1).Add(-time.Millisecond)
	if end.After(now) {
		end = now
	}
	p.Range = analytics.Range{Start: p.From, End: end}
	return p, nil
}

// ttlOf : les vues courantes suivent le reglage, le mois ecoule et les douze
// mois une fois par jour, une periode personnalisee passee jamais.
func ttlOf(key string, cacheMinutes int32, now time.Time) time.Duration {
	switch key {
	case "7d", "30d", "month":
		return time.Duration(cacheMinutes) * time.Minute
	case "prev_month", "12m":
		return analyticsSlowTTL
	}
	if p, err := periodOf(key, now); err == nil {
		local := now.In(zone.Location())
		if p.To.Before(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone.Location())) {
			return 30 * 24 * time.Hour
		}
	}
	return time.Duration(cacheMinutes) * time.Minute
}

// --- Vue -------------------------------------------------------------------------

// StatisticsTotals sont les chiffres d'une periode. TotalTime en secondes.
type StatisticsTotals struct {
	Visitors  int64 `json:"visitors"`
	Visits    int64 `json:"visits"`
	Pageviews int64 `json:"pageviews"`
	Bounces   int64 `json:"bounces"`
	TotalTime int64 `json:"total_time"`
}

// StatisticsPoint est un pas de courbe, a l'heure locale de l'agence.
type StatisticsPoint struct {
	At        string `json:"at"`
	Pageviews int64  `json:"pageviews"`
	Visitors  int64  `json:"visitors"`
}

// StatisticsRow est une ligne de classement.
type StatisticsRow struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// StatisticsData est une vue entiere : ce que le cache garde, ce que la
// photo du mois fige.
type StatisticsData struct {
	Period struct {
		Key      string `json:"key"`
		From     string `json:"from"`
		To       string `json:"to"`
		Unit     string `json:"unit"`
		Timezone string `json:"timezone"`
	} `json:"period"`
	Totals    StatisticsTotals  `json:"totals"`
	Previous  StatisticsTotals  `json:"previous"`
	Series    []StatisticsPoint `json:"series"`
	Pages     []StatisticsRow   `json:"pages"`
	Referrers []StatisticsRow   `json:"referrers"`
	Countries []StatisticsRow   `json:"countries"`
	Devices   []StatisticsRow   `json:"devices"`
	Browsers  []StatisticsRow   `json:"browsers"`
	OS        []StatisticsRow   `json:"os"`
	Events    []StatisticsRow   `json:"events"`
}

// fetch lit une vue entiere au service.
func fetch(ctx context.Context, p analytics.Provider, websiteID string, period analyticsPeriod) (StatisticsData, error) {
	var out StatisticsData
	out.Period.Key, out.Period.Unit, out.Period.Timezone = period.Key, string(period.Unit), zone.Name()
	out.Period.From, out.Period.To = period.From.Format(dateLayout), period.To.Format(dateLayout)

	stats, err := p.Stats(ctx, websiteID, period.Range)
	if err != nil {
		return out, err
	}
	out.Totals, out.Previous = totalsOf(stats.Current), totalsOf(stats.Previous)

	points, err := p.Timeseries(ctx, websiteID, period.Range, period.Unit, zone.Name())
	if err != nil {
		return out, err
	}
	out.Series = make([]StatisticsPoint, 0, len(points))
	for _, pt := range points {
		out.Series = append(out.Series, StatisticsPoint{At: pt.At, Pageviews: pt.Pageviews, Visitors: pt.Visitors})
	}

	for _, m := range []struct {
		dim analytics.Dimension
		dst *[]StatisticsRow
	}{
		{analytics.Page, &out.Pages}, {analytics.Referrer, &out.Referrers}, {analytics.Country, &out.Countries},
		{analytics.Device, &out.Devices}, {analytics.Browser, &out.Browsers}, {analytics.OS, &out.OS}, {analytics.Event, &out.Events},
	} {
		rows, err := p.Metrics(ctx, websiteID, period.Range, m.dim, analyticsTopLimit)
		if err != nil {
			return out, fmt.Errorf("classement %s : %w", m.dim, err)
		}
		*m.dst = make([]StatisticsRow, 0, len(rows))
		for _, r := range rows {
			*m.dst = append(*m.dst, StatisticsRow{Value: r.Value, Count: r.Count})
		}
	}
	return out, nil
}

func totalsOf(t analytics.Totals) StatisticsTotals {
	return StatisticsTotals{Visitors: t.Visitors, Visits: t.Visits, Pageviews: t.Pageviews, Bounces: t.Bounces, TotalTime: t.TotalTime}
}

// --- Tache de fond ---------------------------------------------------------------

// Tick est un passage de la tache : verification, demandes en attente, cache
// a rafraichir, visiteurs du moment, photos du mois, purges. Rien ne s'y
// fait pour un module inactif, hors une verification demandee.
func (s *AnalyticsService) Tick(ctx context.Context) error {
	row, err := s.q.GetAnalyticsSettings(ctx)
	if err != nil {
		return fmt.Errorf("lecture du module : %w", err)
	}
	now := s.now()

	if row.CheckRequestedAt != nil || (row.Enabled && (row.LastCheckAt == nil || now.Sub(*row.LastCheckAt) >= analyticsCheckEvery)) {
		if err := s.check(ctx, row); err != nil {
			return err
		}
		if row, err = s.q.GetAnalyticsSettings(ctx); err != nil {
			return fmt.Errorf("lecture du module : %w", err)
		}
	}
	if !row.Enabled {
		return nil
	}
	p, err := s.provider(row)
	if err != nil {
		return nil // l'erreur est deja a l'ecran, par la verification
	}

	s.serveRequests(ctx, p, now)
	s.refresh(ctx, p, row, now)
	s.refreshActive(ctx, p, now)
	s.snapshot(ctx, p, now)

	if err := s.q.PurgeAnalyticsCache(ctx, now.Add(-7*24*time.Hour)); err != nil {
		s.log.Warn("purge du cache des statistiques", "error", err)
	}
	purgeBefore := now.Add(-24 * time.Hour)
	if err := s.q.PurgeAnalyticsRequests(ctx, &purgeBefore); err != nil {
		s.log.Warn("purge des demandes de statistiques", "error", err)
	}
	return nil
}

// check verifie la connexion et synchronise la liste des sites. Une
// premiere lecture de /stats fixe la version.
func (s *AnalyticsService) check(ctx context.Context, row db.AnalyticsSetting) error {
	p, err := s.provider(row)
	if err != nil {
		return s.recordCheck(ctx, err)
	}
	sites, err := p.ListWebsites(ctx)
	if err != nil {
		return s.recordCheck(ctx, err)
	}

	ids := make([]uuid.UUID, 0, len(sites))
	for _, site := range sites {
		id, err := s.q.UpsertAnalyticsWebsite(ctx, db.UpsertAnalyticsWebsiteParams{
			Provider: analyticsProvider, ExternalID: site.ID, Name: site.Name, Domain: site.Domain,
		})
		if err != nil {
			return s.recordCheck(ctx, fmt.Errorf("site %s : %w", site.Name, err))
		}
		ids = append(ids, id)
	}
	if err := s.q.MarkAnalyticsWebsitesAbsent(ctx, db.MarkAnalyticsWebsitesAbsentParams{Provider: analyticsProvider, Seen: ids}); err != nil {
		return s.recordCheck(ctx, fmt.Errorf("sites disparus : %w", err))
	}

	if len(sites) > 0 && !row.VersionPinned {
		now := s.now()
		if _, err := p.Stats(ctx, sites[0].ID, analytics.Range{Start: now.Add(-24 * time.Hour), End: now}); err != nil {
			return s.recordCheck(ctx, fmt.Errorf("lecture des chiffres : %w", err))
		}
	}
	return s.recordCheck(ctx, nil)
}

func (s *AnalyticsService) recordCheck(ctx context.Context, checkErr error) error {
	var msg *string
	if checkErr != nil {
		text := checkErr.Error()
		if errors.Is(checkErr, analytics.ErrUnauthorized) {
			text = "Umami refuse les identifiants : vérifiez-les"
		}
		msg = &text
	}
	if err := s.q.RecordAnalyticsCheck(ctx, msg); err != nil {
		return fmt.Errorf("enregistrement de la verification : %w", err)
	}
	return nil
}

// serveRequests sert les vues demandees par les ecrans.
func (s *AnalyticsService) serveRequests(ctx context.Context, p analytics.Provider, now time.Time) {
	pending, err := s.q.ListPendingAnalyticsRequests(ctx, 10)
	if err != nil {
		s.log.Warn("demandes de statistiques", "error", err)
		return
	}
	for _, r := range pending {
		var msg *string
		if err := s.store(ctx, p, r.WebsiteID, r.ExternalID, r.PeriodKey, now); err != nil {
			text := "Statistiques momentanément indisponibles"
			if errors.Is(err, analytics.ErrNotFound) {
				text = "Ce site n’existe plus dans Umami"
			}
			msg = &text
			s.log.Warn("vue de statistiques", "period", r.PeriodKey, "error", err)
		}
		if err := s.q.ServeAnalyticsRequest(ctx, db.ServeAnalyticsRequestParams{ID: r.ID, Error: msg}); err != nil {
			s.log.Warn("demande de statistiques non close", "error", err)
		}
	}
}

func (s *AnalyticsService) store(ctx context.Context, p analytics.Provider, websiteID uuid.UUID, externalID, key string, now time.Time) error {
	period, err := periodOf(key, now)
	if err != nil {
		return err
	}
	data, err := fetch(ctx, p, externalID, period)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.q.UpsertAnalyticsCache(ctx, db.UpsertAnalyticsCacheParams{WebsiteID: websiteID, PeriodKey: key, Payload: payload})
}

// refresh tient le cache des sites de production a jour : les trente jours
// toujours — la vue par defaut —, les autres periodes tant qu'un ecran les a
// lues recemment, chacune a son rythme. Quelques vues par passage.
func (s *AnalyticsService) refresh(ctx context.Context, p analytics.Provider, row db.AnalyticsSetting, now time.Time) {
	sites, err := s.q.ListProdAnalyticsWebsites(ctx)
	if err != nil || len(sites) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(sites))
	for _, site := range sites {
		ids = append(ids, site.ID)
	}
	states, err := s.q.ListAnalyticsCacheState(ctx, ids)
	if err != nil {
		s.log.Warn("etat du cache des statistiques", "error", err)
		return
	}
	type cacheKey struct {
		site uuid.UUID
		key  string
	}
	known := map[cacheKey]db.ListAnalyticsCacheStateRow{}
	for _, st := range states {
		known[cacheKey{st.WebsiteID, st.PeriodKey}] = st
	}

	budget := analyticsRefreshPerTick
	for _, site := range sites {
		// La vue par defaut existe pour chaque site, avant meme qu'on la lise.
		if _, ok := known[cacheKey{site.ID, "30d"}]; !ok && budget > 0 {
			budget--
			if err := s.store(ctx, p, site.ID, site.ExternalID, "30d", now); err != nil {
				s.log.Warn("statistiques des trente jours", "error", err)
			}
		}
	}
	for _, st := range states {
		if budget == 0 {
			break
		}
		if st.PeriodKey != "30d" && now.Sub(st.LastReadAt) > analyticsReadWindow {
			continue
		}
		if now.Sub(st.FetchedAt) < ttlOf(st.PeriodKey, row.CacheMinutes, now) {
			continue
		}
		external := ""
		for _, site := range sites {
			if site.ID == st.WebsiteID {
				external = site.ExternalID
			}
		}
		budget--
		if err := s.store(ctx, p, st.WebsiteID, external, st.PeriodKey, now); err != nil {
			s.log.Warn("rafraichissement des statistiques", "period", st.PeriodKey, "error", err)
		}
	}
}

// refreshActive releve, chaque minute, les visiteurs du moment des sites
// qu'un ecran regarde.
func (s *AnalyticsService) refreshActive(ctx context.Context, p analytics.Provider, now time.Time) {
	stale := now.Add(-55 * time.Second)
	watched, err := s.q.ListWatchedAnalyticsActive(ctx, db.ListWatchedAnalyticsActiveParams{Since: now.Add(-2 * time.Minute), Stale: &stale})
	if err != nil {
		return
	}
	for _, w := range watched {
		n, err := p.Active(ctx, w.ExternalID)
		if err != nil {
			s.log.Warn("visiteurs du moment", "error", err)
			continue
		}
		if err := s.q.SetAnalyticsActive(ctx, db.SetAnalyticsActiveParams{WebsiteID: w.WebsiteID, Visitors: ptr(int32(n))}); err != nil {
			s.log.Warn("visiteurs du moment non gardes", "error", err)
		}
	}
}

// snapshot fige, le 1er du mois et les jours suivants s'il le faut, les
// chiffres du mois ecoule de chaque site de production.
func (s *AnalyticsService) snapshot(ctx context.Context, p analytics.Provider, now time.Time) {
	period, err := periodOf("prev_month", now)
	if err != nil {
		return
	}
	month := time.Date(period.From.Year(), period.From.Month(), 1, 0, 0, 0, 0, time.UTC)
	due, err := s.q.ListAnalyticsSnapshotsDue(ctx, month)
	if err != nil || len(due) == 0 {
		return
	}
	for _, site := range due[:min(len(due), 2)] {
		data, err := fetch(ctx, p, site.ExternalID, period)
		if err != nil {
			s.log.Warn("photo du mois", "error", err)
			continue
		}
		payload, err := json.Marshal(data)
		if err != nil {
			continue
		}
		if err := s.q.InsertAnalyticsSnapshot(ctx, db.InsertAnalyticsSnapshotParams{WebsiteID: site.ID, Month: month, Payload: payload}); err != nil {
			s.log.Warn("photo du mois non gardee", "error", err)
		}
	}
}

// --- Sites et projets ----------------------------------------------------------

// AnalyticsWebsiteView est un site du service.
type AnalyticsWebsiteView struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Domain     string    `json:"domain"`
	Present    bool      `json:"present"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Projects   []string  `json:"projects"`
}

// AnalyticsWebsitePage est une page de sites.
type AnalyticsWebsitePage struct {
	Items []AnalyticsWebsiteView `json:"items"`
	Total int                    `json:"total"`
	Page  int                    `json:"page"`
}

// Websites rend une page de sites, filtree par nom ou domaine.
func (s *AnalyticsService) Websites(ctx context.Context, query string, page, pageSize int) (AnalyticsWebsitePage, error) {
	page, pageSize = clampPage(page, pageSize)
	query = strings.TrimSpace(query)
	total, err := s.q.CountAnalyticsWebsites(ctx, query)
	if err != nil {
		return AnalyticsWebsitePage{}, fmt.Errorf("compte des sites : %w", err)
	}
	rows, err := s.q.ListAnalyticsWebsites(ctx, db.ListAnalyticsWebsitesParams{Q: query, Lim: int32(pageSize), Off: int32((page - 1) * pageSize)})
	if err != nil {
		return AnalyticsWebsitePage{}, fmt.Errorf("liste des sites : %w", err)
	}
	out := AnalyticsWebsitePage{Items: make([]AnalyticsWebsiteView, 0, len(rows)), Total: int(total), Page: page}
	for _, r := range rows {
		out.Items = append(out.Items, AnalyticsWebsiteView{
			ID: r.ID, Name: r.Name, Domain: r.Domain, Present: r.Present, LastSeenAt: r.LastSeenAt, Projects: r.Projects,
		})
	}
	return out, nil
}

// ProjectAnalyticsView est l'onglet « Statistiques » des parametres d'un
// projet.
type ProjectAnalyticsView struct {
	ModuleEnabled bool                   `json:"module_enabled"`
	Sites         []ProjectAnalyticsSite `json:"sites"`
}

// ProjectAnalyticsSite est un site lie au projet.
type ProjectAnalyticsSite struct {
	Environment string    `json:"environment"`
	WebsiteID   uuid.UUID `json:"website_id"`
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	Present     bool      `json:"present"`
}

// ProjectAnalytics rend les sites d'un projet.
func (s *AnalyticsService) ProjectAnalytics(ctx context.Context, projectID uuid.UUID) (ProjectAnalyticsView, error) {
	exists, err := s.q.AnalyticsProjectExists(ctx, projectID)
	if err != nil {
		return ProjectAnalyticsView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ProjectAnalyticsView{}, domain.ErrNotFound
	}
	settings, err := s.q.GetAnalyticsSettings(ctx)
	if err != nil {
		return ProjectAnalyticsView{}, fmt.Errorf("lecture du module : %w", err)
	}
	rows, err := s.q.ListProjectAnalyticsWebsites(ctx, projectID)
	if err != nil {
		return ProjectAnalyticsView{}, fmt.Errorf("sites du projet : %w", err)
	}
	out := ProjectAnalyticsView{ModuleEnabled: settings.Enabled, Sites: make([]ProjectAnalyticsSite, 0, len(rows))}
	for _, r := range rows {
		out.Sites = append(out.Sites, ProjectAnalyticsSite{Environment: r.Environment, WebsiteID: r.ID, Name: r.Name, Domain: r.Domain, Present: r.Present})
	}
	return out, nil
}

// ProjectWebsiteInput est un lien demande par l'ecran.
type ProjectWebsiteInput struct {
	WebsiteID   uuid.UUID
	Environment string
}

// SetProjectWebsites remplace les sites d'un projet : une production au
// plus, c'est elle que voit le client.
func (s *AnalyticsService) SetProjectWebsites(ctx context.Context, projectID uuid.UUID, sites []ProjectWebsiteInput) (ProjectAnalyticsView, error) {
	exists, err := s.q.AnalyticsProjectExists(ctx, projectID)
	if err != nil {
		return ProjectAnalyticsView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ProjectAnalyticsView{}, domain.ErrNotFound
	}
	if len(sites) > maxProjectSites {
		return ProjectAnalyticsView{}, domain.ErrValidation.WithDetails(map[string]any{"sites": fmt.Sprintf("%d sites au plus", maxProjectSites)})
	}
	seen := map[uuid.UUID]bool{}
	prod := 0
	ids := make([]uuid.UUID, 0, len(sites))
	for _, site := range sites {
		switch site.Environment {
		case "prod":
			prod++
		case "preprod":
		default:
			return ProjectAnalyticsView{}, domain.ErrValidation.WithDetails(map[string]any{"environment": "prod ou preprod"})
		}
		if seen[site.WebsiteID] {
			return ProjectAnalyticsView{}, domain.ErrValidation.WithDetails(map[string]any{"sites": "Un site ne se lie qu’une fois"})
		}
		seen[site.WebsiteID] = true
		ids = append(ids, site.WebsiteID)
	}
	if prod > 1 {
		return ProjectAnalyticsView{}, domain.ErrValidation.WithDetails(map[string]any{"sites": "Un seul site de production : c’est celui que voit le client"})
	}
	if len(ids) > 0 {
		found, err := s.q.AnalyticsWebsitesExist(ctx, ids)
		if err != nil {
			return ProjectAnalyticsView{}, fmt.Errorf("verification des sites : %w", err)
		}
		if int(found) != len(ids) {
			return ProjectAnalyticsView{}, domain.ErrValidation.WithDetails(map[string]any{"sites": "Site inconnu"})
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectAnalyticsView{}, fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.DeleteProjectAnalyticsWebsites(ctx, projectID); err != nil {
		return ProjectAnalyticsView{}, fmt.Errorf("liens actuels : %w", err)
	}
	for _, site := range sites {
		if err := q.InsertProjectAnalyticsWebsite(ctx, db.InsertProjectAnalyticsWebsiteParams{
			ProjectID: projectID, WebsiteID: site.WebsiteID, Environment: site.Environment,
		}); err != nil {
			return ProjectAnalyticsView{}, fmt.Errorf("lien au site : %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectAnalyticsView{}, err
	}
	return s.ProjectAnalytics(ctx, projectID)
}

// --- Portail -------------------------------------------------------------------------

// StatisticsView est la page « Statistiques » d'un projet du client.
type StatisticsView struct {
	// ready : les chiffres sont la ; pending : la tache les prepare, relire
	// dans quelques secondes ; error : le service n'a pas pu les donner.
	Status  string `json:"status"`
	Message string `json:"message"`
	Website struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
	} `json:"website"`
	FetchedAt *time.Time      `json:"fetched_at"`
	Data      *StatisticsData `json:"data"`
}

// ActiveView sont les visiteurs du moment.
type ActiveView struct {
	// Nul tant que la tache ne les a pas releves.
	Visitors  *int32     `json:"visitors"`
	FetchedAt *time.Time `json:"fetched_at"`
}

// portalWebsite rend le site de production d'un projet du client, ou
// « introuvable » : un projet d'un autre client, un module inactif, un
// projet sans site ne se distinguent pas.
func (s *AnalyticsService) portalWebsite(ctx context.Context, userID, projectID uuid.UUID) (db.PortalAnalyticsWebsiteRow, error) {
	clientID, err := s.q.PortalClientOf(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && clientID == nil) {
		return db.PortalAnalyticsWebsiteRow{}, domain.ErrNotFound
	}
	if err != nil {
		return db.PortalAnalyticsWebsiteRow{}, fmt.Errorf("lecture du compte : %w", err)
	}
	site, err := s.q.PortalAnalyticsWebsite(ctx, db.PortalAnalyticsWebsiteParams{ProjectID: projectID, ClientID: *clientID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.PortalAnalyticsWebsiteRow{}, domain.ErrNotFound
	}
	if err != nil {
		return db.PortalAnalyticsWebsiteRow{}, fmt.Errorf("site du projet : %w", err)
	}
	return site, nil
}

// PortalStatistics rend la page d'un projet pour une periode : depuis le
// cache, ou en attente d'une vue que la tache prepare. Une periode
// personnalisee compte dans la limite du compte, sauf deja en cache ou deja
// demandee.
func (s *AnalyticsService) PortalStatistics(ctx context.Context, userID, projectID uuid.UUID, key string) (StatisticsView, error) {
	site, err := s.portalWebsite(ctx, userID, projectID)
	if err != nil {
		return StatisticsView{}, err
	}
	now := s.now()
	if _, err := periodOf(key, now); err != nil {
		return StatisticsView{}, domain.ErrValidation.WithDetails(map[string]any{"period": err.Error()})
	}

	var out StatisticsView
	out.Website.Name, out.Website.Domain = site.Name, site.Domain

	cached, err := s.q.GetAnalyticsCache(ctx, db.GetAnalyticsCacheParams{WebsiteID: site.ID, PeriodKey: key})
	switch {
	case err == nil:
		var data StatisticsData
		if err := json.Unmarshal(cached.Payload, &data); err != nil {
			return StatisticsView{}, fmt.Errorf("vue en cache illisible : %w", err)
		}
		if err := s.q.TouchAnalyticsCache(ctx, db.TouchAnalyticsCacheParams{WebsiteID: site.ID, PeriodKey: key}); err != nil {
			return StatisticsView{}, fmt.Errorf("lecture du cache : %w", err)
		}
		fetched := cached.FetchedAt
		out.Status, out.Data, out.FetchedAt = "ready", &data, &fetched
		return out, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return StatisticsView{}, fmt.Errorf("lecture du cache : %w", err)
	}

	// Pas en cache : deja demandee, en echec, ou a demander.
	pending, err := s.q.PendingAnalyticsRequestExists(ctx, db.PendingAnalyticsRequestExistsParams{WebsiteID: site.ID, PeriodKey: key})
	if err != nil {
		return StatisticsView{}, fmt.Errorf("demandes en attente : %w", err)
	}
	if !pending {
		if last, err := s.q.LastAnalyticsRequestError(ctx, db.LastAnalyticsRequestErrorParams{WebsiteID: site.ID, PeriodKey: key}); err == nil && last != nil {
			out.Status, out.Message = "error", *last
			// Une erreur passee ne bloque pas : la demande repart, et l'ecran
			// le dira au prochain passage.
		}
		if strings.HasPrefix(key, "custom:") {
			count, err := s.q.CountAnalyticsRequestsSince(ctx, db.CountAnalyticsRequestsSinceParams{UserID: &userID, Since: now.Add(-time.Hour)})
			if err != nil {
				return StatisticsView{}, fmt.Errorf("demandes du compte : %w", err)
			}
			if count >= analyticsRequestsPerHour {
				return StatisticsView{}, domain.ErrRateLimited.WithMessage("Trop de périodes personnalisées en une heure : réessayez plus tard, ou choisissez une période prédéfinie")
			}
		}
		if err := s.q.CreateAnalyticsRequest(ctx, db.CreateAnalyticsRequestParams{WebsiteID: site.ID, PeriodKey: key, RequestedBy: &userID}); err != nil {
			return StatisticsView{}, fmt.Errorf("demande de statistiques : %w", err)
		}
	}
	if out.Status == "" {
		out.Status, out.Message = "pending", "Les chiffres de cette période sont en cours de préparation."
	}
	return out, nil
}

// PortalActive rend les visiteurs du moment, et signale qu'un ecran regarde :
// la tache les relevera chaque minute tant qu'il est ouvert.
func (s *AnalyticsService) PortalActive(ctx context.Context, userID, projectID uuid.UUID) (ActiveView, error) {
	site, err := s.portalWebsite(ctx, userID, projectID)
	if err != nil {
		return ActiveView{}, err
	}
	if err := s.q.WatchAnalyticsActive(ctx, site.ID); err != nil {
		return ActiveView{}, fmt.Errorf("visiteurs du moment : %w", err)
	}
	row, err := s.q.GetAnalyticsActive(ctx, site.ID)
	if err != nil {
		return ActiveView{}, fmt.Errorf("visiteurs du moment : %w", err)
	}
	return ActiveView{Visitors: row.Visitors, FetchedAt: row.FetchedAt}, nil
}

// AudienceSite est un site de production du client et ses trente derniers
// jours, pour le bandeau du portail.
type AudienceSite struct {
	ID         uuid.UUID   `json:"id"`
	Name       string      `json:"name"`
	Domain     string      `json:"domain"`
	ProjectIDs []uuid.UUID `json:"project_ids"`
	// Visiteurs sur la periode, et la courbe jour par jour.
	Visitors int64           `json:"visitors"`
	Series   []AudiencePoint `json:"series"`
}

// AudiencePoint est un jour de la courbe.
type AudiencePoint struct {
	At       string `json:"at"`
	Visitors int64  `json:"visitors"`
}

// AudienceView est le bandeau d'audience : vide quand le module est coupe ou
// qu'aucun site de production n'a encore ses chiffres.
type AudienceView struct {
	Sites []AudienceSite `json:"sites"`
}

// PortalAudience rend les trente jours des sites de production du client,
// lus dans le cache que la tache tient toujours pret pour eux.
func (s *AnalyticsService) PortalAudience(ctx context.Context, userID uuid.UUID) (AudienceView, error) {
	out := AudienceView{Sites: []AudienceSite{}}
	clientID, err := s.q.PortalClientOf(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && clientID == nil) {
		return out, nil
	}
	if err != nil {
		return AudienceView{}, fmt.Errorf("lecture du compte : %w", err)
	}
	rows, err := s.q.PortalAudienceSites(ctx, *clientID)
	if err != nil {
		return AudienceView{}, fmt.Errorf("sites du client : %w", err)
	}
	for _, row := range rows {
		var data StatisticsData
		if err := json.Unmarshal(row.Payload, &data); err != nil {
			return AudienceView{}, fmt.Errorf("vue en cache illisible : %w", err)
		}
		site := AudienceSite{
			ID: row.ID, Name: row.Name, Domain: row.Domain, ProjectIDs: row.ProjectIds,
			Visitors: data.Totals.Visitors, Series: make([]AudiencePoint, 0, len(data.Series)),
		}
		for _, pt := range data.Series {
			site.Series = append(site.Series, AudiencePoint{At: pt.At, Visitors: pt.Visitors})
		}
		out.Sites = append(out.Sites, site)
	}
	return out, nil
}

func ptr[T any](v T) *T { return &v }
