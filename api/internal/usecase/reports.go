package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	mailer "github.com/plugiit/piilot-app/api/internal/mail"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/zone"
)

// ReportService porte le rapport mensuel de maintenance : la generation des
// brouillons, la relecture, la validation, l'envoi et l'archive du client.
//
// Un rapport repose sur ses donnees figees : tout ce qu'il montre est copie
// a sa generation. Tout vient de tables locales (agregats de Kuma, photo du
// mois d'Umami, interventions, mises en ligne) : rien n'appelle un service
// exterieur, ni ici ni ailleurs.
type ReportService struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	clientURL string
	canSend   bool
	log       *slog.Logger
	now       func() time.Time
}

// NewReportService construit le service. clientURL est l'adresse de l'espace
// client, ou mene le lien de l'e-mail.
func NewReportService(pool *pgxpool.Pool, clientURL string, canSend bool, log *slog.Logger) *ReportService {
	return &ReportService{pool: pool, q: db.New(pool), clientURL: strings.TrimRight(clientURL, "/"), canSend: canSend, log: log, now: time.Now}
}

// --- Donnees du rapport ----------------------------------------------------------

// ReportData est le contenu fige d'un rapport. Une section absente est nulle :
// la formule ne l'inclut pas, ou elle n'a pas de donnees ce mois-la.
type ReportData struct {
	Month   string `json:"month"`
	Project struct {
		Name       string `json:"name"`
		ClientName string `json:"client_name"`
	} `json:"project"`
	Plan struct {
		Name            string `json:"name"`
		IncludedMinutes int32  `json:"included_minutes"`
	} `json:"plan"`
	Availability  *ReportAvailability  `json:"availability"`
	Audience      *ReportAudience      `json:"audience"`
	Interventions *ReportInterventions `json:"interventions"`
	Deployments   *ReportDeployments   `json:"deployments"`
	Updates       *ReportUpdates       `json:"updates"`
	Backups       *ReportBackups       `json:"backups"`
	Domain        *ReportDomains       `json:"domain"`
}

// ReportAvailability est la section « Disponibilite ».
type ReportAvailability struct {
	Sites     []ReportSite     `json:"sites"`
	Incidents []ReportIncident `json:"incidents"`
}

// ReportSite est un site du projet et son mois.
type ReportSite struct {
	Name        string   `json:"name"`
	Environment string   `json:"environment"`
	URL         string   `json:"url"`
	Uptime      *float64 `json:"uptime"`
	// Temps de reponse moyen du mois, nul sans mesure.
	AvgResponseMS *int `json:"avg_response_ms"`
	// Jours avant l'expiration du certificat, le jour de la generation.
	CertDays  *int32      `json:"cert_days"`
	CertValid *bool       `json:"cert_valid"`
	Days      []UptimeDay `json:"days"`
}

// ReportIncident est une panne qui touche le mois.
type ReportIncident struct {
	Site            string     `json:"site"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	DurationSeconds *int32     `json:"duration_seconds"`
	Message         string     `json:"message"`
}

// ReportAudience est la section « Audience » : la photo figee du mois du site
// de production. Indisponible quand Umami n'a pas pu la donner a temps.
type ReportAudience struct {
	Unavailable bool            `json:"unavailable"`
	Website     string          `json:"website"`
	Domain      string          `json:"domain"`
	Data        *StatisticsData `json:"data"`
}

// ReportInterventions est la section « Interventions ».
type ReportInterventions struct {
	Items []ReportIntervention `json:"items"`
	// Toutes les interventions du mois, visibles ou non : c'est ce qui compte
	// sur le temps inclus.
	ConsumedMinutes int32 `json:"consumed_minutes"`
	VisibleMinutes  int32 `json:"visible_minutes"`
	IncludedMinutes int32 `json:"included_minutes"`
	OverrunMinutes  int32 `json:"overrun_minutes"`
}

// ReportIntervention est une intervention visible du mois.
type ReportIntervention struct {
	SpentOn string `json:"spent_on"`
	Kind    string `json:"kind"`
	Minutes int32  `json:"minutes"`
	Note    string `json:"note"`
}

// ReportDeployments est la section « Mises en ligne ».
type ReportDeployments struct {
	Items []ReportDeployment `json:"items"`
}

// ReportDeployment est une mise en ligne du mois.
type ReportDeployment struct {
	Name        string    `json:"name"`
	Environment string    `json:"environment"`
	URL         string    `json:"url"`
	DeployedAt  time.Time `json:"deployed_at"`
}

// ReportUpdates est la section « Mises a jour » : ce qui a ete fusionne dans le
// mois, en distinguant les dependances et les correctifs de securite.
type ReportUpdates struct {
	DependencyCount int            `json:"dependency_count"`
	SecurityCount   int            `json:"security_count"`
	OtherCount      int            `json:"other_count"`
	Items           []ReportUpdate `json:"items"`
	// Les lancements reussis du mois, sur l'environnement choisi.
	Runs []ReportUpdateRun `json:"runs"`
	// Le dernier lancement du mois en production a echoue : un humain relit.
	// Efface pour le client, qui n'en voit que ce que l'agence ecrit.
	LastRunFailed bool `json:"last_run_failed,omitempty"`
}

// ReportUpdateRun est un lancement reussi.
type ReportUpdateRun struct {
	Environment string     `json:"environment"`
	FinishedAt  *time.Time `json:"finished_at"`
}

// ReportUpdate est une mise a jour de dependance ou de securite du mois.
type ReportUpdate struct {
	Title    string     `json:"title"`
	MergedAt *time.Time `json:"merged_at"`
	Security bool       `json:"security"`
}

// ReportBackups est la section « Sauvegardes ».
type ReportBackups struct {
	Successes         int        `json:"successes"`
	Failures          int        `json:"failures"`
	LastSuccessAt     *time.Time `json:"last_success_at"`
	LastRestoreTestOn *string    `json:"last_restore_test_on"`
	// A la fin du mois : une sauvegarde reussie dans le delai attendu, et pas
	// d'echec depuis. Pour la relecture : efface pour le client.
	Healthy bool `json:"healthy,omitempty"`
}

// ReportDomains est la section « Domaine ».
type ReportDomains struct {
	Items []ReportDomain `json:"items"`
}

// ReportDomain est un domaine et son echeance, le jour de la generation.
type ReportDomain struct {
	Domain    string     `json:"domain"`
	ExpiresAt *time.Time `json:"expires_at"`
	DaysLeft  *int       `json:"days_left"`
}

// reportInput est ce qu'une section recoit pour collecter son mois.
type reportInput struct {
	projectID      uuid.UUID
	month, lastDay time.Time
	// Bornes du mois en instants, dans le fuseau de l'agence.
	since, until time.Time
	included     int32
	// Le jour de la generation : l'echeance d'un domaine se compte depuis lui.
	now time.Time
}

// reportSection est une section du rapport : sa cle dans la formule, et ce
// qui la remplit. Une section sans donnees laisse le rapport intact. Les
// sources de la 1.6 s'ajoutent ici, sans toucher au reste.
type reportSection struct {
	key     string
	collect func(ctx context.Context, q *db.Queries, in reportInput, out *ReportData) error
}

var reportSectionRegistry = []reportSection{
	{"availability", collectAvailability},
	{"audience", collectAudience},
	{"interventions", collectInterventions},
	{"deployments", collectDeployments},
	{"updates", collectUpdates},
	{"backups", collectBackups},
	{"domain", collectDomain},
}

func collectAvailability(ctx context.Context, q *db.Queries, in reportInput, out *ReportData) error {
	sites, err := q.ReportUptimeSites(ctx, db.ReportUptimeSitesParams{ProjectID: in.projectID, FromDay: in.month, ToDay: in.lastDay})
	if err != nil {
		return fmt.Errorf("disponibilite : %w", err)
	}
	if len(sites) == 0 {
		return nil
	}
	days, err := q.ReportUptimeDays(ctx, db.ReportUptimeDaysParams{ProjectID: in.projectID, FromDay: in.month, ToDay: in.lastDay})
	if err != nil {
		return fmt.Errorf("frise du mois : %w", err)
	}
	byDay := map[uuid.UUID]map[string]*float64{}
	for _, d := range days {
		if byDay[d.MonitorID] == nil {
			byDay[d.MonitorID] = map[string]*float64{}
		}
		byDay[d.MonitorID][d.Day.Format(dateLayout)] = uptimeRatio(d.ChecksUp, d.ChecksDown)
	}

	section := &ReportAvailability{Sites: make([]ReportSite, 0, len(sites)), Incidents: []ReportIncident{}}
	for _, r := range sites {
		site := ReportSite{
			Name: r.Name, Environment: r.Environment, URL: r.Url, Uptime: uptimeRatio(r.Up, r.Down),
			CertDays: r.CertDays, CertValid: r.CertValid, Days: []UptimeDay{},
		}
		if r.PingCount > 0 {
			avg := int(r.PingSum / int64(r.PingCount))
			site.AvgResponseMS = &avg
		}
		for d := in.month; !d.After(in.lastDay); d = d.AddDate(0, 0, 1) {
			key := d.Format(dateLayout)
			site.Days = append(site.Days, UptimeDay{Day: key, Uptime: byDay[r.ID][key]})
		}
		section.Sites = append(section.Sites, site)
	}

	incidents, err := q.ReportIncidents(ctx, db.ReportIncidentsParams{ProjectID: in.projectID, Since: &in.since, Until: in.until})
	if err != nil {
		return fmt.Errorf("incidents du mois : %w", err)
	}
	for _, i := range incidents {
		section.Incidents = append(section.Incidents, ReportIncident{
			Site: i.MonitorName, StartedAt: i.StartedAt, EndedAt: i.EndedAt, DurationSeconds: i.DurationSeconds, Message: i.Message,
		})
	}
	out.Availability = section
	return nil
}

func collectAudience(ctx context.Context, q *db.Queries, in reportInput, out *ReportData) error {
	has, err := q.ReportHasAudienceSite(ctx, in.projectID)
	if err != nil {
		return fmt.Errorf("site d'audience : %w", err)
	}
	if !has {
		return nil
	}
	snap, err := q.ReportAudienceSnapshot(ctx, db.ReportAudienceSnapshotParams{ProjectID: in.projectID, Month: in.month})
	if errors.Is(err, pgx.ErrNoRows) {
		out.Audience = &ReportAudience{Unavailable: true}
		return nil
	}
	if err != nil {
		return fmt.Errorf("photo du mois : %w", err)
	}
	var data StatisticsData
	if err := json.Unmarshal(snap.Payload, &data); err != nil {
		return fmt.Errorf("photo du mois illisible : %w", err)
	}
	out.Audience = &ReportAudience{Website: snap.WebsiteName, Domain: snap.Domain, Data: &data}
	return nil
}

func collectInterventions(ctx context.Context, q *db.Queries, in reportInput, out *ReportData) error {
	rows, err := q.ReportInterventions(ctx, db.ReportInterventionsParams{ProjectID: in.projectID, FromDay: in.month, ToDay: in.lastDay})
	if err != nil {
		return fmt.Errorf("interventions du mois : %w", err)
	}
	consumed, err := q.ReportConsumed(ctx, db.ReportConsumedParams{ProjectID: in.projectID, Month: in.month})
	if err != nil {
		return fmt.Errorf("temps du mois : %w", err)
	}
	section := &ReportInterventions{Items: make([]ReportIntervention, 0, len(rows)), ConsumedMinutes: consumed, IncludedMinutes: in.included}
	for _, r := range rows {
		kind := ""
		if r.InterventionKind != nil {
			kind = *r.InterventionKind
		}
		section.Items = append(section.Items, ReportIntervention{SpentOn: r.SpentOn.Format(dateLayout), Kind: kind, Minutes: r.Minutes, Note: r.ClientNote})
		section.VisibleMinutes += r.Minutes
	}
	if consumed > in.included {
		section.OverrunMinutes = consumed - in.included
	}
	out.Interventions = section
	return nil
}

func collectDeployments(ctx context.Context, q *db.Queries, in reportInput, out *ReportData) error {
	rows, err := q.ReportDeployments(ctx, db.ReportDeploymentsParams{ProjectID: in.projectID, Since: in.since, Until: in.until})
	if err != nil {
		return fmt.Errorf("mises en ligne du mois : %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	section := &ReportDeployments{Items: make([]ReportDeployment, 0, len(rows))}
	for _, r := range rows {
		section.Items = append(section.Items, ReportDeployment{Name: r.Name, Environment: r.Environment, URL: r.Url, DeployedAt: r.DeployedAt})
	}
	out.Deployments = section
	return nil
}

// isDependencyUpdate reconnait les robots de mise a jour, a leur auteur ou au
// prefixe de branche qu'ils imposent.
func isDependencyUpdate(author, branch string) bool {
	author, branch = strings.ToLower(author), strings.ToLower(branch)
	return strings.Contains(author, "renovate") || strings.Contains(author, "dependabot") ||
		strings.HasPrefix(branch, "renovate/") || strings.HasPrefix(branch, "dependabot/")
}

// isSecurityUpdate : l'etiquette du webhook, ou ce que Renovate et les avis de
// securite ecrivent dans le titre.
func isSecurityUpdate(labelled bool, title string) bool {
	title = strings.ToLower(title)
	return labelled || strings.Contains(title, "[security]") || strings.Contains(title, "cve-") || strings.Contains(title, "ghsa-")
}

func collectUpdates(ctx context.Context, q *db.Queries, in reportInput, out *ReportData) error {
	rows, err := q.ReportMergedPullRequests(ctx, db.ReportMergedPullRequestsParams{ProjectID: in.projectID, Since: &in.since, Until: &in.until})
	if err != nil {
		return fmt.Errorf("mises a jour du mois : %w", err)
	}
	runs, err := q.ReportSiteUpdateRuns(ctx, db.ReportSiteUpdateRunsParams{ProjectID: in.projectID, Since: &in.since, Until: &in.until})
	if err != nil {
		return fmt.Errorf("lancements du mois : %w", err)
	}
	if len(rows) == 0 && len(runs) == 0 {
		return nil
	}
	section := &ReportUpdates{Items: []ReportUpdate{}, Runs: []ReportUpdateRun{}}
	for _, r := range runs {
		// Seule la production dit si le site du client est a jour : une
		// preproduction reussie ne rattrape pas un echec en production.
		if r.Environment == "prod" {
			section.LastRunFailed = r.Status != "success"
		}
		if r.Status == "success" {
			section.Runs = append(section.Runs, ReportUpdateRun{Environment: r.Environment, FinishedAt: r.FinishedAt})
		}
	}
	for _, r := range rows {
		security := isSecurityUpdate(r.Security, r.Title)
		dependency := isDependencyUpdate(r.Author, r.Branch)
		switch {
		case dependency && security:
			section.DependencyCount++
			section.SecurityCount++
		case dependency:
			section.DependencyCount++
		case security:
			section.SecurityCount++
		default:
			section.OtherCount++
			continue
		}
		section.Items = append(section.Items, ReportUpdate{Title: r.Title, MergedAt: r.MergedAt, Security: security})
	}
	out.Updates = section
	return nil
}

func collectBackups(ctx context.Context, q *db.Queries, in reportInput, out *ReportData) error {
	source, err := q.GetSiteBackupSource(ctx, in.projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("source des sauvegardes : %w", err)
	}
	counts, err := q.ReportBackupCounts(ctx, db.ReportBackupCountsParams{ProjectID: in.projectID, Since: in.since, Until: in.until})
	if err != nil {
		return fmt.Errorf("sauvegardes du mois : %w", err)
	}
	section := &ReportBackups{Successes: int(counts.Successes), Failures: int(counts.Failures)}
	if source.LastRestoreTestOn != nil {
		day := source.LastRestoreTestOn.Format(dateLayout)
		section.LastRestoreTestOn = &day
	}
	success, err := q.ReportLastBackupSuccess(ctx, db.ReportLastBackupSuccessParams{ProjectID: in.projectID, Until: in.until})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		out.Backups = section
		return nil
	case err != nil:
		return fmt.Errorf("derniere sauvegarde : %w", err)
	}
	section.LastSuccessAt = &success
	last, err := q.ReportLastBackupRun(ctx, db.ReportLastBackupRunParams{ProjectID: in.projectID, Until: in.until})
	if err != nil {
		return fmt.Errorf("derniere execution : %w", err)
	}
	section.Healthy = last.Status == "success" && in.until.Sub(success) <= time.Duration(source.ExpectedHours)*time.Hour
	out.Backups = section
	return nil
}

func collectDomain(ctx context.Context, q *db.Queries, in reportInput, out *ReportData) error {
	rows, err := q.ListProjectDomains(ctx, in.projectID)
	if err != nil {
		return fmt.Errorf("domaines du projet : %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	section := &ReportDomains{Items: make([]ReportDomain, 0, len(rows))}
	for _, d := range rows {
		item := ReportDomain{Domain: d.Domain, ExpiresAt: d.ExpiresAt}
		if d.ExpiresAt != nil {
			left := daysLeft(*d.ExpiresAt, in.now)
			item.DaysLeft = &left
		}
		section.Items = append(section.Items, item)
	}
	out.Domain = section
	return nil
}

// plan est la formule d'un projet pour un mois.
type reportPlan struct {
	name     string
	included int32
	sections []string
}

// build collecte le mois d'un projet, section par section, selon sa formule.
func (s *ReportService) build(ctx context.Context, projectID uuid.UUID, month time.Time, plan reportPlan) (ReportData, error) {
	loc := zone.Location()
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	in := reportInput{
		projectID: projectID, month: first, lastDay: first.AddDate(0, 1, -1), included: plan.included,
		since: time.Date(first.Year(), first.Month(), 1, 0, 0, 0, 0, loc),
		until: time.Date(first.Year(), first.Month()+1, 1, 0, 0, 0, 0, loc),
		now:   s.now(),
	}
	var out ReportData
	out.Month = first.Format("2006-01")
	out.Plan.Name, out.Plan.IncludedMinutes = plan.name, plan.included
	wanted := map[string]bool{}
	for _, k := range plan.sections {
		wanted[k] = true
	}
	for _, section := range reportSectionRegistry {
		if !wanted[section.key] {
			continue
		}
		if err := section.collect(ctx, s.q, in, &out); err != nil {
			return out, err
		}
	}
	return out, nil
}

// --- Reglages -----------------------------------------------------------------------

// ReportSettings sont les reglages d'envoi.
type ReportSettings struct {
	AutoSend bool `json:"auto_send"`
	SendDay  int  `json:"send_day"`
	WaitDays int  `json:"wait_days"`
}

// Settings rend les reglages d'envoi.
func (s *ReportService) Settings(ctx context.Context) (ReportSettings, error) {
	row, err := s.q.GetReportSettings(ctx)
	if err != nil {
		return ReportSettings{}, fmt.Errorf("lecture des reglages : %w", err)
	}
	return ReportSettings{AutoSend: row.AutoSend, SendDay: int(row.SendDay), WaitDays: int(row.WaitDays)}, nil
}

// UpdateSettings enregistre les reglages d'envoi.
func (s *ReportService) UpdateSettings(ctx context.Context, in ReportSettings) (ReportSettings, error) {
	details := map[string]any{}
	if in.SendDay < 1 || in.SendDay > 28 {
		details["send_day"] = "Entre le 1er et le 28"
	}
	if in.WaitDays < 0 || in.WaitDays > 10 {
		details["wait_days"] = "Entre 0 et 10 jours"
	}
	if len(details) > 0 {
		return ReportSettings{}, domain.ErrValidation.WithDetails(details)
	}
	row, err := s.q.UpdateReportSettings(ctx, db.UpdateReportSettingsParams{AutoSend: in.AutoSend, SendDay: int32(in.SendDay), WaitDays: int32(in.WaitDays)})
	if err != nil {
		return ReportSettings{}, fmt.Errorf("enregistrement des reglages : %w", err)
	}
	return ReportSettings{AutoSend: row.AutoSend, SendDay: int(row.SendDay), WaitDays: int(row.WaitDays)}, nil
}

// --- Generation ----------------------------------------------------------------------

// Generate cree les brouillons d'un mois, pour tous les projets sous formule
// qui n'en ont pas, ou pour un seul. Sans attendre la photo d'Umami : c'est
// le geste d'un admin, qui sait ce qu'il demande.
func (s *ReportService) Generate(ctx context.Context, month time.Time, projectID *uuid.UUID) (int, error) {
	return s.generate(ctx, month, projectID, false)
}

func (s *ReportService) generate(ctx context.Context, month time.Time, projectID *uuid.UUID, waitAudience bool) (int, error) {
	first, last := monthBounds(month)
	due, err := s.q.ListReportsDue(ctx, db.ListReportsDueParams{Month: first, MonthEnd: last, ProjectID: projectID})
	if err != nil {
		return 0, fmt.Errorf("projets a rapporter : %w", err)
	}
	// Un projet en erreur ne prive pas les autres de leur rapport : l'ordre de
	// la liste est stable, il bloquerait les memes a chaque passage.
	var errs []error
	created := 0
	for _, d := range due {
		if waitAudience {
			// La photo d'Umami n'est pas encore la : on l'attend, le temps
			// que dit le reglage.
			has, err := s.q.ReportHasAudienceSite(ctx, d.ProjectID)
			if err != nil {
				errs = append(errs, fmt.Errorf("site d'audience du projet %s : %w", d.ProjectID, err))
				continue
			}
			if has {
				if _, err := s.q.ReportAudienceSnapshot(ctx, db.ReportAudienceSnapshotParams{ProjectID: d.ProjectID, Month: first}); errors.Is(err, pgx.ErrNoRows) {
					continue
				}
			}
		}
		if err := s.createDraft(ctx, d.ProjectID, first, reportPlan{name: d.PlanName, included: d.IncludedMinutes, sections: d.Sections}); err != nil {
			errs = append(errs, fmt.Errorf("projet %s : %w", d.ProjectID, err))
			continue
		}
		created++
	}
	return created, errors.Join(errs...)
}

func (s *ReportService) createDraft(ctx context.Context, projectID uuid.UUID, month time.Time, plan reportPlan) error {
	data, err := s.build(ctx, projectID, month, plan)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	id, err := s.q.InsertReport(ctx, db.InsertReportParams{
		ProjectID: projectID, Month: month, PlanName: plan.name, IncludedMinutes: plan.included, Sections: plan.sections, Payload: payload,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("brouillon du rapport : %w", err)
	}
	_, err = s.updateHoldReasons(ctx, id)
	return err
}

// Ce qui empeche un brouillon de partir tout seul. Le code est stable : le
// front en fait une phrase.
const (
	holdNoRecipients        = "no_recipients"
	holdAvailabilityMissing = "availability_missing"
	holdIncident            = "incident"
	holdBackupMissing       = "backup_missing"
	holdBackupStale         = "backup_stale"
	holdDomainMissing       = "domain_missing"
	holdDomainExpiring      = "domain_expiring"
	holdUpdatesMissing      = "updates_missing"
	holdUpdateFailed        = "update_failed"

	// domainHoldDays : a moins d'un mois de l'echeance, un humain regarde.
	domainHoldDays = 30
)

// holdReasonsOf dit ce qui retient un brouillon de l'envoi automatique. Dans
// le doute il retient : un rapport en retard se rattrape, un rapport faux
// parti chez le client, non. L'audience ne retient jamais : ce n'est pas une
// preuve de maintenance, et Umami ne doit pas pouvoir bloquer l'envoi.
func holdReasonsOf(data ReportData, sections []string, recipients int) []string {
	reasons := []string{}
	if recipients == 0 {
		reasons = append(reasons, holdNoRecipients)
	}
	wanted := map[string]bool{}
	for _, k := range sections {
		wanted[k] = true
	}
	if wanted["availability"] {
		if data.Availability == nil || !availabilityComplete(data.Availability.Sites) {
			reasons = append(reasons, holdAvailabilityMissing)
		}
		if data.Availability != nil && len(data.Availability.Incidents) > 0 {
			reasons = append(reasons, holdIncident)
		}
	}
	if wanted["backups"] {
		switch {
		case data.Backups == nil:
			reasons = append(reasons, holdBackupMissing)
		case !data.Backups.Healthy:
			reasons = append(reasons, holdBackupStale)
		}
	}
	if wanted["domain"] {
		reasons = append(reasons, domainHoldReasons(data.Domain)...)
	}
	if wanted["updates"] {
		// Un mois sans la moindre mise a jour est rare : le robot ou la CI est
		// peut-etre en panne, ca merite un regard.
		if data.Updates == nil || (data.Updates.DependencyCount == 0 && !hasProductionRun(data.Updates.Runs)) {
			reasons = append(reasons, holdUpdatesMissing)
		}
		if data.Updates != nil && data.Updates.LastRunFailed {
			reasons = append(reasons, holdUpdateFailed)
		}
	}
	return reasons
}

func hasProductionRun(runs []ReportUpdateRun) bool {
	for _, r := range runs {
		if r.Environment == "prod" {
			return true
		}
	}
	return false
}

func domainHoldReasons(section *ReportDomains) []string {
	if section == nil {
		return []string{holdDomainMissing}
	}
	missing, expiring := false, false
	for _, d := range section.Items {
		if d.DaysLeft == nil {
			missing = true
		} else if *d.DaysLeft < domainHoldDays {
			expiring = true
		}
	}
	reasons := []string{}
	if missing {
		reasons = append(reasons, holdDomainMissing)
	}
	if expiring {
		reasons = append(reasons, holdDomainExpiring)
	}
	return reasons
}

// availabilityComplete : chaque site de production a une mesure chaque jour du
// mois. Un trou ferait passer un releve en panne pour un mois sans incident.
func availabilityComplete(sites []ReportSite) bool {
	production := 0
	for _, site := range sites {
		if site.Environment != "prod" {
			continue
		}
		production++
		if site.Uptime == nil {
			return false
		}
		for _, d := range site.Days {
			if d.Uptime == nil {
				return false
			}
		}
	}
	return production > 0
}

// updateHoldReasons recalcule ce qui retient un brouillon.
func (s *ReportService) updateHoldReasons(ctx context.Context, id uuid.UUID) ([]string, error) {
	row, data, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	recipients, err := s.recipients(ctx, row.ProjectID)
	if err != nil {
		return nil, err
	}
	reasons := holdReasonsOf(data, row.Sections, len(recipients))
	if err := s.q.SetReportHoldReasons(ctx, db.SetReportHoldReasonsParams{ID: id, HoldReasons: reasons}); err != nil {
		return nil, fmt.Errorf("motifs de retenue : %w", err)
	}
	return reasons, nil
}

// refreshDraft remet un brouillon a jour : ses donnees, puis ce qui le retient.
func (s *ReportService) refreshDraft(ctx context.Context, id uuid.UUID) ([]string, error) {
	if err := s.rebuild(ctx, id); err != nil {
		return nil, err
	}
	return s.updateHoldReasons(ctx, id)
}

// draftRefreshEvery : un brouillon suit ce qui se saisit encore sur son mois.
const draftRefreshEvery = time.Hour

// Tick est le passage de la tache : les brouillons du mois ecoule des le 1er,
// en attendant la photo d'Umami le temps regle ; leur mise a jour ; si l'envoi
// automatique est active, l'envoi le jour dit ; et l'alerte de ce qui reste a
// un admin. Une etape en echec n'empeche pas les suivantes.
func (s *ReportService) Tick(ctx context.Context) error {
	return s.TickAt(ctx, s.now())
}

// TickAt est le passage de la tache a un instant donne.
func (s *ReportService) TickAt(ctx context.Context, now time.Time) error {
	settings, err := s.q.GetReportSettings(ctx)
	if err != nil {
		return fmt.Errorf("lecture des reglages : %w", err)
	}
	local := now.In(zone.Location())
	month := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	waited := local.Day()-1 >= int(settings.WaitDays)

	var errs []error
	if _, err := s.generate(ctx, month, nil, !waited); err != nil {
		errs = append(errs, err)
	}
	if err := s.refreshDrafts(ctx, month, now); err != nil {
		errs = append(errs, err)
	}
	if settings.AutoSend && s.canSend && local.Day() >= int(settings.SendDay) {
		if err := s.autoSend(ctx, month); err != nil {
			errs = append(errs, err)
		}
	}
	if err := s.alert(ctx, settings, month, local, waited, now); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (s *ReportService) refreshDrafts(ctx context.Context, month, now time.Time) error {
	ids, err := s.q.ListDraftsToRefresh(ctx, db.ListDraftsToRefreshParams{Month: month, Before: now.Add(-draftRefreshEvery)})
	if err != nil {
		return fmt.Errorf("brouillons a remettre a jour : %w", err)
	}
	var errs []error
	for _, id := range ids {
		if _, err := s.refreshDraft(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("mise a jour du rapport %s : %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// autoSend envoie ce qui peut l'etre. Un brouillon est remis a jour d'abord :
// ce qui part est l'etat du jour, et ce qui le retient est verifie sur cet
// etat. Ce qu'un admin a valide lui-meme part tel quel.
func (s *ReportService) autoSend(ctx context.Context, month time.Time) error {
	pending, err := s.q.ListReportsToSend(ctx, month)
	if err != nil {
		return fmt.Errorf("rapports a envoyer : %w", err)
	}
	var errs []error
	for _, r := range pending {
		if r.Status == "draft" {
			reasons, err := s.refreshDraft(ctx, r.ID)
			if err != nil {
				errs = append(errs, fmt.Errorf("mise a jour du rapport %s : %w", r.ID, err))
				continue
			}
			if len(reasons) > 0 {
				continue
			}
		}
		row, data, err := s.load(ctx, r.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("lecture du rapport %s : %w", r.ID, err))
			continue
		}
		if _, err := s.deliver(ctx, row, data); err != nil {
			// Un rapport qui n'a pas pu partir reste compte dans l'alerte.
			s.log.Warn("envoi automatique du rapport", "report", r.ID, "error", err)
		}
	}
	return errors.Join(errs...)
}

// ReportAttentionPayload est ce que dit l'alerte aux admins.
type ReportAttentionPayload struct {
	Month string `json:"month"`
	// Projets sous formule sans rapport, l'attente de l'audience passee : ils
	// ont echoue, ou leur generation n'a pas encore tourne.
	Missing int `json:"missing"`
	// Rapports du mois ecoule pas encore partis, le jour d'envoi passe.
	Unsent int `json:"unsent"`
	// Parmi eux, ceux que l'envoi automatique ne prendra pas.
	Retained int `json:"retained"`
	// Rapports des deux mois d'avant, jamais partis.
	Overdue int `json:"overdue"`
	// Rapports dont un e-mail a ete abandonne.
	Undelivered int `json:"undelivered"`
	// L'envoi automatique est actif, mais rien ne peut partir sans SMTP.
	SMTPMissing bool `json:"smtp_missing"`
}

func (p ReportAttentionPayload) empty() bool {
	return p.Missing == 0 && p.Unsent == 0 && p.Overdue == 0 && p.Undelivered == 0 && !p.SMTPMissing
}

// worseThan dit si la situation s'est aggravee : ca se dit tout de suite. Une
// situation qui s'ameliore pendant qu'un admin la traite ne se redit pas.
func (p ReportAttentionPayload) worseThan(prev ReportAttentionPayload) bool {
	return p.Month != prev.Month || p.Missing > prev.Missing || p.Unsent > prev.Unsent || p.Retained > prev.Retained ||
		p.Overdue > prev.Overdue || p.Undelivered > prev.Undelivered || (p.SMTPMissing && !prev.SMTPMissing)
}

// reportAlertEvery : une situation inchangee se rappelle une fois par jour.
const reportAlertEvery = 24 * time.Hour

// alert previent les admins de ce qui ne se reglera pas tout seul. Sans ca, un
// rapport qui ne part pas ne se voit que dans les logs, ou dans un ecran que
// personne n'a de raison d'ouvrir.
func (s *ReportService) alert(ctx context.Context, settings db.ReportSetting, month, local time.Time, waited bool, now time.Time) error {
	counts, err := s.q.ReportAttention(ctx, month)
	if err != nil {
		return fmt.Errorf("etat des rapports : %w", err)
	}
	p := ReportAttentionPayload{
		Month: month.Format("2006-01"), Overdue: int(counts.Overdue), Undelivered: int(counts.Undelivered),
		SMTPMissing: settings.AutoSend && !s.canSend,
	}
	if waited {
		first, last := monthBounds(month)
		due, err := s.q.ListReportsDue(ctx, db.ListReportsDueParams{Month: first, MonthEnd: last})
		if err != nil {
			return fmt.Errorf("projets a rapporter : %w", err)
		}
		p.Missing = len(due)
	}
	// Avant le jour d'envoi, un brouillon a relire est dans l'ordre des choses.
	if local.Day() >= int(settings.SendDay) {
		p.Unsent, p.Retained = int(counts.Unsent), int(counts.Retained)
	}
	if p.empty() {
		return nil
	}

	last, err := s.q.LastReportAttention(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("derniere alerte : %w", err)
	}
	if err == nil {
		var prev ReportAttentionPayload
		if err := json.Unmarshal(last.Payload, &prev); err != nil {
			return fmt.Errorf("derniere alerte illisible : %w", err)
		}
		if !p.worseThan(prev) && now.Sub(last.CreatedAt) < reportAlertEvery {
			return nil
		}
	}

	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("alerte des rapports : %w", err)
	}
	admins, err := s.q.ListReportAdmins(ctx)
	if err != nil {
		return fmt.Errorf("destinataires de l'alerte : %w", err)
	}
	// Sans admin actif, l'alerte n'arrive a personne : au moins, le journal le dit.
	if len(admins) == 0 {
		s.log.Warn("rapports mensuels a traiter, mais aucun admin a prevenir", "month", p.Month)
	}
	for _, admin := range admins {
		if _, err := s.q.CreateNotification(ctx, db.CreateNotificationParams{UserID: admin.ID, Kind: "report_attention", Payload: payload}); err != nil {
			return fmt.Errorf("alerte des rapports : %w", err)
		}
	}
	return nil
}

// --- Relecture et envoi ------------------------------------------------------------------

// ReportView est un rapport, pour l'agence.
type ReportView struct {
	ID          uuid.UUID      `json:"id"`
	Project     DeliverableRef `json:"project"`
	ClientName  string         `json:"client_name"`
	Month       string         `json:"month"`
	Status      string         `json:"status"`
	Sections    []string       `json:"sections"`
	Data        ReportData     `json:"data"`
	Editorial   string         `json:"editorial"`
	GeneratedAt time.Time      `json:"generated_at"`
	ValidatedAt *time.Time     `json:"validated_at"`
	SentAt      *time.Time     `json:"sent_at"`
	// Envoye : a qui il est parti. Sinon : a qui il partirait.
	Recipients []string `json:"recipients"`
	// Retenu par un admin : l'envoi automatique ne le prend pas.
	Held bool `json:"held"`
	// Ce qui retient l'envoi automatique.
	HoldReasons []string `json:"hold_reasons"`
	// Les adresses que la file d'envoi a abandonnees.
	Undelivered []string `json:"undelivered"`
	// Les e-mails encore en file : « envoye » n'est pas encore « remis ».
	EmailsPending int `json:"emails_pending"`
}

// ReportListItem est une ligne de la liste du mois.
type ReportListItem struct {
	ID           uuid.UUID      `json:"id"`
	Project      DeliverableRef `json:"project"`
	ClientName   string         `json:"client_name"`
	Status       string         `json:"status"`
	PlanName     string         `json:"plan_name"`
	HasEditorial bool           `json:"has_editorial"`
	GeneratedAt  time.Time      `json:"generated_at"`
	SentAt       *time.Time     `json:"sent_at"`
	Held         bool           `json:"held"`
	HoldReasons  []string       `json:"hold_reasons"`
	Undelivered  int            `json:"undelivered"`
}

// ReportPage est la liste des rapports d'un mois.
type ReportPage struct {
	Month  string           `json:"month"`
	Items  []ReportListItem `json:"items"`
	Total  int              `json:"total"`
	Page   int              `json:"page"`
	Counts struct {
		Drafts    int `json:"drafts"`
		Validated int `json:"validated"`
		Sent      int `json:"sent"`
	} `json:"counts"`
	// Projets sous formule ce mois-la sans rapport encore.
	Pending int `json:"pending"`
}

// List rend les rapports d'un mois, filtres par statut.
func (s *ReportService) List(ctx context.Context, month time.Time, status *string, page, pageSize int) (ReportPage, error) {
	page, pageSize = clampPage(page, pageSize)
	first, last := monthBounds(month)
	total, err := s.q.CountReports(ctx, db.CountReportsParams{Month: first, Status: status})
	if err != nil {
		return ReportPage{}, fmt.Errorf("compte des rapports : %w", err)
	}
	rows, err := s.q.ListReports(ctx, db.ListReportsParams{Month: first, Status: status, Lim: int32(pageSize), Off: int32((page - 1) * pageSize)})
	if err != nil {
		return ReportPage{}, fmt.Errorf("liste des rapports : %w", err)
	}
	counts, err := s.q.CountReportsByStatus(ctx, first)
	if err != nil {
		return ReportPage{}, fmt.Errorf("compte par statut : %w", err)
	}
	due, err := s.q.ListReportsDue(ctx, db.ListReportsDueParams{Month: first, MonthEnd: last})
	if err != nil {
		return ReportPage{}, fmt.Errorf("projets a rapporter : %w", err)
	}
	out := ReportPage{Month: first.Format("2006-01"), Items: make([]ReportListItem, 0, len(rows)), Total: int(total), Page: page, Pending: len(due)}
	out.Counts.Drafts, out.Counts.Validated, out.Counts.Sent = int(counts.Drafts), int(counts.Validated), int(counts.Sent)
	for _, r := range rows {
		item := ReportListItem{
			ID: r.ID, Project: DeliverableRef{ID: r.ProjectID, Name: r.ProjectName}, Status: r.Status, PlanName: r.PlanName,
			HasEditorial: r.HasEditorial, GeneratedAt: r.GeneratedAt, SentAt: r.SentAt,
			Held: r.Held, HoldReasons: r.HoldReasons, Undelivered: len(r.Undelivered),
		}
		if r.ClientName != nil {
			item.ClientName = *r.ClientName
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func (s *ReportService) load(ctx context.Context, id uuid.UUID) (db.GetReportRow, ReportData, error) {
	row, err := s.q.GetReport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, ReportData{}, domain.ErrNotFound
	}
	if err != nil {
		return row, ReportData{}, fmt.Errorf("lecture du rapport : %w", err)
	}
	var data ReportData
	if err := json.Unmarshal(row.Payload, &data); err != nil {
		return row, ReportData{}, fmt.Errorf("rapport illisible : %w", err)
	}
	data.Project.Name = row.ProjectName
	if row.ClientName != nil {
		data.Project.ClientName = *row.ClientName
	}
	return row, data, nil
}

// Get rend un rapport, et a qui il partirait s'il n'est pas encore envoye.
func (s *ReportService) Get(ctx context.Context, id uuid.UUID) (ReportView, error) {
	row, data, err := s.load(ctx, id)
	if err != nil {
		return ReportView{}, err
	}
	out := ReportView{
		ID: row.ID, Project: DeliverableRef{ID: row.ProjectID, Name: row.ProjectName}, ClientName: data.Project.ClientName,
		Month: row.Month.Format("2006-01"), Status: row.Status, Sections: row.Sections, Data: data, Editorial: row.Editorial,
		GeneratedAt: row.GeneratedAt, ValidatedAt: row.ValidatedAt, SentAt: row.SentAt, Recipients: row.Recipients,
		Held: row.Held, HoldReasons: row.HoldReasons, Undelivered: row.Undelivered,
	}
	if row.Status == "sent" {
		pending, err := s.q.CountReportEmailsPending(ctx, &row.ID)
		if err != nil {
			return ReportView{}, fmt.Errorf("e-mails en file : %w", err)
		}
		out.EmailsPending = int(pending)
	}
	if row.Status != "sent" {
		recipients, err := s.recipients(ctx, row.ProjectID)
		if err != nil {
			return ReportView{}, err
		}
		out.Recipients = make([]string, 0, len(recipients))
		for _, r := range recipients {
			out.Recipients = append(out.Recipients, r.email)
		}
	}
	return out, nil
}

var errReportLocked = domain.ErrConflict.WithMessage("Ce rapport n’est plus un brouillon : repassez-le en brouillon pour le modifier")

// SetEditorial enregistre le mot du mois d'un brouillon.
func (s *ReportService) SetEditorial(ctx context.Context, id uuid.UUID, text string) (ReportView, error) {
	text = strings.TrimSpace(text)
	if len([]rune(text)) > 4000 {
		return ReportView{}, domain.ErrValidation.WithDetails(map[string]any{"editorial": "4 000 caractères au plus"})
	}
	n, err := s.q.SetReportEditorial(ctx, db.SetReportEditorialParams{ID: id, Editorial: text})
	if err != nil {
		return ReportView{}, fmt.Errorf("mot du mois : %w", err)
	}
	if n == 0 {
		return ReportView{}, s.lockedOrMissing(ctx, id)
	}
	return s.Get(ctx, id)
}

// Regenerate refait les donnees d'un brouillon, avec la formule du mois telle
// qu'elle est : une intervention saisie en retard, une photo d'Umami arrivee
// depuis.
func (s *ReportService) Regenerate(ctx context.Context, id uuid.UUID) (ReportView, error) {
	if _, err := s.refreshDraft(ctx, id); err != nil {
		return ReportView{}, err
	}
	return s.Get(ctx, id)
}

func (s *ReportService) rebuild(ctx context.Context, id uuid.UUID) error {
	row, _, err := s.load(ctx, id)
	if err != nil {
		return err
	}
	if row.Status != "draft" {
		return errReportLocked
	}
	first, last := monthBounds(row.Month)
	plan := reportPlan{name: row.PlanName, included: row.IncludedMinutes, sections: row.Sections}
	if p, err := s.q.GetProjectPlanForMonth(ctx, db.GetProjectPlanForMonthParams{ProjectID: row.ProjectID, Month: first, MonthEnd: last}); err == nil {
		plan = reportPlan{name: p.PlanName, included: p.IncludedMinutes, sections: p.Sections}
	}
	data, err := s.build(ctx, row.ProjectID, first, plan)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("donnees du rapport : %w", err)
	}
	n, err := s.q.RegenerateReport(ctx, db.RegenerateReportParams{ID: id, PlanName: plan.name, IncludedMinutes: plan.included, Sections: plan.sections, Payload: payload})
	if err != nil {
		return fmt.Errorf("regeneration du rapport : %w", err)
	}
	if n == 0 {
		return errReportLocked
	}
	return nil
}

// Validate fige un brouillon : il est pret a partir.
//
// seen est la version que l'admin a sous les yeux : les donnees d'un brouillon
// suivent ce qui se saisit encore, il ne valide pas ce qu'il n'a pas lu.
func (s *ReportService) Validate(ctx context.Context, id, userID uuid.UUID, seen time.Time) (ReportView, error) {
	n, err := s.q.ValidateReport(ctx, db.ValidateReportParams{ID: id, UserID: &userID, Seen: seen})
	if err != nil {
		return ReportView{}, fmt.Errorf("validation du rapport : %w", err)
	}
	if n == 0 {
		row, _, err := s.load(ctx, id)
		if err != nil {
			return ReportView{}, err
		}
		if row.Status == "draft" {
			return ReportView{}, domain.ErrConflict.WithMessage("Ses données ont changé depuis votre lecture : relisez-le avant de le valider")
		}
		return ReportView{}, errReportLocked
	}
	return s.Get(ctx, id)
}

// Unvalidate repasse un rapport valide, pas encore envoye, en brouillon.
func (s *ReportService) Unvalidate(ctx context.Context, id uuid.UUID) (ReportView, error) {
	n, err := s.q.UnvalidateReport(ctx, id)
	if err != nil {
		return ReportView{}, fmt.Errorf("retour en brouillon : %w", err)
	}
	if n == 0 {
		row, _, err := s.load(ctx, id)
		if err != nil {
			return ReportView{}, err
		}
		if row.Status == "sent" {
			return ReportView{}, domain.ErrConflict.WithMessage("Ce rapport est envoyé : il ne change plus")
		}
		return ReportView{}, domain.ErrConflict.WithMessage("Ce rapport est déjà un brouillon")
	}
	return s.Get(ctx, id)
}

func (s *ReportService) lockedOrMissing(ctx context.Context, id uuid.UUID) error {
	if _, _, err := s.load(ctx, id); err != nil {
		return err
	}
	return errReportLocked
}

type reportRecipient struct {
	email, firstname string
}

// recipients : les comptes du portail du client, moins les exclus, plus les
// adresses du projet. Sans doublon.
func (s *ReportService) recipients(ctx context.Context, projectID uuid.UUID) ([]reportRecipient, error) {
	contacts, err := s.q.ListProjectReportContacts(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("comptes du client : %w", err)
	}
	extras, err := s.q.ListProjectReportExtraRecipients(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("adresses du projet : %w", err)
	}
	seen := map[string]bool{}
	var out []reportRecipient
	for _, c := range contacts {
		if c.Excluded || seen[strings.ToLower(c.Email)] {
			continue
		}
		seen[strings.ToLower(c.Email)] = true
		out = append(out, reportRecipient{email: c.Email, firstname: c.Firstname})
	}
	for _, e := range extras {
		if seen[strings.ToLower(e.Email)] {
			continue
		}
		seen[strings.ToLower(e.Email)] = true
		out = append(out, reportRecipient{email: e.Email})
	}
	return out, nil
}

// Send envoie un rapport valide : un e-mail par destinataire, dans la file
// d'envoi, et le rapport ne change plus.
func (s *ReportService) Send(ctx context.Context, id uuid.UUID) (ReportView, error) {
	row, data, err := s.load(ctx, id)
	if err != nil {
		return ReportView{}, err
	}
	switch row.Status {
	case "sent":
		return ReportView{}, domain.ErrConflict.WithMessage("Ce rapport est déjà envoyé")
	case "draft":
		return ReportView{}, domain.ErrConflict.WithMessage("Validez le rapport avant de l’envoyer")
	}
	return s.deliver(ctx, row, data)
}

// deliver met les e-mails en file et marque le rapport envoye, dans une seule
// transaction : un rapport marque envoye a ses e-mails en file, et inversement.
// Un brouillon, venu de l'envoi automatique, est valide dans la meme : s'il ne
// part pas, il reste brouillon et ses motifs seront revus.
func (s *ReportService) deliver(ctx context.Context, row db.GetReportRow, data ReportData) (ReportView, error) {
	if row.ProjectDeletedAt != nil {
		return ReportView{}, domain.ErrConflict.WithMessage("Le projet de ce rapport est supprimé")
	}
	if !s.canSend {
		return ReportView{}, domain.ErrConflict.WithMessage("L’envoi d’e-mails n’est pas configuré (SMTP)")
	}
	recipients, err := s.recipients(ctx, row.ProjectID)
	if err != nil {
		return ReportView{}, err
	}
	if len(recipients) == 0 {
		return ReportView{}, domain.ErrValidation.WithMessage("Aucun destinataire : invitez un contact au portail ou ajoutez une adresse au projet")
	}

	summary := reportMailOf(data, row.Editorial)
	url := fmt.Sprintf("%s/client/rapports/%s", s.clientURL, row.ID)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReportView{}, fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if row.Status == "draft" {
		n, err := q.AutoValidateReport(ctx, row.ID)
		if err != nil {
			return ReportView{}, fmt.Errorf("validation automatique : %w", err)
		}
		if n == 0 {
			return ReportView{}, domain.ErrConflict.WithMessage("Le rapport a changé entre-temps : rechargez-le")
		}
	}
	emails := make([]string, 0, len(recipients))
	for _, r := range recipients {
		msg, err := mailer.MonthlyReport(r.email, r.firstname, summary, url)
		if err != nil {
			return ReportView{}, fmt.Errorf("e-mail du rapport : %w", err)
		}
		if err := enqueueReport(ctx, q, row.ID, msg); err != nil {
			return ReportView{}, err
		}
		emails = append(emails, r.email)
	}
	n, err := q.MarkReportSent(ctx, db.MarkReportSentParams{ID: row.ID, Recipients: emails})
	if err != nil {
		return ReportView{}, fmt.Errorf("envoi du rapport : %w", err)
	}
	if n == 0 {
		return ReportView{}, domain.ErrConflict.WithMessage("Le rapport a changé entre-temps : rechargez-le")
	}
	if err := tx.Commit(ctx); err != nil {
		return ReportView{}, fmt.Errorf("envoi du rapport : %w", err)
	}
	return s.Get(ctx, row.ID)
}

func enqueueReport(ctx context.Context, q *db.Queries, id uuid.UUID, msg mailer.Message) error {
	if err := q.EnqueueReportEmail(ctx, db.EnqueueReportEmailParams{
		Kind: msg.Kind, ToAddress: msg.To, Subject: msg.Subject, TextBody: msg.Text, HtmlBody: msg.HTML,
		ReplyTo: msg.ReplyTo, MessageID: msg.MessageID, InReplyTo: msg.InReplyTo, ReportID: &id,
	}); err != nil {
		return fmt.Errorf("mise en file de l'e-mail : %w", err)
	}
	return nil
}

// Resend remet en file les e-mails abandonnes d'un rapport envoye. Le rapport
// ne change pas : seules les adresses qui ne l'ont pas recu le recoivent.
func (s *ReportService) Resend(ctx context.Context, id uuid.UUID) (ReportView, error) {
	row, data, err := s.load(ctx, id)
	if err != nil {
		return ReportView{}, err
	}
	if row.Status != "sent" || len(row.Undelivered) == 0 {
		return ReportView{}, domain.ErrConflict.WithMessage("Aucun e-mail de ce rapport n’est à relancer")
	}
	if !s.canSend {
		return ReportView{}, domain.ErrConflict.WithMessage("L’envoi d’e-mails n’est pas configuré (SMTP)")
	}
	recipients, err := s.recipients(ctx, row.ProjectID)
	if err != nil {
		return ReportView{}, err
	}
	firstnames := make(map[string]string, len(recipients))
	for _, r := range recipients {
		firstnames[strings.ToLower(r.email)] = r.firstname
	}
	summary := reportMailOf(data, row.Editorial)
	url := fmt.Sprintf("%s/client/rapports/%s", s.clientURL, row.ID)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReportView{}, fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	undelivered, err := q.LockReportUndelivered(ctx, id)
	if err != nil {
		return ReportView{}, fmt.Errorf("relance du rapport : %w", err)
	}
	for _, address := range undelivered {
		msg, err := mailer.MonthlyReport(address, firstnames[strings.ToLower(address)], summary, url)
		if err != nil {
			return ReportView{}, fmt.Errorf("e-mail du rapport : %w", err)
		}
		if err := enqueueReport(ctx, q, id, msg); err != nil {
			return ReportView{}, err
		}
	}
	n, err := q.ClearReportUndelivered(ctx, id)
	if err != nil {
		return ReportView{}, fmt.Errorf("relance du rapport : %w", err)
	}
	if n == 0 {
		return ReportView{}, domain.ErrConflict.WithMessage("Le rapport a changé entre-temps : rechargez-le")
	}
	if err := tx.Commit(ctx); err != nil {
		return ReportView{}, fmt.Errorf("relance du rapport : %w", err)
	}
	return s.Get(ctx, id)
}

// --- Destinataires d'un projet -----------------------------------------------------------

// ReportRecipientsView est la carte « Destinataires » d'un projet.
type ReportRecipientsView struct {
	Contacts []ReportContact `json:"contacts"`
	Extra    []string        `json:"extra"`
	// Le rapport de ce projet ne part jamais tout seul.
	ManualReview bool `json:"manual_review"`
}

// ReportContact est un compte du portail du client.
type ReportContact struct {
	Person   *Person `json:"person"`
	Email    string  `json:"email"`
	Excluded bool    `json:"excluded"`
}

// Recipients rend les destinataires des rapports d'un projet.
func (s *ReportService) Recipients(ctx context.Context, projectID uuid.UUID) (ReportRecipientsView, error) {
	exists, err := s.q.ReportProjectExists(ctx, projectID)
	if err != nil {
		return ReportRecipientsView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ReportRecipientsView{}, domain.ErrNotFound
	}
	contacts, err := s.q.ListProjectReportContacts(ctx, projectID)
	if err != nil {
		return ReportRecipientsView{}, fmt.Errorf("comptes du client : %w", err)
	}
	extras, err := s.q.ListProjectReportExtraRecipients(ctx, projectID)
	if err != nil {
		return ReportRecipientsView{}, fmt.Errorf("adresses du projet : %w", err)
	}
	manual, err := s.q.IsReportManualReview(ctx, projectID)
	if err != nil {
		return ReportRecipientsView{}, fmt.Errorf("relecture du projet : %w", err)
	}
	out := ReportRecipientsView{Contacts: make([]ReportContact, 0, len(contacts)), Extra: make([]string, 0, len(extras)), ManualReview: manual}
	for _, c := range contacts {
		out.Contacts = append(out.Contacts, ReportContact{Person: personOf(c.ID, c.Firstname, c.Lastname, c.AvatarUrl), Email: c.Email, Excluded: c.Excluded})
	}
	for _, e := range extras {
		out.Extra = append(out.Extra, e.Email)
	}
	return out, nil
}

// SetRecipients remplace les exclusions, les adresses ajoutees et la relecture
// obligatoire d'un projet.
func (s *ReportService) SetRecipients(ctx context.Context, projectID uuid.UUID, excluded []uuid.UUID, extra []string, manualReview bool) (ReportRecipientsView, error) {
	exists, err := s.q.ReportProjectExists(ctx, projectID)
	if err != nil {
		return ReportRecipientsView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ReportRecipientsView{}, domain.ErrNotFound
	}
	if len(extra) > 10 {
		return ReportRecipientsView{}, domain.ErrValidation.WithDetails(map[string]any{"extra": "10 adresses au plus"})
	}
	clean := make([]string, 0, len(extra))
	for _, e := range extra {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if addr, err := mail.ParseAddress(e); err != nil || addr.Address != e || !strings.Contains(e[strings.Index(e, "@"):], ".") {
			return ReportRecipientsView{}, domain.ErrValidation.WithDetails(map[string]any{"extra": "Adresse invalide : " + e})
		}
		clean = append(clean, e)
	}
	sort.Strings(clean)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReportRecipientsView{}, fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.DeleteProjectReportOptOuts(ctx, projectID); err != nil {
		return ReportRecipientsView{}, fmt.Errorf("exclusions : %w", err)
	}
	for _, u := range excluded {
		if err := q.InsertProjectReportOptOut(ctx, db.InsertProjectReportOptOutParams{ProjectID: projectID, UserID: u}); err != nil {
			if isForeignKeyViolation(err) {
				return ReportRecipientsView{}, domain.ErrValidation.WithDetails(map[string]any{"excluded": "Compte inconnu"})
			}
			return ReportRecipientsView{}, fmt.Errorf("exclusion : %w", err)
		}
	}
	if err := q.DeleteProjectReportRecipients(ctx, projectID); err != nil {
		return ReportRecipientsView{}, fmt.Errorf("adresses : %w", err)
	}
	for _, e := range clean {
		if err := q.InsertProjectReportRecipient(ctx, db.InsertProjectReportRecipientParams{ProjectID: projectID, Email: e}); err != nil {
			return ReportRecipientsView{}, fmt.Errorf("adresse : %w", err)
		}
	}
	if manualReview {
		err = q.SetReportManualReview(ctx, projectID)
	} else {
		err = q.UnsetReportManualReview(ctx, projectID)
	}
	if err != nil {
		return ReportRecipientsView{}, fmt.Errorf("relecture du projet : %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ReportRecipientsView{}, err
	}
	return s.Recipients(ctx, projectID)
}

// --- Portail -----------------------------------------------------------------------------

// PortalReportItem est un rapport de l'archive du client.
type PortalReportItem struct {
	ID       uuid.UUID  `json:"id"`
	Month    string     `json:"month"`
	PlanName string     `json:"plan_name"`
	SentAt   *time.Time `json:"sent_at"`
}

// PortalReport est un rapport envoye, pour le client.
type PortalReport struct {
	ID        uuid.UUID      `json:"id"`
	Project   DeliverableRef `json:"project"`
	Month     string         `json:"month"`
	Data      ReportData     `json:"data"`
	Editorial string         `json:"editorial"`
	SentAt    *time.Time     `json:"sent_at"`
}

func (s *ReportService) clientOf(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	clientID, err := s.q.PortalClientOf(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && clientID == nil) {
		return uuid.Nil, domain.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("lecture du compte : %w", err)
	}
	return *clientID, nil
}

// PortalReports rend l'archive des rapports envoyes d'un projet du client.
func (s *ReportService) PortalReports(ctx context.Context, userID, projectID uuid.UUID) ([]PortalReportItem, error) {
	clientID, err := s.clientOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	mine, err := s.q.PortalProjectOfClient(ctx, db.PortalProjectOfClientParams{ProjectID: projectID, ClientID: clientID})
	if err != nil {
		return nil, fmt.Errorf("lecture du projet : %w", err)
	}
	if !mine {
		return nil, domain.ErrNotFound
	}
	rows, err := s.q.ListPortalReports(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("archive des rapports : %w", err)
	}
	out := make([]PortalReportItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, PortalReportItem{ID: r.ID, Month: r.Month.Format("2006-01"), PlanName: r.PlanName, SentAt: r.SentAt})
	}
	return out, nil
}

// PortalReport rend un rapport envoye, pour le client du projet seulement.
func (s *ReportService) PortalReport(ctx context.Context, userID, id uuid.UUID) (PortalReport, error) {
	clientID, err := s.clientOf(ctx, userID)
	if err != nil {
		return PortalReport{}, err
	}
	row, err := s.q.GetPortalReport(ctx, db.GetPortalReportParams{ID: id, ClientID: clientID})
	if errors.Is(err, pgx.ErrNoRows) {
		return PortalReport{}, domain.ErrNotFound
	}
	if err != nil {
		return PortalReport{}, fmt.Errorf("lecture du rapport : %w", err)
	}
	var data ReportData
	if err := json.Unmarshal(row.Payload, &data); err != nil {
		return PortalReport{}, fmt.Errorf("rapport illisible : %w", err)
	}
	data.Project.Name = row.ProjectName
	// Les drapeaux de relecture restent a l'agence : le client lit ce qui s'est
	// passe, pas ce qui a retenu le rapport.
	if data.Updates != nil {
		data.Updates.LastRunFailed = false
	}
	if data.Backups != nil {
		data.Backups.Healthy = false
	}
	return PortalReport{
		ID: row.ID, Project: DeliverableRef{ID: row.ProjectID, Name: row.ProjectName}, Month: row.Month.Format("2006-01"),
		Data: data, Editorial: row.Editorial, SentAt: row.SentAt,
	}, nil
}

// --- E-mail ---------------------------------------------------------------------------------

var frenchMonths = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}

func monthName(month string) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return month
	}
	return fmt.Sprintf("%s %d", frenchMonths[t.Month()-1], t.Year())
}

func dayName(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), frenchMonths[t.Month()-1])
}

// minutesText ecrit une duree comme l'application : « 2h35 », « 45 min ».
func minutesText(m int32) string {
	if m <= 0 {
		return "0 min"
	}
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	if m%60 == 0 {
		return fmt.Sprintf("%dh", m/60)
	}
	return fmt.Sprintf("%dh%02d", m/60, m%60)
}

// numberText ecrit un entier a la francaise : « 3 263 ».
func numberText(n int64) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(" ")
		}
		b.WriteRune(c)
	}
	return b.String()
}

func percentText(v float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
	return strings.Replace(s, ".", ",", 1) + " %"
}

func deltaText(cur, prev int64) (string, bool) {
	if prev == 0 {
		return "", true
	}
	v := float64(cur-prev) / float64(prev) * 100
	s := strings.Replace(fmt.Sprintf("%+.1f", v), ".", ",", 1)
	s = strings.Replace(s, ",0", "", 1)
	return s + " %", v >= 0
}

var interventionLabels = map[string]string{"contenu": "Contenu", "correctif": "Correctif", "mise_a_jour": "Mise à jour", "support": "Support", "autre": "Autre"}

// reportMailOf resume le rapport pour l'e-mail : chiffres, listes, barres.
func reportMailOf(d ReportData, editorial string) mailer.ReportMail {
	out := mailer.ReportMail{Month: monthName(d.Month), ProjectName: d.Project.Name}
	for _, p := range strings.Split(editorial, "\n") {
		if p = strings.TrimSpace(p); p != "" {
			out.Editorial = append(out.Editorial, p)
		}
	}

	if a := d.Availability; a != nil && len(a.Sites) > 0 {
		out.HasAvailability = true
		site := a.Sites[0]
		if site.Uptime != nil {
			out.Figures = append(out.Figures, mailer.ReportFigure{Label: "Disponibilité · " + site.Name, Value: percentText(*site.Uptime)})
		}
		if site.AvgResponseMS != nil {
			out.Figures = append(out.Figures, mailer.ReportFigure{Label: "Temps de réponse moyen", Value: fmt.Sprintf("%d ms", *site.AvgResponseMS)})
		}
		for _, i := range a.Incidents {
			detail := i.Message
			if i.DurationSeconds != nil {
				detail = strings.TrimSpace(fmt.Sprintf("%s · %s", minutesText((*i.DurationSeconds+59)/60), detail))
			}
			out.Incidents = append(out.Incidents, mailer.ReportItem{Date: dayName(i.StartedAt.In(zone.Location())), Label: "Interruption de " + i.Site, Detail: strings.Trim(detail, " ·")})
		}
	}

	if au := d.Audience; au != nil && au.Data != nil {
		t, p := au.Data.Totals, au.Data.Previous
		dv, gv := deltaText(t.Visitors, p.Visitors)
		dp, gp := deltaText(t.Pageviews, p.Pageviews)
		out.Figures = append(out.Figures,
			mailer.ReportFigure{Label: "Visiteurs", Value: numberText(t.Visitors), Delta: dv, DeltaGood: gv},
			mailer.ReportFigure{Label: "Pages vues", Value: numberText(t.Pageviews), Delta: dp, DeltaGood: gp},
		)
		out.Pages = barsOf(au.Data.Pages, func(v string) string {
			if v == "" {
				return "/"
			}
			return v
		})
		out.Sources = barsOf(au.Data.Referrers, func(v string) string {
			if v == "" {
				return "Accès direct"
			}
			return strings.TrimPrefix(v, "www.")
		})
	}

	if iv := d.Interventions; iv != nil {
		out.Consumed = fmt.Sprintf("%s consommées sur %s incluses", minutesText(iv.ConsumedMinutes), minutesText(iv.IncludedMinutes))
		if iv.IncludedMinutes > 0 {
			out.ConsumedRatio = int(min(100, iv.ConsumedMinutes*100/iv.IncludedMinutes))
		} else if iv.ConsumedMinutes > 0 {
			out.ConsumedRatio = 100
		}
		if iv.OverrunMinutes > 0 {
			out.Overrun = "+" + minutesText(iv.OverrunMinutes)
		}
		for _, it := range iv.Items {
			day, _ := time.Parse(dateLayout, it.SpentOn)
			out.Interventions = append(out.Interventions, mailer.ReportItem{Date: dayName(day), Label: interventionLabels[it.Kind], Detail: it.Note})
		}
	}

	if dep := d.Deployments; dep != nil {
		for _, it := range dep.Items {
			out.Deployments = append(out.Deployments, mailer.ReportItem{Date: dayName(it.DeployedAt.In(zone.Location())), Label: it.Name, Detail: it.Environment})
		}
	}
	if u := d.Updates; u != nil {
		if u.DependencyCount > 0 {
			out.Figures = append(out.Figures, mailer.ReportFigure{Label: "Mises à jour de dépendances", Value: numberText(int64(u.DependencyCount))})
		}
		if u.SecurityCount > 0 {
			out.Figures = append(out.Figures, mailer.ReportFigure{Label: "Correctifs de sécurité", Value: numberText(int64(u.SecurityCount))})
		}
		production := 0
		for _, r := range u.Runs {
			if r.Environment == "prod" {
				production++
			}
		}
		if production > 0 {
			out.Figures = append(out.Figures, mailer.ReportFigure{Label: "Mises à jour en production", Value: numberText(int64(production))})
		}
		if preprod := len(u.Runs) - production; preprod > 0 {
			out.Figures = append(out.Figures, mailer.ReportFigure{Label: "Mises à jour en préproduction", Value: numberText(int64(preprod))})
		}
	}
	if b := d.Backups; b != nil {
		out.Figures = append(out.Figures, mailer.ReportFigure{Label: "Sauvegardes réussies", Value: numberText(int64(b.Successes))})
	}
	if domains := d.Domain; domains != nil {
		for _, item := range domains.Items {
			if item.ExpiresAt != nil {
				out.Figures = append(out.Figures, mailer.ReportFigure{Label: "Échéance · " + item.Domain, Value: item.ExpiresAt.In(zone.Location()).Format("02/01/2006")})
			}
		}
	}
	return out
}

func barsOf(rows []StatisticsRow, label func(string) string) []mailer.ReportRow {
	if len(rows) == 0 {
		return nil
	}
	top := rows[:min(len(rows), 5)]
	maxCount := top[0].Count
	for _, r := range top {
		maxCount = max(maxCount, r.Count)
	}
	out := make([]mailer.ReportRow, 0, len(top))
	for _, r := range top {
		ratio := 0
		if maxCount > 0 {
			ratio = int(r.Count * 100 / maxCount)
		}
		out = append(out, mailer.ReportRow{Label: label(r.Value), Value: numberText(r.Count), Ratio: max(ratio, 2)})
	}
	return out
}
