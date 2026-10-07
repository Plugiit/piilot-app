package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

type auditRecorder struct{ entries []usecase.AuditEntry }

func (r *auditRecorder) Record(_ context.Context, e usecase.AuditEntry) error {
	r.entries = append(r.entries, e)
	return nil
}
func (r *auditRecorder) List(context.Context, usecase.AuditFilters, int) (usecase.AuditPage, error) {
	return usecase.AuditPage{}, nil
}
func (r *auditRecorder) Export(context.Context, usecase.AuditFilters) ([]usecase.AuditItem, int, error) {
	return nil, 0, nil
}

func TestLIntercepteurDAuditNommeLesGestes(t *testing.T) {
	rec := &auditRecorder{}
	audit := NewAudit(rec, nil)
	app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
		if e, ok := err.(*domain.Error); ok {
			return c.Status(e.Status).SendString(e.Message)
		}
		return c.SendStatus(500)
	}})
	v1 := app.Group("/api/v1")
	v1.Use(audit.Middleware)
	v1.Group("/admin").Delete("/projects/:id", func(c fiber.Ctx) error { return c.SendStatus(204) })
	v1.Group("/admin").Patch("/accounts/:id", func(c fiber.Ctx) error { return domain.ErrForbidden })
	v1.Group("/admin").Get("/projects", func(c fiber.Ctx) error { return c.SendStatus(200) })
	v1.Group("/auth").Post("/login", func(c fiber.Ctx) error { return domain.ErrUnauthorized })

	do := func(method, path, body string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if _, err := app.Test(req); err != nil {
			t.Fatal(err)
		}
	}
	do("DELETE", "/api/v1/admin/projects/42", "")
	do("PATCH", "/api/v1/admin/accounts/7", `{"role":"admin"}`)
	do("GET", "/api/v1/admin/projects", "")
	do("POST", "/api/v1/auth/login", `{"email":"Pirate@Example.fr","password":"secret"}`)

	if len(rec.entries) != 2 {
		t.Fatalf("lignes : %+v", rec.entries)
	}
	if e := rec.entries[0]; e.Action != "project.deleted" || e.TargetID != "42" || e.TargetType != "project" {
		t.Errorf("suppression : %+v", e)
	}
	if e := rec.entries[1]; e.Action != "auth.login_failed" || e.Details["email"] != "pirate@example.fr" {
		t.Errorf("echec de connexion : %+v", e)
	}
	for _, e := range rec.entries {
		for _, v := range e.Details {
			if v == "secret" {
				t.Fatal("un mot de passe est parti au journal")
			}
		}
	}
}
