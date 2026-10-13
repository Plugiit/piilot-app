package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/zone"
)

// MaintenanceService porte les formules de maintenance et le journal des
// interventions. Une intervention est une saisie de temps qualifiee : elle se
// cree par le service du temps, ce service-ci la lit.
type MaintenanceService struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewMaintenanceService construit le service.
func NewMaintenanceService(pool *pgxpool.Pool) *MaintenanceService {
	return &MaintenanceService{pool: pool, q: db.New(pool)}
}

// reportSections sont les sections qu'une formule peut mettre dans son
// rapport. Celles de la 1.6 s'affichent quand leurs donnees existent.
var reportSections = []string{"availability", "audience", "interventions", "deployments", "updates", "backups", "domain"}

// --- Formules ---------------------------------------------------------------

// MaintenancePlan est une formule.
type MaintenancePlan struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	IncludedMinutes int       `json:"included_minutes"`
	Sections        []string  `json:"sections"`
	Active          bool      `json:"active"`
}

// MaintenancePlanInput est ce que l'ecran enregistre.
type MaintenancePlanInput struct {
	Name            string
	IncludedMinutes int
	Sections        []string
	Active          bool
}

func planOf(p db.MaintenancePlan) MaintenancePlan {
	return MaintenancePlan{ID: p.ID, Name: p.Name, IncludedMinutes: int(p.IncludedMinutes), Sections: p.Sections, Active: p.Active}
}

func (in MaintenancePlanInput) clean() (MaintenancePlanInput, error) {
	out := in
	out.Name = strings.TrimSpace(in.Name)
	details := map[string]any{}
	if out.Name == "" || len([]rune(out.Name)) > 80 {
		details["name"] = "Un nom de 80 caractères au plus"
	}
	if out.IncludedMinutes < 0 || out.IncludedMinutes > 100000 {
		details["included_minutes"] = "Entre 0 et 100 000 minutes"
	}
	// Les sections dans l'ordre du rapport, sans doublon ni inconnue.
	wanted := map[string]bool{}
	for _, s := range in.Sections {
		wanted[s] = true
	}
	out.Sections = []string{}
	for _, s := range reportSections {
		if wanted[s] {
			out.Sections = append(out.Sections, s)
			delete(wanted, s)
		}
	}
	if len(wanted) > 0 {
		details["sections"] = "Section inconnue"
	}
	if len(details) > 0 {
		return out, domain.ErrValidation.WithDetails(details)
	}
	return out, nil
}

// Plans liste les formules, les actives d'abord.
func (s *MaintenanceService) Plans(ctx context.Context) ([]MaintenancePlan, error) {
	rows, err := s.q.ListMaintenancePlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("liste des formules : %w", err)
	}
	out := make([]MaintenancePlan, 0, len(rows))
	for _, r := range rows {
		out = append(out, planOf(r))
	}
	return out, nil
}

// CreatePlan ajoute une formule.
func (s *MaintenanceService) CreatePlan(ctx context.Context, in MaintenancePlanInput) (MaintenancePlan, error) {
	clean, err := in.clean()
	if err != nil {
		return MaintenancePlan{}, err
	}
	row, err := s.q.CreateMaintenancePlan(ctx, db.CreateMaintenancePlanParams{
		Name: clean.Name, IncludedMinutes: int32(clean.IncludedMinutes), Sections: clean.Sections, Active: clean.Active,
	})
	if isUniqueViolation(err) {
		return MaintenancePlan{}, domain.ErrValidation.WithDetails(map[string]any{"name": "Une formule porte déjà ce nom"})
	}
	if err != nil {
		return MaintenancePlan{}, fmt.Errorf("creation de la formule : %w", err)
	}
	return planOf(row), nil
}

// UpdatePlan modifie une formule. Les mois passes des projets la gardent
// telle qu'elle est maintenant : un rapport envoye, lui, a fige la sienne.
func (s *MaintenanceService) UpdatePlan(ctx context.Context, id uuid.UUID, in MaintenancePlanInput) (MaintenancePlan, error) {
	clean, err := in.clean()
	if err != nil {
		return MaintenancePlan{}, err
	}
	row, err := s.q.UpdateMaintenancePlan(ctx, db.UpdateMaintenancePlanParams{
		ID: id, Name: clean.Name, IncludedMinutes: int32(clean.IncludedMinutes), Sections: clean.Sections, Active: clean.Active,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return MaintenancePlan{}, domain.ErrNotFound
	case isUniqueViolation(err):
		return MaintenancePlan{}, domain.ErrValidation.WithDetails(map[string]any{"name": "Une formule porte déjà ce nom"})
	case err != nil:
		return MaintenancePlan{}, fmt.Errorf("mise a jour de la formule : %w", err)
	}
	return planOf(row), nil
}

// DeletePlan supprime une formule qui n'a jamais servi. Une formule qui a
// servi se desactive : elle reste l'histoire des projets qui l'ont eue.
func (s *MaintenanceService) DeletePlan(ctx context.Context, id uuid.UUID) error {
	used, err := s.q.MaintenancePlanInUse(ctx, id)
	if err != nil {
		return fmt.Errorf("usage de la formule : %w", err)
	}
	if used {
		return domain.ErrConflict.WithMessage("Cette formule a servi à des projets : désactivez-la plutôt")
	}
	n, err := s.q.DeleteMaintenancePlan(ctx, id)
	if err != nil {
		return fmt.Errorf("suppression de la formule : %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Formule d'un projet -------------------------------------------------------

// MaintenancePeriod est une periode de formule d'un projet.
type MaintenancePeriod struct {
	ID              uuid.UUID `json:"id"`
	PlanID          uuid.UUID `json:"plan_id"`
	PlanName        string    `json:"plan_name"`
	IncludedMinutes int       `json:"included_minutes"`
	StartsOn        string    `json:"starts_on"`
	EndsOn          *string   `json:"ends_on"`
}

// ProjectMaintenanceView est l'onglet « Maintenance » d'un projet.
type ProjectMaintenanceView struct {
	// La periode en cours ou a venir, nulle sans formule.
	Current *MaintenancePeriod `json:"current"`
	// Toutes les periodes, la plus recente d'abord.
	History []MaintenancePeriod `json:"history"`
}

// ProjectMaintenance rend la formule d'un projet et son historique.
func (s *MaintenanceService) ProjectMaintenance(ctx context.Context, projectID uuid.UUID) (ProjectMaintenanceView, error) {
	exists, err := s.q.MaintenanceProjectExists(ctx, projectID)
	if err != nil {
		return ProjectMaintenanceView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ProjectMaintenanceView{}, domain.ErrNotFound
	}
	rows, err := s.q.ListProjectMaintenance(ctx, projectID)
	if err != nil {
		return ProjectMaintenanceView{}, fmt.Errorf("formule du projet : %w", err)
	}
	out := ProjectMaintenanceView{History: make([]MaintenancePeriod, 0, len(rows))}
	for _, r := range rows {
		p := MaintenancePeriod{
			ID: r.ID, PlanID: r.PlanID, PlanName: r.PlanName, IncludedMinutes: int(r.IncludedMinutes),
			StartsOn: r.StartsOn.Format(dateLayout),
		}
		if r.EndsOn != nil {
			end := r.EndsOn.Format(dateLayout)
			p.EndsOn = &end
		}
		out.History = append(out.History, p)
		if p.EndsOn == nil && out.Current == nil {
			current := p
			out.Current = &current
		}
	}
	return out, nil
}

// SetProjectMaintenance change la formule d'un projet a partir d'un mois :
// la periode en cours s'arrete la veille, une periode qui n'avait pas
// commence disparait, et la nouvelle s'ouvre. Sans formule, le projet sort
// de la maintenance a partir de ce mois.
func (s *MaintenanceService) SetProjectMaintenance(ctx context.Context, projectID uuid.UUID, planID *uuid.UUID, month time.Time) (ProjectMaintenanceView, error) {
	exists, err := s.q.MaintenanceProjectExists(ctx, projectID)
	if err != nil {
		return ProjectMaintenanceView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return ProjectMaintenanceView{}, domain.ErrNotFound
	}
	// Le temps inclus est mensuel : une formule commence un 1er.
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)

	if planID != nil {
		plan, err := s.q.GetMaintenancePlan(ctx, *planID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectMaintenanceView{}, domain.ErrValidation.WithDetails(map[string]any{"plan_id": "Formule inconnue"})
		}
		if err != nil {
			return ProjectMaintenanceView{}, fmt.Errorf("lecture de la formule : %w", err)
		}
		if !plan.Active {
			return ProjectMaintenanceView{}, domain.ErrValidation.WithDetails(map[string]any{"plan_id": "Cette formule est désactivée"})
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectMaintenanceView{}, fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.DeleteFutureProjectMaintenance(ctx, db.DeleteFutureProjectMaintenanceParams{ProjectID: projectID, Day: start}); err != nil {
		return ProjectMaintenanceView{}, fmt.Errorf("periodes a venir : %w", err)
	}
	if err := q.CloseProjectMaintenance(ctx, db.CloseProjectMaintenanceParams{ProjectID: projectID, Day: start}); err != nil {
		return ProjectMaintenanceView{}, fmt.Errorf("fin de la periode : %w", err)
	}
	if planID != nil {
		if err := q.OpenProjectMaintenance(ctx, db.OpenProjectMaintenanceParams{ProjectID: projectID, PlanID: *planID, StartsOn: start}); err != nil {
			return ProjectMaintenanceView{}, fmt.Errorf("nouvelle periode : %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectMaintenanceView{}, err
	}
	return s.ProjectMaintenance(ctx, projectID)
}

// --- Interventions --------------------------------------------------------------

// InterventionItem est une intervention de la liste de l'agence.
type InterventionItem struct {
	ID            uuid.UUID      `json:"id"`
	SpentOn       string         `json:"spent_on"`
	Minutes       int32          `json:"minutes"`
	Kind          string         `json:"kind"`
	Note          string         `json:"note"`
	ClientVisible bool           `json:"client_visible"`
	ClientNote    string         `json:"client_note"`
	Project       DeliverableRef `json:"project"`
	Person        *Person        `json:"person"`
}

// MaintenanceSummary est le mois d'un projet : son temps inclus, son
// consomme, son depassement.
type MaintenanceSummary struct {
	Project DeliverableRef `json:"project"`
	// Nuls pour un projet sans formule ce mois-la.
	PlanName        *string `json:"plan_name"`
	IncludedMinutes *int32  `json:"included_minutes"`
	ConsumedMinutes int32   `json:"consumed_minutes"`
	// La part visible du client ; le reste est interne.
	VisibleMinutes int32 `json:"visible_minutes"`
	// Au-dela du temps inclus, zero sinon. Sans montant : la facturation
	// reste sur l'ancienne plateforme.
	OverrunMinutes int32 `json:"overrun_minutes"`
}

// InterventionPage est la page « Interventions » d'un mois.
type InterventionPage struct {
	Month     string               `json:"month"`
	Items     []InterventionItem   `json:"items"`
	Total     int                  `json:"total"`
	Page      int                  `json:"page"`
	Summaries []MaintenanceSummary `json:"summaries"`
}

// monthBounds rend le 1er et le dernier jour du mois.
func monthBounds(month time.Time) (time.Time, time.Time) {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first, first.AddDate(0, 1, -1)
}

// CurrentMonth rend le mois en cours, dans le fuseau de l'agence.
func CurrentMonth() time.Time {
	now := time.Now().In(zone.Location())
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func maintenanceSummaryOf(r db.ListMaintenanceMonthRow) MaintenanceSummary {
	out := MaintenanceSummary{
		Project: DeliverableRef{ID: r.ID, Name: r.Name}, PlanName: r.PlanName, IncludedMinutes: r.IncludedMinutes,
		ConsumedMinutes: r.ConsumedMinutes, VisibleMinutes: r.VisibleMinutes,
	}
	if r.IncludedMinutes != nil && r.ConsumedMinutes > *r.IncludedMinutes {
		out.OverrunMinutes = r.ConsumedMinutes - *r.IncludedMinutes
	}
	return out
}

// Interventions rend les interventions d'un mois, de toute l'equipe, et le
// bilan de chaque projet.
func (s *MaintenanceService) Interventions(ctx context.Context, projectID *uuid.UUID, month time.Time, page, pageSize int) (InterventionPage, error) {
	page, pageSize = clampPage(page, pageSize)
	first, last := monthBounds(month)

	total, err := s.q.CountInterventions(ctx, db.CountInterventionsParams{FromDay: first, ToDay: last, ProjectID: projectID})
	if err != nil {
		return InterventionPage{}, fmt.Errorf("compte des interventions : %w", err)
	}
	rows, err := s.q.ListInterventions(ctx, db.ListInterventionsParams{
		FromDay: first, ToDay: last, ProjectID: projectID, Lim: int32(pageSize), Off: int32((page - 1) * pageSize),
	})
	if err != nil {
		return InterventionPage{}, fmt.Errorf("liste des interventions : %w", err)
	}
	summaries, err := s.q.ListMaintenanceMonth(ctx, db.ListMaintenanceMonthParams{Month: first, MonthEnd: last, ProjectID: projectID})
	if err != nil {
		return InterventionPage{}, fmt.Errorf("bilan du mois : %w", err)
	}

	out := InterventionPage{
		Month: first.Format("2006-01"), Items: make([]InterventionItem, 0, len(rows)), Total: int(total), Page: page,
		Summaries: make([]MaintenanceSummary, 0, len(summaries)),
	}
	for _, r := range rows {
		kind := ""
		if r.InterventionKind != nil {
			kind = *r.InterventionKind
		}
		out.Items = append(out.Items, InterventionItem{
			ID: r.ID, SpentOn: r.SpentOn.Format(dateLayout), Minutes: r.Minutes, Kind: kind, Note: r.Note,
			ClientVisible: r.ClientVisible, ClientNote: r.ClientNote,
			Project: DeliverableRef{ID: r.ProjectID, Name: r.ProjectName},
			Person:  personOf(r.UserID, r.Firstname, r.Lastname, r.AvatarUrl),
		})
	}
	for _, r := range summaries {
		out.Summaries = append(out.Summaries, maintenanceSummaryOf(r))
	}
	return out, nil
}

// --- Portail ----------------------------------------------------------------------

// PortalIntervention est une intervention vue par le client.
type PortalIntervention struct {
	ID      uuid.UUID `json:"id"`
	SpentOn string    `json:"spent_on"`
	Minutes int32     `json:"minutes"`
	Kind    string    `json:"kind"`
	Note    string    `json:"note"`
}

// PortalInterventionPage est l'historique d'un projet du client, avec le
// bilan du mois en cours quand le projet est sous formule.
type PortalInterventionPage struct {
	Items []PortalIntervention `json:"items"`
	Total int                  `json:"total"`
	Page  int                  `json:"page"`
	// Nul sans formule ce mois-ci.
	CurrentMonth *MaintenanceSummary `json:"current_month"`
}

// PortalInterventions rend les interventions visibles d'un projet du client.
// Un projet d'un autre client est introuvable.
func (s *MaintenanceService) PortalInterventions(ctx context.Context, userID, projectID uuid.UUID, page, pageSize int) (PortalInterventionPage, error) {
	clientID, err := s.q.PortalClientOf(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && clientID == nil) {
		return PortalInterventionPage{}, domain.ErrNotFound
	}
	if err != nil {
		return PortalInterventionPage{}, fmt.Errorf("lecture du compte : %w", err)
	}
	mine, err := s.q.PortalProjectOfClient(ctx, db.PortalProjectOfClientParams{ProjectID: projectID, ClientID: *clientID})
	if err != nil {
		return PortalInterventionPage{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if !mine {
		return PortalInterventionPage{}, domain.ErrNotFound
	}

	page, pageSize = clampPage(page, pageSize)
	total, err := s.q.CountPortalInterventions(ctx, projectID)
	if err != nil {
		return PortalInterventionPage{}, fmt.Errorf("compte des interventions : %w", err)
	}
	rows, err := s.q.ListPortalInterventions(ctx, db.ListPortalInterventionsParams{ProjectID: projectID, Lim: int32(pageSize), Off: int32((page - 1) * pageSize)})
	if err != nil {
		return PortalInterventionPage{}, fmt.Errorf("liste des interventions : %w", err)
	}
	out := PortalInterventionPage{Items: make([]PortalIntervention, 0, len(rows)), Total: int(total), Page: page}
	for _, r := range rows {
		kind := ""
		if r.InterventionKind != nil {
			kind = *r.InterventionKind
		}
		out.Items = append(out.Items, PortalIntervention{ID: r.ID, SpentOn: r.SpentOn.Format(dateLayout), Minutes: r.Minutes, Kind: kind, Note: r.ClientNote})
	}

	first, last := monthBounds(CurrentMonth())
	summaries, err := s.q.ListMaintenanceMonth(ctx, db.ListMaintenanceMonthParams{Month: first, MonthEnd: last, ProjectID: &projectID})
	if err != nil {
		return PortalInterventionPage{}, fmt.Errorf("bilan du mois : %w", err)
	}
	for _, r := range summaries {
		if r.IncludedMinutes != nil {
			summary := maintenanceSummaryOf(r)
			out.CurrentMonth = &summary
		}
	}
	return out, nil
}
