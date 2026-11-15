package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// contactImporter importe un contact.
//
// Le client se retrouve par SIRET puis par nom ; sans client dans la ligne,
// le contact est libre. Un client nomme mais inconnu est refuse
// (unknown_client) : on ne cree jamais de client a la volee.
// Doublon : l'e-mail (insensible a la casse, tous clients confondus), sinon
// prenom + nom chez le meme client.
type contactImporter struct{}

func (contactImporter) importRow(ctx context.Context, q *db.Queries, importID uuid.UUID, v map[string]string) (importResult, error) {
	firstname := strings.TrimSpace(v["firstname"])
	lastname := strings.TrimSpace(v["lastname"])
	email := strings.TrimSpace(v["email"])
	label := strings.TrimSpace(firstname + " " + lastname)

	if label == "" {
		label = email
	}

	invalid := func(field, msg string) importResult {
		return importResult{Outcome: ImportRowError, Reason: ReasonInvalid, Label: label + " : " + field + " : " + msg}
	}

	if firstname == "" && lastname == "" {
		return invalid("firstname", "un prénom ou un nom est requis"), nil
	}

	if email != "" {
		if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
			return invalid("email", "adresse e-mail invalide"), nil
		}
	}

	check := replayCheck{
		entity: DataEntityContacts, label: label,
		exists: func(ctx context.Context, q *db.Queries, id uuid.UUID) (bool, error) {
			switch _, err := q.GetContact(ctx, id); {
			case err == nil:
				return true, nil
			case errors.Is(err, pgx.ErrNoRows):
				return false, nil
			default:
				return false, fmt.Errorf("recherche du contact : %w", err)
			}
		},
	}

	if res, done, err := check.checkID(ctx, q, v); err != nil || done {
		return res, err
	}

	var client *db.Client

	if siret, name := v["client_siret"], v["client"]; normalizeSiret(siret) != "" || strings.TrimSpace(name) != "" {
		found, err := resolveClient(ctx, q, siret, name)
		if err != nil {
			return importResult{}, err
		}

		if found == nil {
			return importResult{Outcome: ImportRowError, Reason: ReasonUnknownClient, Label: label + " : client : client introuvable"}, nil
		}

		client = found
	}

	var clientID *uuid.UUID
	if client != nil {
		clientID = &client.ID
	}

	natural := "name:" + strings.ToLower(firstname) + "|" + strings.ToLower(lastname) + "|" + uuidKey(clientID)
	if email != "" {
		natural = "email:" + strings.ToLower(email)
	}

	res, sourceKey, done, err := check.checkKey(ctx, q, v, natural)
	if err != nil || done {
		return res, err
	}

	if email != "" {
		switch existing, err := q.GetContactByEmail(ctx, email); {
		case err == nil:
			return importResult{Outcome: ImportRowSkipped, Reason: ReasonDuplicateEmail, Label: label, EntityID: &existing.ID}, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return importResult{}, fmt.Errorf("recherche de l'e-mail : %w", err)
		}
	} else {
		switch existing, err := q.GetContactByName(ctx, db.GetContactByNameParams{Firstname: firstname, Lastname: lastname, ClientID: clientID}); {
		case err == nil:
			return importResult{Outcome: ImportRowSkipped, Reason: ReasonDuplicateName, Label: label, EntityID: &existing.ID}, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return importResult{}, fmt.Errorf("recherche du nom : %w", err)
		}
	}

	var contactEmail *string
	if email != "" {
		contactEmail = &email
	}

	contact, err := q.CreateContact(ctx, db.CreateContactParams{
		ClientID: clientID, Firstname: firstname, Lastname: lastname,
		Role: strings.TrimSpace(v["role"]), Email: contactEmail, Phone: strings.TrimSpace(v["phone"]),
	})
	if err != nil {
		return importResult{Label: label}, fmt.Errorf("creation du contact : %w", err)
	}

	// Comme a la creation depuis l'ecran : le premier contact d'un client
	// en devient l'interlocuteur principal.
	if client != nil && client.PrimaryContactID == nil {
		if err := q.SetPrimaryContact(ctx, db.SetPrimaryContactParams{ClientID: client.ID, ContactID: &contact.ID}); err != nil {
			return importResult{}, fmt.Errorf("designation du contact principal : %w", err)
		}
	}

	if err := q.UpsertImportKey(ctx, db.UpsertImportKeyParams{
		Entity: DataEntityContacts, SourceKey: sourceKey, EntityID: contact.ID, ImportID: &importID,
	}); err != nil {
		return importResult{}, fmt.Errorf("ecriture de la cle d'import : %w", err)
	}

	return importResult{Outcome: ImportRowCreated, Label: label, EntityID: &contact.ID}, nil
}

// uuidKey ecrit un identifiant optionnel pour une cle naturelle.
func uuidKey(id *uuid.UUID) string {
	if id == nil {
		return "-"
	}

	return id.String()
}
