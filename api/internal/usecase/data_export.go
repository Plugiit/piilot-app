package usecase

import (
	"context"
	"fmt"
	"io"

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
	default:
		return domain.ErrValidation.WithDetails(map[string]any{"entity": "Cet export n'est pas encore disponible"})
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
			"siret":          c.Siret,
			"vat_number":     c.VatNumber,
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
