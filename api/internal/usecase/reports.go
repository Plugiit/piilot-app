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

// reportInput est ce qu'une section recoit pour collecter son mois.
type reportInput struct {
	projectID      uuid.UUID
	month, lastDay time.Time
	// Bornes du mois en instants, dans le fuseau de l'agence.
	since, until time.Time
	included     int32
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
	created := 0
	for _, d := range due {
		if waitAudience {
			// La photo d'Umami n'est pas encore la : on l'attend, le temps
			// que dit le reglage.
			has, err := s.q.ReportHasAudienceSite(ctx, d.ProjectID)
			if err != nil {
				return created, fmt.Errorf("site d'audience : %w", err)
			}
			if has {
				if _, err := s.q.ReportAudienceSnapshot(ctx, db.ReportAudienceSnapshotParams{ProjectID: d.ProjectID, Month: first}); errors.Is(err, pgx.ErrNoRows) {
					continue
				}
			}
		}
		if err := s.createDraft(ctx, d.ProjectID, first, reportPlan{name: d.PlanName, included: d.IncludedMinutes, sections: d.Sections}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
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
	if _, err := s.q.InsertReport(ctx, db.InsertReportParams{
		ProjectID: projectID, Month: month, PlanName: plan.name, IncludedMinutes: plan.included, Sections: plan.sections, Payload: payload,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("brouillon du rapport : %w", err)
	}
	return nil
}

// Tick est le passage de la tache : les brouillons du mois ecoule des le 1er,
// en attendant la photo d'Umami le temps regle ; puis, si l'envoi
// automatique est active, l'envoi le jour dit.
func (s *ReportService) Tick(ctx context.Context) error {
	settings, err := s.q.GetReportSettings(ctx)
	if err != nil {
		return fmt.Errorf("lecture des reglages : %w", err)
	}
	local := s.now().In(zone.Location())
	month := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	waited := local.Day()-1 >= int(settings.WaitDays)

	if _, err := s.generate(ctx, month, nil, !waited); err != nil {
		return err
	}
	if !settings.AutoSend || local.Day() < int(settings.SendDay) {
		return nil
	}
	pending, err := s.q.ListReportsToSend(ctx, month)
	if err != nil {
		return fmt.Errorf("rapports a envoyer : %w", err)
	}
	for _, r := range pending {
		if r.Status == "draft" {
			if _, err := s.q.ValidateReport(ctx, db.ValidateReportParams{ID: r.ID}); err != nil {
				return fmt.Errorf("validation automatique : %w", err)
			}
		}
		if _, err := s.Send(ctx, r.ID); err != nil {
			// Un rapport sans destinataire attend un admin ; les autres partent.
			s.log.Warn("envoi automatique du rapport", "report", r.ID, "error", err)
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
	row, _, err := s.load(ctx, id)
	if err != nil {
		return ReportView{}, err
	}
	if row.Status != "draft" {
		return ReportView{}, errReportLocked
	}
	first, last := monthBounds(row.Month)
	plan := reportPlan{name: row.PlanName, included: row.IncludedMinutes, sections: row.Sections}
	if p, err := s.q.GetProjectPlanForMonth(ctx, db.GetProjectPlanForMonthParams{ProjectID: row.ProjectID, Month: first, MonthEnd: last}); err == nil {
		plan = reportPlan{name: p.PlanName, included: p.IncludedMinutes, sections: p.Sections}
	}
	data, err := s.build(ctx, row.ProjectID, first, plan)
	if err != nil {
		return ReportView{}, err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return ReportView{}, err
	}
	n, err := s.q.RegenerateReport(ctx, db.RegenerateReportParams{ID: id, PlanName: plan.name, IncludedMinutes: plan.included, Sections: plan.sections, Payload: payload})
	if err != nil {
		return ReportView{}, fmt.Errorf("regeneration du rapport : %w", err)
	}
	if n == 0 {
		return ReportView{}, errReportLocked
	}
	return s.Get(ctx, id)
}

// Validate fige un brouillon : il est pret a partir.
func (s *ReportService) Validate(ctx context.Context, id, userID uuid.UUID) (ReportView, error) {
	n, err := s.q.ValidateReport(ctx, db.ValidateReportParams{ID: id, UserID: &userID})
	if err != nil {
		return ReportView{}, fmt.Errorf("validation du rapport : %w", err)
	}
	if n == 0 {
		return ReportView{}, s.lockedOrMissing(ctx, id)
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
// d'envoi, et le rapport ne change plus. Le tout dans une transaction : un
// rapport marque envoye a ses e-mails en file, et inversement.
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
	emails := make([]string, 0, len(recipients))
	for _, r := range recipients {
		msg, err := mailer.MonthlyReport(r.email, r.firstname, summary, url)
		if err != nil {
			return ReportView{}, fmt.Errorf("e-mail du rapport : %w", err)
		}
		if err := enqueue(ctx, q, msg); err != nil {
			return ReportView{}, err
		}
		emails = append(emails, r.email)
	}
	n, err := q.MarkReportSent(ctx, db.MarkReportSentParams{ID: id, Recipients: emails})
	if err != nil {
		return ReportView{}, fmt.Errorf("envoi du rapport : %w", err)
	}
	if n == 0 {
		return ReportView{}, domain.ErrConflict.WithMessage("Le rapport a changé entre-temps : rechargez-le")
	}
	if err := tx.Commit(ctx); err != nil {
		return ReportView{}, err
	}
	return s.Get(ctx, id)
}

// --- Destinataires d'un projet -----------------------------------------------------------

// ReportRecipientsView est la carte « Destinataires » d'un projet.
type ReportRecipientsView struct {
	Contacts []ReportContact `json:"contacts"`
	Extra    []string        `json:"extra"`
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
	out := ReportRecipientsView{Contacts: make([]ReportContact, 0, len(contacts)), Extra: make([]string, 0, len(extras))}
	for _, c := range contacts {
		out.Contacts = append(out.Contacts, ReportContact{Person: personOf(c.ID, c.Firstname, c.Lastname, c.AvatarUrl), Email: c.Email, Excluded: c.Excluded})
	}
	for _, e := range extras {
		out.Extra = append(out.Extra, e.Email)
	}
	return out, nil
}

// SetRecipients remplace les exclusions et les adresses ajoutees d'un projet.
func (s *ReportService) SetRecipients(ctx context.Context, projectID uuid.UUID, excluded []uuid.UUID, extra []string) (ReportRecipientsView, error) {
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
