package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/csvio"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Libelles acceptes a l'import, apres csvio.NormalizeHeader. Les valeurs sont
// celles des CHECK de la base (000037 pour le statut, 000005 pour la priorite).
var (
	projectStatusAliases = map[string]string{
		"cadrage": "cadrage", "en cadrage": "cadrage",
		"production": "production", "en production": "production", "en cours": "production",
		"attente": "attente", "en attente": "attente", "attente client": "attente",
		"livre": "livre", "livree": "livre", "termine": "livre",
		"hebergement": "hebergement", "heberge": "hebergement", "en hebergement": "hebergement",
	}
	projectPriorityAliases = map[string]string{
		"low": "low", "basse": "low", "faible": "low",
		"medium": "medium", "moyenne": "medium", "normale": "medium",
		"high": "high", "haute": "high", "elevee": "high",
	}
)

const maxHoursSold = 99999.99 // numeric(7, 2)

// projectImporter importe un projet. Le client est obligatoire et doit
// exister ; doublon : (client, nom en minuscules).
type projectImporter struct{}

func (projectImporter) importRow(ctx context.Context, q *db.Queries, importID uuid.UUID, v map[string]string) (importResult, error) {
	name := strings.TrimSpace(v["name"])
	label := name

	invalid := func(field, msg string) importResult {
		return importResult{Outcome: ImportRowError, Reason: ReasonInvalid, Label: label + " : " + field + " : " + msg}
	}

	if name == "" {
		return invalid("name", "le nom est requis"), nil
	}

	if len([]rune(name)) > 200 {
		return invalid("name", "200 caractères au plus"), nil
	}

	status, priority := "cadrage", "medium"

	if raw := strings.TrimSpace(v["status"]); raw != "" {
		s, ok := enumValue(raw, projectStatusAliases)
		if !ok {
			return invalid("status", "statut inconnu (cadrage, production, attente, livré ou hébergement)"), nil
		}

		status = s
	}

	if raw := strings.TrimSpace(v["priority"]); raw != "" {
		p, ok := enumValue(raw, projectPriorityAliases)
		if !ok {
			return invalid("priority", "priorité inconnue (basse, moyenne ou haute)"), nil
		}

		priority = p
	}

	startsOn, err := optionalDate(v["starts_on"])
	if err != nil {
		return invalid("starts_on", "date invalide (AAAA-MM-JJ ou JJ/MM/AAAA)"), nil
	}

	dueOn, err := optionalDate(v["due_on"])
	if err != nil {
		return invalid("due_on", "date invalide (AAAA-MM-JJ ou JJ/MM/AAAA)"), nil
	}

	if startsOn != nil && dueOn != nil && dueOn.Before(*startsOn) {
		return invalid("due_on", "l'échéance précède le début"), nil
	}

	var hoursSold float64

	if raw := strings.TrimSpace(v["hours_sold"]); raw != "" {
		if hoursSold, err = csvio.ParseDecimal(raw); err != nil || hoursSold < 0 || hoursSold > maxHoursSold {
			return invalid("hours_sold", "nombre d'heures invalide"), nil
		}
	}

	if normalizeSiret(v["client_siret"]) == "" && strings.TrimSpace(v["client"]) == "" {
		return importResult{Outcome: ImportRowError, Reason: ReasonUnknownClient, Label: label + " : client : le client est requis"}, nil
	}

	client, err := resolveClient(ctx, q, v["client_siret"], v["client"])
	if err != nil {
		return importResult{}, err
	}

	if client == nil {
		return importResult{Outcome: ImportRowError, Reason: ReasonUnknownClient, Label: label + " : client : client introuvable"}, nil
	}

	check := replayCheck{
		entity: DataEntityProjects, label: label,
		exists: func(ctx context.Context, q *db.Queries, id uuid.UUID) (bool, error) {
			ok, err := q.ProjectExists(ctx, id)
			if err != nil {
				return false, fmt.Errorf("recherche du projet : %w", err)
			}

			return ok, nil
		},
	}

	res, sourceKey, done, err := check.run(ctx, q, v, "project:"+client.ID.String()+"|"+strings.ToLower(name))
	if err != nil || done {
		return res, err
	}

	switch existing, err := q.GetProjectByClientAndName(ctx, db.GetProjectByClientAndNameParams{ClientID: client.ID, Name: name}); {
	case err == nil:
		return importResult{Outcome: ImportRowSkipped, Reason: ReasonDuplicateName, Label: label, EntityID: &existing.ID}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return importResult{}, fmt.Errorf("recherche du nom : %w", err)
	}

	project, err := q.CreateProject(ctx, db.CreateProjectParams{
		ClientID: client.ID, Name: name, Description: strings.TrimSpace(v["description"]),
		Status: status, Priority: priority, HoursSold: hoursSold, StartsOn: startsOn, DueOn: dueOn,
	})
	if err != nil {
		return importResult{Label: label}, fmt.Errorf("creation du projet : %w", err)
	}

	if err := q.UpsertImportKey(ctx, db.UpsertImportKeyParams{
		Entity: DataEntityProjects, SourceKey: sourceKey, EntityID: project.ID, ImportID: &importID,
	}); err != nil {
		return importResult{}, fmt.Errorf("ecriture de la cle d'import : %w", err)
	}

	return importResult{Outcome: ImportRowCreated, Label: label, EntityID: &project.ID}, nil
}

// optionalDate lit une date, ou rien si la cellule est vide.
func optionalDate(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	t, err := csvio.ParseDate(raw)
	if err != nil {
		return nil, err
	}

	return &t, nil
}
