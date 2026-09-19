package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// maxReportDays borne la periode d'un rapport. Au-dela d'un an, la somme au
// rendu cesserait d'etre bornee ; un bilan pluriannuel n'est pas l'usage.
const maxReportDays = 366

// ExportLimit borne l'export CSV. Largement au-dessus d'une annee de saisies
// pour une agence, et une requete ne part jamais sans LIMIT.
const ExportLimit = 50_000

// Axes de regroupement acceptes.
var reportGroups = map[string]struct{}{
	"project": {}, "user": {}, "service": {}, "client": {},
}

// TimeReportFilters est ce que l'ecran des rapports peut restreindre.
type TimeReportFilters struct {
	From      time.Time
	To        time.Time
	ProjectID *uuid.UUID
	UserID    *uuid.UUID
	ServiceID *uuid.UUID
	ClientID  *uuid.UUID
	// Nul : tout le temps. Vrai : seulement les projets clients. Faux :
	// seulement les projets internes.
	Billable *bool
}

// validate controle la periode, seule entree qui puisse faire deraper la
// requete.
func (f TimeReportFilters) validate() error {
	if f.To.Before(f.From) {
		return domain.ErrValidation.WithDetails(map[string]any{
			"to": "La fin de période précède son début",
		})
	}

	if f.To.Sub(f.From) > (maxReportDays-1)*24*time.Hour {
		return domain.ErrValidation.WithDetails(map[string]any{
			"to": "Une période ne dépasse pas un an",
		})
	}

	return nil
}

// ReportTotals sont les chiffres d'en-tete de la periode filtree.
type ReportTotals struct {
	Minutes            int64 `json:"minutes"`
	BillableMinutes    int64 `json:"billable_minutes"`
	NonBillableMinutes int64 `json:"non_billable_minutes"`
	Entries            int64 `json:"entries"`
	People             int64 `json:"people"`
	Projects           int64 `json:"projects"`
}

// ReportBucket est une tranche du graphique d'evolution.
type ReportBucket struct {
	// Premier jour de la tranche, AAAA-MM-JJ.
	Start              string `json:"start"`
	BillableMinutes    int64  `json:"billable_minutes"`
	NonBillableMinutes int64  `json:"non_billable_minutes"`
}

// ReportGroup est une ligne du tableau regroupe.
type ReportGroup struct {
	// Identifiant du projet, de la personne, du service ou du client. Vide
	// pour le temps sans service.
	Key   string `json:"key"`
	Label string `json:"label"`
	// Complement du libelle : le client d'un projet.
	Detail             string `json:"detail"`
	Color              string `json:"color"`
	Minutes            int64  `json:"minutes"`
	BillableMinutes    int64  `json:"billable_minutes"`
	NonBillableMinutes int64  `json:"non_billable_minutes"`
}

// ReportGroupPage est la page courante du tableau regroupe.
type ReportGroupPage struct {
	By       string        `json:"by"`
	Items    []ReportGroup `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// TimeReport est ce que l'ecran des rapports affiche en haut : totaux,
// evolution et tableau regroupe. Le detail des saisies a son propre endpoint,
// pagine a part.
type TimeReport struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Taille des tranches du graphique : day, week ou month, choisie selon la
	// longueur de la periode.
	Bucket string          `json:"bucket"`
	Totals ReportTotals    `json:"totals"`
	Series []ReportBucket  `json:"series"`
	Groups ReportGroupPage `json:"groups"`
}

// ReportEntry est une saisie dans le detail d'un rapport.
type ReportEntry struct {
	ID           uuid.UUID `json:"id"`
	SpentOn      string    `json:"spent_on"`
	Minutes      int32     `json:"minutes"`
	Note         string    `json:"note"`
	UserID       uuid.UUID `json:"user_id"`
	UserName     string    `json:"user_name"`
	ProjectID    uuid.UUID `json:"project_id"`
	ProjectName  string    `json:"project_name"`
	ClientName   string    `json:"client_name"`
	Billable     bool      `json:"billable"`
	TaskTitle    *string   `json:"task_title"`
	ServiceName  *string   `json:"service_name"`
	ServiceColor *string   `json:"service_color"`
}

// ReportEntryPage est une page du detail.
type ReportEntryPage struct {
	Items    []ReportEntry `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// TimeReportService sert l'ecran « Rapports de temps ».
type TimeReportService struct {
	q *db.Queries
}

func NewTimeReportService(pool *pgxpool.Pool) *TimeReportService {
	return &TimeReportService{q: db.New(pool)}
}

// bucketFor choisit la tranche du graphique : une barre par jour sur un mois,
// par semaine sur un trimestre, par mois au-dela. Assez de barres pour lire
// une tendance, pas au point de ne plus pouvoir les distinguer.
func bucketFor(from, to time.Time) string {
	days := int(to.Sub(from).Hours()/24) + 1

	switch {
	case days <= 45:
		return "day"
	case days <= 120:
		return "week"
	default:
		return "month"
	}
}

// bucketStart ramene un jour au debut de sa tranche, comme date_trunc : la
// semaine commence le lundi.
func bucketStart(day time.Time, bucket string) time.Time {
	switch bucket {
	case "week":
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	case "month":
		return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return day
	}
}

func nextBucket(day time.Time, bucket string) time.Time {
	switch bucket {
	case "week":
		return day.AddDate(0, 0, 7)
	case "month":
		return day.AddDate(0, 1, 0)
	default:
		return day.AddDate(0, 0, 1)
	}
}

// Report renvoie totaux, evolution et une page du tableau regroupe.
func (s *TimeReportService) Report(
	ctx context.Context,
	f TimeReportFilters,
	groupBy string,
	page, pageSize int,
) (TimeReport, error) {
	if err := f.validate(); err != nil {
		return TimeReport{}, err
	}

	if _, ok := reportGroups[groupBy]; !ok {
		return TimeReport{}, domain.ErrValidation.WithDetails(map[string]any{
			"group_by": "Regroupement inconnu",
		})
	}

	page, pageSize = clampPage(page, pageSize)

	totals, err := s.q.TimeReportTotals(ctx, db.TimeReportTotalsParams{
		FromDay: f.From, ToDay: f.To,
		ProjectID: f.ProjectID, UserID: f.UserID, ServiceID: f.ServiceID,
		ClientID: f.ClientID, Billable: f.Billable,
	})
	if err != nil {
		return TimeReport{}, fmt.Errorf("totaux du rapport : %w", err)
	}

	bucket := bucketFor(f.From, f.To)

	series, err := s.q.TimeReportSeries(ctx, db.TimeReportSeriesParams{
		Bucket: bucket, FromDay: f.From, ToDay: f.To,
		ProjectID: f.ProjectID, UserID: f.UserID, ServiceID: f.ServiceID,
		ClientID: f.ClientID, Billable: f.Billable,
	})
	if err != nil {
		return TimeReport{}, fmt.Errorf("evolution du rapport : %w", err)
	}

	groups, err := s.q.TimeReportGroups(ctx, db.TimeReportGroupsParams{
		GroupBy: groupBy, FromDay: f.From, ToDay: f.To,
		ProjectID: f.ProjectID, UserID: f.UserID, ServiceID: f.ServiceID,
		ClientID: f.ClientID, Billable: f.Billable,
		PageOffset: int32((page - 1) * pageSize), PageSize: int32(pageSize),
	})
	if err != nil {
		return TimeReport{}, fmt.Errorf("regroupement du rapport : %w", err)
	}

	report := TimeReport{
		From:   f.From.Format(dateLayout),
		To:     f.To.Format(dateLayout),
		Bucket: bucket,
		Totals: ReportTotals{
			Minutes:            totals.Minutes,
			BillableMinutes:    totals.BillableMinutes,
			NonBillableMinutes: totals.NonBillableMinutes,
			Entries:            totals.Entries,
			People:             totals.People,
			Projects:           totals.Projects,
		},
		Groups: ReportGroupPage{
			By:       groupBy,
			Items:    make([]ReportGroup, 0, len(groups)),
			Page:     page,
			PageSize: pageSize,
		},
	}

	// Chaque tranche de la periode figure dans la serie, vide ou non : le
	// graphique montre un trou la ou personne n'a pointe, au lieu de
	// resserrer l'axe comme si ces jours n'existaient pas.
	byStart := make(map[string]db.TimeReportSeriesRow, len(series))
	for _, row := range series {
		byStart[row.Bucket.Format(dateLayout)] = row
	}

	for day := bucketStart(f.From, bucket); !day.After(f.To); day = nextBucket(day, bucket) {
		key := day.Format(dateLayout)
		row := byStart[key]

		// La premiere tranche peut commencer avant la periode (le lundi d'une
		// semaine entamee) : elle s'affiche a partir du premier jour demande,
		// seul temps qu'elle contienne.
		start := key
		if day.Before(f.From) {
			start = f.From.Format(dateLayout)
		}

		report.Series = append(report.Series, ReportBucket{
			Start:              start,
			BillableMinutes:    row.BillableMinutes,
			NonBillableMinutes: row.NonBillableMinutes,
		})
	}

	for _, row := range groups {
		report.Groups.Total = row.TotalGroups
		report.Groups.Items = append(report.Groups.Items, ReportGroup{
			Key:                row.Key,
			Label:              row.Label,
			Detail:             row.Detail,
			Color:              row.Color,
			Minutes:            row.Minutes,
			BillableMinutes:    row.BillableMinutes,
			NonBillableMinutes: row.NonBillableMinutes,
		})
	}

	// Une page au-dela de la derniere ne rend aucune ligne, donc aucun total :
	// on le relit sur la premiere plutot que d'annoncer zero groupe.
	if len(groups) == 0 && page > 1 {
		first, err := s.q.TimeReportGroups(ctx, db.TimeReportGroupsParams{
			GroupBy: groupBy, FromDay: f.From, ToDay: f.To,
			ProjectID: f.ProjectID, UserID: f.UserID, ServiceID: f.ServiceID,
			ClientID: f.ClientID, Billable: f.Billable,
			PageOffset: 0, PageSize: 1,
		})
		if err == nil && len(first) > 0 {
			report.Groups.Total = first[0].TotalGroups
		}
	}

	return report, nil
}

// Entries renvoie une page du detail des saisies.
func (s *TimeReportService) Entries(
	ctx context.Context,
	f TimeReportFilters,
	page, pageSize int,
) (ReportEntryPage, error) {
	if err := f.validate(); err != nil {
		return ReportEntryPage{}, err
	}

	page, pageSize = clampPage(page, pageSize)

	totals, err := s.q.TimeReportTotals(ctx, db.TimeReportTotalsParams{
		FromDay: f.From, ToDay: f.To,
		ProjectID: f.ProjectID, UserID: f.UserID, ServiceID: f.ServiceID,
		ClientID: f.ClientID, Billable: f.Billable,
	})
	if err != nil {
		return ReportEntryPage{}, fmt.Errorf("total du detail : %w", err)
	}

	items, err := s.entries(ctx, f, (page-1)*pageSize, pageSize)
	if err != nil {
		return ReportEntryPage{}, err
	}

	return ReportEntryPage{Items: items, Total: totals.Entries, Page: page, PageSize: pageSize}, nil
}

// Export renvoie toutes les saisies filtrees, dans la limite de l'export.
func (s *TimeReportService) Export(ctx context.Context, f TimeReportFilters) ([]ReportEntry, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}

	return s.entries(ctx, f, 0, ExportLimit)
}

func (s *TimeReportService) entries(
	ctx context.Context,
	f TimeReportFilters,
	offset, limit int,
) ([]ReportEntry, error) {
	rows, err := s.q.TimeReportEntries(ctx, db.TimeReportEntriesParams{
		FromDay: f.From, ToDay: f.To,
		ProjectID: f.ProjectID, UserID: f.UserID, ServiceID: f.ServiceID,
		ClientID: f.ClientID, Billable: f.Billable,
		PageOffset: int32(offset), PageSize: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("detail des saisies : %w", err)
	}

	items := make([]ReportEntry, 0, len(rows))
	for _, row := range rows {
		items = append(items, ReportEntry{
			ID:           row.ID,
			SpentOn:      row.SpentOn.Format(dateLayout),
			Minutes:      row.Minutes,
			Note:         row.Note,
			UserID:       row.UserID,
			UserName:     row.UserName,
			ProjectID:    row.ProjectID,
			ProjectName:  row.ProjectName,
			ClientName:   row.ClientName,
			Billable:     row.Billable != nil && *row.Billable,
			TaskTitle:    row.TaskTitle,
			ServiceName:  row.ServiceName,
			ServiceColor: row.ServiceColor,
		})
	}

	return items, nil
}

// clampPage ramene la pagination dans ses bornes, comme les autres listes.
func clampPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	return page, pageSize
}
