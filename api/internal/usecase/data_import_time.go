package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/csvio"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Bornes d'une saisie, celles du CHECK time_entries_minutes_check.
const (
	minTimeMinutes = 1
	maxTimeMinutes = 24 * 60
)

// timeEntryImporter importe une saisie de temps.
//
// Le compte se retrouve par e-mail (admin ou equipe, vivant), le projet par
// client + nom, le service par nom. Une saisie n'a pas de cle metier : le
// rejeu repose sur l'identifiant d'origine, sinon sur l'empreinte de
// (compte, projet, jour, duree, service, note) -- deux saisies strictement
// identiques d'un meme fichier n'en font donc qu'une, sauf si le fichier porte
// un identifiant d'origine ou un identifiant Piilot.
//
// Le total d'heures du projet est recalcule par le declencheur de la base.
type timeEntryImporter struct{}

func (timeEntryImporter) importRow(ctx context.Context, q *db.Queries, importID uuid.UUID, v map[string]string) (importResult, error) {
	email := strings.TrimSpace(v["email"])
	projectName := strings.TrimSpace(v["project"])
	label := projectName

	if label == "" {
		label = email
	}

	invalid := func(field, msg string) importResult {
		return importResult{Outcome: ImportRowError, Reason: ReasonInvalid, Label: label + " : " + field + " : " + msg}
	}

	if email == "" {
		return importResult{Outcome: ImportRowError, Reason: ReasonUnknownUser, Label: label + " : email : l'e-mail du compte est requis"}, nil
	}

	if projectName == "" {
		return invalid("project", "le projet est requis"), nil
	}

	day, err := csvio.ParseDate(v["date"])
	if err != nil {
		return invalid("date", "date invalide (AAAA-MM-JJ ou JJ/MM/AAAA)"), nil
	}

	minutes, err := csvio.ParseDuration(v["minutes"])
	if err != nil || minutes < minTimeMinutes || minutes > maxTimeMinutes {
		return invalid("minutes", "durée invalide : entre 1 minute et 24 h"), nil
	}

	var userID uuid.UUID

	switch id, err := q.FindImportUserByEmail(ctx, email); {
	case err == nil:
		userID = id
	case errors.Is(err, pgx.ErrNoRows):
		return importResult{Outcome: ImportRowError, Reason: ReasonUnknownUser, Label: label + " : email : aucun compte de l'équipe pour cette adresse"}, nil
	default:
		return importResult{}, fmt.Errorf("recherche du compte : %w", err)
	}

	var clientName *string
	if c := strings.TrimSpace(v["client"]); c != "" {
		clientName = &c
	}

	projects, err := q.FindProjectsForTimeImport(ctx, db.FindProjectsForTimeImportParams{Name: projectName, ClientName: clientName})
	if err != nil {
		return importResult{}, fmt.Errorf("recherche du projet : %w", err)
	}

	switch len(projects) {
	case 0:
		return importResult{Outcome: ImportRowError, Reason: ReasonUnknownProject, Label: label + " : project : projet introuvable chez ce client"}, nil
	case 1:
	default:
		return invalid("project", "plusieurs projets portent ce nom : indiquez le client"), nil
	}

	projectID := projects[0].ID

	var serviceID *uuid.UUID

	if s := strings.TrimSpace(v["service"]); s != "" {
		switch id, err := q.FindServiceByName(ctx, s); {
		case err == nil:
			serviceID = &id
		case errors.Is(err, pgx.ErrNoRows):
			return invalid("service", "service inconnu"), nil
		default:
			return importResult{}, fmt.Errorf("recherche du service : %w", err)
		}
	}

	note := strings.TrimSpace(v["note"])

	natural := strings.Join([]string{
		"time", userID.String(), projectID.String(), day.Format("2006-01-02"),
		strconv.Itoa(minutes), uuidKey(serviceID), note, strings.TrimSpace(v[FieldKeyID]),
	}, "|")

	check := replayCheck{
		entity: DataEntityTimeEntries, label: label,
		exists: func(ctx context.Context, q *db.Queries, id uuid.UUID) (bool, error) {
			ok, err := q.TimeEntryExists(ctx, id)
			if err != nil {
				return false, fmt.Errorf("recherche de la saisie : %w", err)
			}

			return ok, nil
		},
	}

	res, sourceKey, done, err := check.run(ctx, q, v, natural)
	if err != nil || done {
		return res, err
	}

	id, err := q.CreateTimeEntry(ctx, db.CreateTimeEntryParams{
		UserID: userID, ProjectID: projectID, ServiceID: serviceID, SpentOn: day, Minutes: int32(minutes), Note: note,
	})
	if err != nil {
		return importResult{Label: label}, fmt.Errorf("creation de la saisie : %w", err)
	}

	if err := q.UpsertImportKey(ctx, db.UpsertImportKeyParams{
		Entity: DataEntityTimeEntries, SourceKey: sourceKey, EntityID: id, ImportID: &importID,
	}); err != nil {
		return importResult{}, fmt.Errorf("ecriture de la cle d'import : %w", err)
	}

	return importResult{Outcome: ImportRowCreated, Label: label, EntityID: &id}, nil
}
