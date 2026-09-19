package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// TimeReportService est le contrat de l'ecran « Rapports de temps ».
type TimeReportService interface {
	Report(ctx context.Context, f usecase.TimeReportFilters, groupBy string, page, pageSize int) (usecase.TimeReport, error)
	Entries(ctx context.Context, f usecase.TimeReportFilters, page, pageSize int) (usecase.ReportEntryPage, error)
	Export(ctx context.Context, f usecase.TimeReportFilters) ([]usecase.ReportEntry, error)
}

// TimeReports porte les endpoints des rapports de temps.
type TimeReports struct {
	svc TimeReportService
}

func NewTimeReports(svc TimeReportService) *TimeReports {
	return &TimeReports{svc: svc}
}

// filters lit les filtres communs aux trois endpoints.
//
// Sans periode, le mois en cours : c'est le rapport qu'on ouvre le plus
// souvent, pour boucler le mois.
func reportFilters(c fiber.Ctx) (usecase.TimeReportFilters, error) {
	now := time.Now()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	var f usecase.TimeReportFilters
	var err error

	if f.From, err = parseDay(c.Query("from", firstOfMonth.Format("2006-01-02")), "from"); err != nil {
		return f, err
	}
	if f.To, err = parseDay(c.Query("to", lastOfMonth.Format("2006-01-02")), "to"); err != nil {
		return f, err
	}

	for _, field := range []struct {
		name string
		dest **uuid.UUID
	}{
		{"project_id", &f.ProjectID},
		{"user_id", &f.UserID},
		{"service_id", &f.ServiceID},
		{"client_id", &f.ClientID},
	} {
		raw := strings.TrimSpace(c.Query(field.name))
		if raw == "" {
			continue
		}

		id, err := uuid.Parse(raw)
		if err != nil {
			return f, domain.ErrValidation.WithDetails(map[string]any{field.name: "Identifiant invalide"})
		}
		*field.dest = &id
	}

	switch c.Query("billable") {
	case "":
	case "true":
		value := true
		f.Billable = &value
	case "false":
		value := false
		f.Billable = &value
	default:
		return f, domain.ErrValidation.WithDetails(map[string]any{"billable": "true ou false attendu"})
	}

	return f, nil
}

// Report sert l'en-tete de l'ecran : totaux, evolution, tableau regroupe.
func (h *TimeReports) Report(c fiber.Ctx) error {
	f, err := reportFilters(c)
	if err != nil {
		return err
	}

	report, err := h.svc.Report(
		c.Context(), f, c.Query("group_by", "project"),
		queryInt(c, "page", 1), queryInt(c, "page_size", 10),
	)
	if err != nil {
		return err
	}

	return c.JSON(report)
}

// Entries sert le detail pagine des saisies.
func (h *TimeReports) Entries(c fiber.Ctx) error {
	f, err := reportFilters(c)
	if err != nil {
		return err
	}

	page, err := h.svc.Entries(c.Context(), f, queryInt(c, "page", 1), queryInt(c, "page_size", 25))
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// Export rend les saisies filtrees en CSV.
//
// Point-virgule et BOM UTF-8 : c'est ce qu'Excel en francais ouvre sans
// assistant d'import, accents compris. Les durees en heures decimales a la
// virgule, pour qu'une somme se fasse directement dans le tableur.
func (h *TimeReports) Export(c fiber.Ctx) error {
	f, err := reportFilters(c)
	if err != nil {
		return err
	}

	entries, err := h.svc.Export(c.Context(), f)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(&buf)
	w.Comma = ';'
	w.UseCRLF = true

	_ = w.Write([]string{
		"Date", "Personne", "Client", "Projet", "Tâche", "Service",
		"Facturable", "Durée (h)", "Minutes", "Note",
	})

	for _, e := range entries {
		billable := "Non"
		if e.Billable {
			billable = "Oui"
		}

		_ = w.Write([]string{
			e.SpentOn,
			csvCell(e.UserName),
			csvCell(e.ClientName),
			csvCell(e.ProjectName),
			csvCell(deref(e.TaskTitle)),
			csvCell(deref(e.ServiceName)),
			billable,
			strings.Replace(strconv.FormatFloat(float64(e.Minutes)/60, 'f', 2, 64), ".", ",", 1),
			strconv.Itoa(int(e.Minutes)),
			csvCell(e.Note),
		})
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("ecriture du csv : %w", err)
	}

	name := fmt.Sprintf("piilot-temps_%s_%s.csv", f.From.Format("2006-01-02"), f.To.Format("2006-01-02"))

	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	c.Set(fiber.HeaderCacheControl, "no-store")

	return c.Send(buf.Bytes())
}

// csvCell neutralise une cellule qu'un tableur prendrait pour une formule.
//
// Une note commencant par « = » s'executerait a l'ouverture du fichier : une
// apostrophe en tete la fait lire comme du texte. Les notes et les noms sont
// saisis par des personnes, et le fichier est ouvert par d'autres.
func csvCell(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}

	return value
}

func deref(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
