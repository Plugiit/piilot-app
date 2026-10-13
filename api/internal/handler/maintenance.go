package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// MaintenanceService est ce que le handler attend de la maintenance.
type MaintenanceService interface {
	Plans(ctx context.Context) ([]usecase.MaintenancePlan, error)
	CreatePlan(ctx context.Context, in usecase.MaintenancePlanInput) (usecase.MaintenancePlan, error)
	UpdatePlan(ctx context.Context, id uuid.UUID, in usecase.MaintenancePlanInput) (usecase.MaintenancePlan, error)
	DeletePlan(ctx context.Context, id uuid.UUID) error
	ProjectMaintenance(ctx context.Context, projectID uuid.UUID) (usecase.ProjectMaintenanceView, error)
	SetProjectMaintenance(ctx context.Context, projectID uuid.UUID, planID *uuid.UUID, month time.Time) (usecase.ProjectMaintenanceView, error)
	Interventions(ctx context.Context, projectID *uuid.UUID, month time.Time, page, pageSize int) (usecase.InterventionPage, error)
	PortalInterventions(ctx context.Context, userID, projectID uuid.UUID, page, pageSize int) (usecase.PortalInterventionPage, error)
}

// Maintenance porte les formules et le journal des interventions.
type Maintenance struct {
	svc MaintenanceService
}

// NewMaintenance construit le handler.
func NewMaintenance(svc MaintenanceService) *Maintenance { return &Maintenance{svc: svc} }

// Plans liste les formules.
func (h *Maintenance) Plans(c fiber.Ctx) error {
	out, err := h.svc.Plans(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": out})
}

type planBody struct {
	Name            string   `json:"name"`
	IncludedMinutes int      `json:"included_minutes"`
	Sections        []string `json:"sections"`
	Active          bool     `json:"active"`
}

func (b planBody) input() usecase.MaintenancePlanInput {
	return usecase.MaintenancePlanInput{Name: b.Name, IncludedMinutes: b.IncludedMinutes, Sections: b.Sections, Active: b.Active}
}

// CreatePlan ajoute une formule.
func (h *Maintenance) CreatePlan(c fiber.Ctx) error {
	var b planBody
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.CreatePlan(c.Context(), b.input())
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

// UpdatePlan modifie une formule.
func (h *Maintenance) UpdatePlan(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	var b planBody
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.UpdatePlan(c.Context(), id, b.input())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// DeletePlan supprime une formule qui n'a jamais servi.
func (h *Maintenance) DeletePlan(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	if err := h.svc.DeletePlan(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Project rend la formule d'un projet.
func (h *Maintenance) Project(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	out, err := h.svc.ProjectMaintenance(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// SetProject change la formule d'un projet a partir d'un mois.
func (h *Maintenance) SetProject(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	var b struct {
		PlanID *string `json:"plan_id"`
		Month  string  `json:"month"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	month, err := time.Parse("2006-01", b.Month)
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"month": "Un mois au format AAAA-MM"})
	}
	var planID *uuid.UUID
	if b.PlanID != nil && *b.PlanID != "" {
		p, err := uuid.Parse(*b.PlanID)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"plan_id": "Identifiant invalide"})
		}
		planID = &p
	}
	out, err := h.svc.SetProjectMaintenance(c.Context(), id, planID, month)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Interventions rend les interventions d'un mois. ?month=AAAA-MM (le mois
// en cours par defaut), ?project=… pour un seul projet.
func (h *Maintenance) Interventions(c fiber.Ctx) error {
	month := usecase.CurrentMonth()
	if raw := c.Query("month"); raw != "" {
		m, err := time.Parse("2006-01", raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"month": "Un mois au format AAAA-MM"})
		}
		month = m
	}
	var projectID *uuid.UUID
	if raw := c.Query("project"); raw != "" {
		p, err := uuid.Parse(raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"project": "Identifiant invalide"})
		}
		projectID = &p
	}
	out, err := h.svc.Interventions(c.Context(), projectID, month, queryInt(c, "page", 1), queryInt(c, "page_size", 25))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// PortalInterventions rend l'historique d'un projet du client.
func (h *Maintenance) PortalInterventions(c fiber.Ctx) error {
	user, id, err := portalProject(c)
	if err != nil {
		return err
	}
	out, err := h.svc.PortalInterventions(c.Context(), user, id, queryInt(c, "page", 1), queryInt(c, "page_size", 20))
	if err != nil {
		return err
	}
	return c.JSON(out)
}
