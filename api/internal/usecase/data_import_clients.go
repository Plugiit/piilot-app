package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// clientImporter importe la fiche d'un client.
//
// L'ordre des controles est celui du plan : un doublon est ignore, jamais
// mis a jour.
//  1. « Identifiant Piilot » d'un client vivant : already_exists ;
//  2. cle d'import (identifiant d'origine, sinon cle naturelle) d'un client
//     encore vivant : already_imported ;
//  3. doublon metier : SIRET, puis nom.
type clientImporter struct{}

func (clientImporter) importRow(ctx context.Context, q *db.Queries, importID uuid.UUID, v map[string]string) (importResult, error) {
	name := strings.TrimSpace(v["name"])
	label := name

	if label == "" {
		label = strings.TrimSpace(v["legal_name"])
	}

	invalid := func(field, msg string) importResult {
		return importResult{Outcome: ImportRowError, Reason: ReasonInvalid, Label: label + " : " + field + " : " + msg}
	}

	if name == "" {
		return invalid("name", "le nom est requis"), nil
	}

	if len([]rune(name)) > 200 {
		return invalid("name", "200 caractères au plus"), nil
	}

	status := strings.ToLower(strings.TrimSpace(v["status"]))
	if status == "" {
		status = "lead"
	}

	if _, ok := clientStatuses[status]; !ok {
		return invalid("status", "étape inconnue (lead, devis, actif, veille ou perdu)"), nil
	}

	identity := ClientIdentity{
		Kind:       strings.ToLower(strings.TrimSpace(v["kind"])),
		LegalName:  v["legal_name"],
		LegalForm:  v["legal_form"],
		Siret:      v["siret"],
		VatNumber:  v["vat_number"],
		Phone:      v["phone"],
		Address:    v["address"],
		PostalCode: v["postal_code"],
		City:       v["city"],
		Country:    v["country"],
	}

	website := strings.TrimSpace(v["website"])
	if website != "" && !isHTTPURL(website) {
		return invalid("website", "adresse web invalide (http ou https)"), nil
	}

	if errs := identity.normalize(false); len(errs) > 0 {
		keys := make([]string, 0, len(errs))
		for k := range errs {
			keys = append(keys, k)
		}

		slices.Sort(keys)

		return invalid(keys[0], fmt.Sprint(errs[keys[0]])), nil
	}

	// 1 et 2. Identifiant Piilot, puis cle d'import.
	check := replayCheck{
		entity: DataEntityClients, label: label,
		exists: func(ctx context.Context, q *db.Queries, id uuid.UUID) (bool, error) {
			switch _, err := q.GetClientByID(ctx, id); {
			case err == nil:
				return true, nil
			case errors.Is(err, pgx.ErrNoRows):
				return false, nil
			default:
				return false, fmt.Errorf("recherche du client : %w", err)
			}
		},
	}

	res, sourceKey, done, err := check.run(ctx, q, v, clientNaturalKey(identity.Siret, name))
	if err != nil || done {
		return res, err
	}

	// 3. Doublons metier.
	if identity.Siret != "" {
		switch existing, err := q.GetClientBySiret(ctx, identity.Siret); {
		case err == nil:
			return importResult{Outcome: ImportRowSkipped, Reason: ReasonDuplicateSiret, Label: label, EntityID: &existing.ID}, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return importResult{}, fmt.Errorf("recherche du SIRET : %w", err)
		}
	}

	switch existing, err := q.GetClientByName(ctx, name); {
	case err == nil:
		return importResult{Outcome: ImportRowSkipped, Reason: ReasonDuplicateName, Label: label, EntityID: &existing.ID}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return importResult{}, fmt.Errorf("recherche du nom : %w", err)
	}

	client, err := q.CreateImportedClient(ctx, db.CreateImportedClientParams{
		Name: name, Kind: identity.Kind, LegalName: identity.LegalName, LegalForm: identity.LegalForm,
		Siret: identity.Siret, VatNumber: identity.VatNumber, Status: status, Website: website,
		Phone: identity.Phone, Address: identity.Address, PostalCode: identity.PostalCode,
		City: identity.City, Country: identity.Country,
	})
	if err != nil {
		// Renvoye tel quel : l'appelant sait lire une violation d'unicite.
		return importResult{Label: label}, fmt.Errorf("creation du client : %w", err)
	}

	if err := q.UpsertImportKey(ctx, db.UpsertImportKeyParams{
		Entity: DataEntityClients, SourceKey: sourceKey, EntityID: client.ID, ImportID: &importID,
	}); err != nil {
		return importResult{}, fmt.Errorf("ecriture de la cle d'import : %w", err)
	}

	return importResult{Outcome: ImportRowCreated, Label: label, EntityID: &client.ID}, nil
}

// clientNaturalKey : le SIRET quand il y en a un, sinon le nom en minuscules.
func clientNaturalKey(siret, name string) string {
	if siret != "" {
		return "siret:" + siret
	}

	return "name:" + strings.ToLower(strings.TrimSpace(name))
}

// importSourceKey est la cle de rejeu d'une ligne : l'identifiant d'origine
// quand le fichier en porte un, sinon l'empreinte de la cle naturelle. Des
// empreintes seulement : import_keys ne garde aucune donnee.
func importSourceKey(sourceID, natural string) string {
	if sourceID != "" {
		return "ext:" + sourceID
	}

	sum := sha256.Sum256([]byte(natural))

	return "nat:" + hex.EncodeToString(sum[:])
}

// isHTTPURL accepte une adresse absolue en http ou https, avec un hote.
func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)

	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
