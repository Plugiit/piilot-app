package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// DataImportService est le contrat dont l'ecran d'import a besoin.
type DataImportService interface {
	Upload(ctx context.Context, actor *uuid.UUID, entity, fileName string, content io.Reader) (usecase.ImportDetail, error)
	Get(ctx context.Context, id uuid.UUID) (usecase.ImportDetail, error)
	List(ctx context.Context, page int) (usecase.ImportPage, error)
	Rows(ctx context.Context, id uuid.UUID, outcome string, page int) (usecase.ImportRowPage, error)
	Run(ctx context.Context, id uuid.UUID, in usecase.RunImportInput) (usecase.ImportItem, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// DataExportService est le contrat de l'export CSV.
type DataExportService interface {
	Write(ctx context.Context, entity string, out io.Writer) error
}

// DataImport porte les endpoints d'import et d'export CSV.
type DataImport struct {
	imports DataImportService
	exports DataExportService
}

// NewDataImport construit le handler.
func NewDataImport(imports DataImportService, exports DataExportService) *DataImport {
	return &DataImport{imports: imports, exports: exports}
}

// entityParam lit et verifie une entite.
func entityParam(raw string) (string, error) {
	if _, ok := usecase.DataSpec(raw); !ok {
		return "", domain.ErrValidation.WithDetails(map[string]any{"entity": "Entité inconnue"})
	}

	return raw, nil
}

// List sert l'historique des imports.
func (h *DataImport) List(c fiber.Ctx) error {
	page, err := h.imports.List(c.Context(), queryInt(c, "page", 1))
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// Upload recoit le fichier (multipart : entity, file).
func (h *DataImport) Upload(c fiber.Ctx) error {
	entity, err := entityParam(c.FormValue("entity"))
	if err != nil {
		return err
	}

	header, err := c.FormFile("file")
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"file": "Aucun fichier reçu sous le champ « file »"})
	}

	content, err := header.Open()
	if err != nil {
		return fmt.Errorf("lecture du fichier envoye : %w", err)
	}
	defer func() { _ = content.Close() }()

	detail, err := h.imports.Upload(c.Context(), actorOf(c), entity, header.Filename, content)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(detail)
}

// Get sert un import.
func (h *DataImport) Get(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	detail, err := h.imports.Get(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(detail)
}

// runImportRequest est le corps de POST /admin/imports/:id/run.
type runImportRequest struct {
	Mapping map[string]string `json:"mapping"`
	DryRun  bool              `json:"dry_run"`
}

// Run met l'import en file : 202, le traitement suit en tache de fond.
func (h *DataImport) Run(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req runImportRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	item, err := h.imports.Run(c.Context(), id, usecase.RunImportInput{Mapping: req.Mapping, DryRun: req.DryRun, ActorID: actorOf(c)})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(item)
}

// Rows sert le rapport d'un import.
func (h *DataImport) Rows(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	page, err := h.imports.Rows(c.Context(), id, c.Query("outcome"), queryInt(c, "page", 1))
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// Delete retire un import de l'historique.
func (h *DataImport) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.imports.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Export rend le CSV d'une entite.
func (h *DataImport) Export(c fiber.Ctx) error {
	entity, err := entityParam(c.Params("entity"))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := h.exports.Write(c.Context(), entity, &buf); err != nil {
		return err
	}

	name := fmt.Sprintf("piilot-%s_%s.csv", entity, time.Now().UTC().Format("2006-01-02"))

	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	c.Set(fiber.HeaderCacheControl, "no-store")

	return c.Send(buf.Bytes())
}
