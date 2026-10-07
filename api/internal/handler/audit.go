package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// AuditService est le contrat du journal d'audit.
type AuditService interface {
	Record(ctx context.Context, e usecase.AuditEntry) error
	List(ctx context.Context, f usecase.AuditFilters, page int) (usecase.AuditPage, error)
	Export(ctx context.Context, f usecase.AuditFilters) ([]usecase.AuditItem, int, error)
}

// Audit ecrit le journal depuis un intercepteur, et le sert aux admins.
type Audit struct {
	svc AuditService
	log *slog.Logger
}

// NewAudit construit le handler.
func NewAudit(svc AuditService, log *slog.Logger) *Audit { return &Audit{svc: svc, log: log} }

// auditActorKey : le handler de connexion y pose le compte qui vient de se
// connecter, que l'intercepteur ne peut pas lire dans une session pas encore
// ouverte.
const auditActorKey = "audit.actor"

// auditRule decrit ce qu'une route ecrit au journal.
type auditRule struct {
	action string
	target string
	// Parametre de chemin qui designe la cible.
	param string
	// Champs du corps JSON recopies dans les details. Jamais un mot de passe.
	fields []string
	// Recopie la chaine de requete : les filtres d'un export.
	query bool
	// N'ecrit que si l'un de ces champs est present dans le corps.
	onlyIf []string
}

// auditRules : ce qui s'ecrit au journal, route par route. Une seule table,
// lisible d'un coup d'oeil, plutot qu'un appel disperse dans chaque handler.
var auditRules = map[string]auditRule{
	"POST /api/v1/auth/me/password":                      {action: "auth.password_changed"},
	"PATCH /api/v1/auth/me":                              {action: "account.email_changed", fields: []string{"email"}, onlyIf: []string{"email"}},
	"POST /api/v1/auth/password/reset/:token":            {action: "auth.password_reset"},
	"POST /api/v1/auth/invitations/:token/accept":        {action: "invitation.accepted"},
	"POST /api/v1/admin/accounts/invitations":            {action: "invitation.created", target: "invitation", fields: []string{"email", "role"}},
	"POST /api/v1/admin/accounts/invitations/:id/resend": {action: "invitation.resent", target: "invitation", param: "id"},
	"DELETE /api/v1/admin/accounts/invitations/:id":      {action: "invitation.revoked", target: "invitation", param: "id"},
	"PATCH /api/v1/admin/accounts/:id":                   {action: "account.role_changed", target: "user", param: "id", fields: []string{"role"}},
	"POST /api/v1/admin/accounts/:id/disable":            {action: "account.disabled", target: "user", param: "id"},
	"POST /api/v1/admin/accounts/:id/enable":             {action: "account.enabled", target: "user", param: "id"},
	"POST /api/v1/admin/accounts/:id/password-reset":     {action: "account.password_reset_link", target: "user", param: "id"},
	"PUT /api/v1/admin/roles/:code/permissions":          {action: "role.permissions_changed", target: "role", param: "code", fields: []string{"permissions"}},
	"POST /api/v1/admin/system/update":                   {action: "system.update_requested"},
	"DELETE /api/v1/admin/crm/clients/:id":               {action: "client.deleted", target: "client", param: "id"},
	"DELETE /api/v1/admin/crm/contacts/:id":              {action: "contact.deleted", target: "contact", param: "id"},
	"DELETE /api/v1/admin/crm/interactions/:id":          {action: "interaction.deleted", target: "interaction", param: "id"},
	"DELETE /api/v1/admin/projects/:id":                  {action: "project.deleted", target: "project", param: "id"},
	"DELETE /api/v1/admin/files/:id":                     {action: "file.deleted", target: "file", param: "id"},
	"DELETE /api/v1/admin/milestones/:id":                {action: "milestone.deleted", target: "milestone", param: "id"},
	"DELETE /api/v1/admin/project-templates/:id":         {action: "project_template.deleted", target: "project_template", param: "id"},
	"DELETE /api/v1/admin/tasks/:id":                     {action: "task.deleted", target: "task", param: "id"},
	"DELETE /api/v1/admin/services/:id":                  {action: "service.deleted", target: "service", param: "id"},
	"DELETE /api/v1/admin/time-entries/:id":              {action: "time_entry.deleted", target: "time_entry", param: "id"},
	"DELETE /api/v1/admin/ticket-reply-templates/:id":    {action: "reply_template.deleted", target: "reply_template", param: "id"},
	"POST /api/v1/admin/integrations/git/rotate":         {action: "secret.git_rotated"},
	"PUT /api/v1/admin/integrations/mail":                {action: "settings.inbound_mail_updated", fields: []string{"address", "imap_enabled", "imap_host", "imap_username"}},
	"POST /api/v1/admin/integrations/mail/rotate":        {action: "secret.inbound_webhook_rotated"},
	"POST /api/v1/admin/inbound-emails/:id/ticket":       {action: "inbound.ticket_created", target: "inbound_email", param: "id", fields: []string{"project_id"}},
	"POST /api/v1/admin/inbound-emails/:id/attach":       {action: "inbound.attached", target: "inbound_email", param: "id", fields: []string{"numero"}},
	"POST /api/v1/admin/inbound-emails/:id/ignore":       {action: "inbound.dismissed", target: "inbound_email", param: "id"},
	"GET /api/v1/admin/time-reports/export":              {action: "export.time_report", query: true},
	"GET /api/v1/admin/audit/export":                     {action: "export.audit", query: true},
	"GET /api/v1/client/files/:id":                       {action: "portal.file_downloaded", target: "file", param: "id"},
	"POST /api/v1/client/deliverables/:id/decision":      {action: "portal.deliverable_decided", target: "deliverable", param: "id", fields: []string{"decision"}},
	"POST /api/v1/public/deliverables/:id/review":        {action: "portal.deliverable_decided_by_link", target: "deliverable", param: "id", fields: []string{"decision"}},
}

// Middleware ecrit au journal ce que la table ci-dessus decrit, une fois la
// requete servie. Un journal qui ne s'ecrit pas ne fait pas echouer le geste :
// l'incident part dans les logs.
func (h *Audit) Middleware(c fiber.Ctx) error {
	err := c.Next()

	route := c.Route()
	if route == nil {
		return err
	}
	key := c.Method() + " " + route.Path

	status := c.Response().StatusCode()
	if err != nil {
		if e, ok := err.(*domain.Error); ok {
			status = e.Status
		} else {
			status = fiber.StatusInternalServerError
		}
	}

	if key == "POST /api/v1/auth/login" {
		h.login(c, status)
		return err
	}

	rule, ok := auditRules[key]
	if !ok || status >= 400 {
		return err
	}

	details := map[string]any{}
	if len(rule.fields) > 0 || len(rule.onlyIf) > 0 {
		var body map[string]any
		_ = json.Unmarshal(c.Body(), &body)
		if len(rule.onlyIf) > 0 {
			present := false
			for _, f := range rule.onlyIf {
				if _, ok := body[f]; ok {
					present = true
				}
			}
			if !present {
				return err
			}
		}
		for _, f := range rule.fields {
			if v, ok := body[f]; ok {
				details[f] = v
			}
		}
	}
	if rule.query {
		args := c.Queries()
		keys := make([]string, 0, len(args))
		for k := range args {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			details[strings.Clone(k)] = strings.Clone(args[k])
		}
	}

	h.record(c, usecase.AuditEntry{
		ActorID: actorOf(c), Action: rule.action, TargetType: rule.target, TargetID: paramOf(c, rule.param), Details: details,
	})
	return err
}

// login ecrit les connexions et leurs echecs, avec l'adresse tentee — jamais
// le mot de passe.
func (h *Audit) login(c fiber.Ctx, status int) {
	var body struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(c.Body(), &body)
	email := strings.ToLower(strings.TrimSpace(body.Email))

	switch {
	case status < 300:
		h.record(c, usecase.AuditEntry{ActorID: actorOf(c), Action: "auth.login"})
	case status == fiber.StatusTooManyRequests:
		h.record(c, usecase.AuditEntry{Action: "auth.login_rate_limited", Details: map[string]any{"email": email}})
	case status == fiber.StatusUnauthorized || status == fiber.StatusForbidden:
		h.record(c, usecase.AuditEntry{Action: "auth.login_failed", Details: map[string]any{"email": email}})
	}
}

// Les chaines de Fiber pointent dans des tampons reutilises par la requete
// suivante : tout ce qui est garde est copie.
func (h *Audit) record(c fiber.Ctx, e usecase.AuditEntry) {
	e.IP = strings.Clone(c.IP())
	e.UserAgent = strings.Clone(c.Get(fiber.HeaderUserAgent))
	e.TargetID = strings.Clone(e.TargetID)
	if err := h.svc.Record(c.Context(), e); err != nil && h.log != nil {
		h.log.Warn("journal d'audit non ecrit", "action", e.Action, "error", err)
	}
}

func actorOf(c fiber.Ctx) *uuid.UUID {
	if id, ok := c.Locals(auditActorKey).(uuid.UUID); ok {
		return &id
	}
	if id, ok := middleware.UserIDFrom(c); ok {
		return &id
	}
	return nil
}

func paramOf(c fiber.Ctx, name string) string {
	if name == "" {
		return ""
	}
	return c.Params(name)
}

// auditFilters lit les filtres de l'ecran.
func auditFilters(c fiber.Ctx) (usecase.AuditFilters, error) {
	f := usecase.AuditFilters{Action: c.Query("action"), Search: c.Query("search")}
	if v := c.Query("actor_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return f, domain.ErrValidation.WithDetails(map[string]any{"actor_id": "Identifiant invalide"})
		}
		f.ActorID = &id
	}
	for _, d := range []struct {
		key string
		dst **time.Time
		add int
	}{{"from", &f.Since, 0}, {"to", &f.Until, 1}} {
		v := c.Query(d.key)
		if v == "" {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02", v, paris)
		if err != nil {
			return f, domain.ErrValidation.WithDetails(map[string]any{d.key: "Date attendue : AAAA-MM-JJ"})
		}
		t = t.AddDate(0, 0, d.add)
		*d.dst = &t
	}
	return f, nil
}

// paris : les bornes de dates sont celles de l'agence.
var paris = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// List rend une page du journal.
func (h *Audit) List(c fiber.Ctx) error {
	f, err := auditFilters(c)
	if err != nil {
		return err
	}
	out, err := h.svc.List(c.Context(), f, queryInt(c, "page", 1))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Export rend le journal filtre en CSV, 50 000 lignes au plus.
func (h *Audit) Export(c fiber.Ctx) error {
	f, err := auditFilters(c)
	if err != nil {
		return err
	}
	items, total, err := h.svc.Export(c.Context(), f)
	if err != nil {
		return err
	}
	if total > usecase.AuditExportMax {
		return domain.ErrValidation.WithMessage(fmt.Sprintf("%d lignes : resserrez les dates (%d au plus par export)", total, usecase.AuditExportMax))
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	w.UseCRLF = true
	_ = w.Write([]string{"Date", "Compte", "Action", "Cible", "Identifiant", "Adresse IP", "Navigateur", "Détails"})
	for _, it := range items {
		details, _ := json.Marshal(it.Details)
		_ = w.Write([]string{
			it.At.In(paris).Format("2006-01-02 15:04:05"), csvCell(it.ActorEmail), it.Action, it.TargetType,
			csvCell(it.TargetID), it.IP, csvCell(it.UserAgent), csvCell(string(details)),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("ecriture du csv : %w", err)
	}

	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="piilot-audit_`+time.Now().In(paris).Format("2006-01-02")+`.csv"`)
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Send(buf.Bytes())
}
