package usecase

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/csvio"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// exportPageSize : lignes lues par requete. La pagination se fait par cle
// (id > dernier id), pas par OFFSET : elle reste stable si une ligne est
// creee pendant l'export.
const exportPageSize = 1000

// DataExportService ecrit les exports CSV.
type DataExportService struct {
	q *db.Queries
}

// NewDataExportService construit le service.
func NewDataExportService(q *db.Queries) *DataExportService {
	return &DataExportService{q: q}
}

// Write ecrit le CSV d'une entite : les en-tetes de DataSpec, puis une
// ligne par enregistrement vivant. Les textes libres passent par csvio.Cell.
func (s *DataExportService) Write(ctx context.Context, entity string, out io.Writer) error {
	fields, ok := DataSpec(entity)
	if !ok {
		return domain.ErrValidation.WithDetails(map[string]any{"entity": "Entité inconnue"})
	}

	var page func(ctx context.Context, after uuid.UUID) (rows []map[string]string, last uuid.UUID, err error)

	switch entity {
	case DataEntityClients:
		page = s.clientsPage
	case DataEntityContacts:
		page = s.contactsPage
	case DataEntityProjects:
		page = s.projectsPage
	case DataEntityTimeEntries:
		page = s.timeEntriesPage
	default:
		return domain.ErrValidation.WithDetails(map[string]any{"entity": "Entité inconnue"})
	}

	w := csvio.NewWriter(out)

	headers := make([]string, len(fields))
	for i, f := range fields {
		headers[i] = f.Header
	}

	if err := w.Write(headers); err != nil {
		return err
	}

	after := uuid.Nil

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		rows, last, err := page(ctx, after)
		if err != nil {
			return err
		}

		for _, row := range rows {
			record := make([]string, len(fields))
			for i, f := range fields {
				record[i] = row[f.Key]
			}

			if err := w.Write(record); err != nil {
				return err
			}
		}

		if len(rows) < exportPageSize {
			return w.Flush()
		}

		after = last
	}
}

func (s *DataExportService) clientsPage(ctx context.Context, after uuid.UUID) ([]map[string]string, uuid.UUID, error) {
	rows, err := s.q.ExportClientsAfter(ctx, db.ExportClientsAfterParams{After: after, PageSize: exportPageSize})
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("lecture des clients : %w", err)
	}

	out := make([]map[string]string, 0, len(rows))
	last := after

	for _, c := range rows {
		last = c.ID

		out = append(out, map[string]string{
			FieldKeyID:       c.ID.String(),
			FieldKeySourceID: csvio.Cell(c.SourceKey),
			"name":           csvio.Cell(c.Name),
			"kind":           c.Kind,
			"legal_name":     csvio.Cell(c.LegalName),
			"legal_form":     csvio.Cell(c.LegalForm),
			"siret":          csvio.Cell(c.Siret),
			"vat_number":     csvio.Cell(c.VatNumber),
			"status":         c.Status,
			"website":        csvio.Cell(c.Website),
			"phone":          csvio.Cell(c.Phone),
			"address":        csvio.Cell(c.Address),
			"postal_code":    csvio.Cell(c.PostalCode),
			"city":           csvio.Cell(c.City),
			"country":        csvio.Cell(c.Country),
		})
	}

	return out, last, nil
}

func (s *DataExportService) contactsPage(ctx context.Context, after uuid.UUID) ([]map[string]string, uuid.UUID, error) {
	rows, err := s.q.ExportContactsAfter(ctx, db.ExportContactsAfterParams{After: after, PageSize: exportPageSize})
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("lecture des contacts : %w", err)
	}

	out := make([]map[string]string, 0, len(rows))
	last := after

	for _, c := range rows {
		last = c.ID

		email := ""
		if c.Email != nil {
			email = *c.Email
		}

		out = append(out, map[string]string{
			FieldKeyID:       c.ID.String(),
			FieldKeySourceID: csvio.Cell(c.SourceKey),
			"firstname":      csvio.Cell(c.Firstname),
			"lastname":       csvio.Cell(c.Lastname),
			"role":           csvio.Cell(c.Role),
			"email":          csvio.Cell(email),
			"phone":          csvio.Cell(c.Phone),
			"client":         csvio.Cell(c.ClientName),
			"client_siret":   csvio.Cell(c.ClientSiret),
		})
	}

	return out, last, nil
}

func (s *DataExportService) projectsPage(ctx context.Context, after uuid.UUID) ([]map[string]string, uuid.UUID, error) {
	rows, err := s.q.ExportProjectsAfter(ctx, db.ExportProjectsAfterParams{After: after, PageSize: exportPageSize})
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("lecture des projets : %w", err)
	}

	out := make([]map[string]string, 0, len(rows))
	last := after

	for _, p := range rows {
		last = p.ID

		out = append(out, map[string]string{
			FieldKeyID:       p.ID.String(),
			FieldKeySourceID: csvio.Cell(p.SourceKey),
			"name":           csvio.Cell(p.Name),
			"client":         csvio.Cell(p.ClientName),
			"client_siret":   csvio.Cell(p.ClientSiret),
			"status":         p.Status,
			"priority":       p.Priority,
			"description":    csvio.Cell(p.Description),
			"starts_on":      isoDay(p.StartsOn),
			"due_on":         isoDay(p.DueOn),
			"hours_sold":     csvio.FormatDecimal(p.HoursSold, 2),
		})
	}

	return out, last, nil
}

func (s *DataExportService) timeEntriesPage(ctx context.Context, after uuid.UUID) ([]map[string]string, uuid.UUID, error) {
	rows, err := s.q.ExportTimeEntriesAfter(ctx, db.ExportTimeEntriesAfterParams{After: after, PageSize: exportPageSize})
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("lecture des saisies de temps : %w", err)
	}

	out := make([]map[string]string, 0, len(rows))
	last := after

	for _, e := range rows {
		last = e.ID

		out = append(out, map[string]string{
			FieldKeyID:       e.ID.String(),
			FieldKeySourceID: csvio.Cell(e.SourceKey),
			"email":          csvio.Cell(e.UserEmail),
			"client":         csvio.Cell(e.ClientName),
			"project":        csvio.Cell(e.ProjectName),
			"service":        csvio.Cell(e.ServiceName),
			"date":           e.SpentOn.Format("2006-01-02"),
			"minutes":        strconv.Itoa(int(e.Minutes)),
			"note":           csvio.Cell(e.Note),
		})
	}

	return out, last, nil
}

// isoDay ecrit une date optionnelle en AAAA-MM-JJ.
func isoDay(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format("2006-01-02")
}
