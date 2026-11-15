package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/csvio"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// replayCheck porte les deux controles de rejeu communs a toutes les entites,
// dans l'ordre du plan :
//  1. « Identifiant Piilot » d'une entite vivante : already_exists ;
//  2. cle d'import (identifiant d'origine, sinon cle naturelle) d'une entite
//     encore vivante : already_imported.
//
// Un doublon est ignore, jamais mis a jour.
type replayCheck struct {
	entity string
	label  string
	// exists dit si l'entite designee est encore vivante.
	exists func(ctx context.Context, q *db.Queries, id uuid.UUID) (bool, error)
}

// run rend done=true quand la ligne est tranchee (res porte le resultat).
func (c replayCheck) run(ctx context.Context, q *db.Queries, v map[string]string, natural string) (res importResult, sourceKey string, done bool, err error) {
	if raw := strings.TrimSpace(v[FieldKeyID]); raw != "" {
		id, perr := uuid.Parse(raw)
		if perr != nil {
			return importResult{Outcome: ImportRowError, Reason: ReasonInvalid, Label: c.label + " : id : identifiant invalide"}, "", true, nil
		}

		ok, eerr := c.exists(ctx, q, id)
		if eerr != nil {
			return importResult{}, "", false, eerr
		}

		if ok {
			return importResult{Outcome: ImportRowSkipped, Reason: ReasonAlreadyExists, Label: c.label, EntityID: &id}, "", true, nil
		}
	}

	sourceKey = importSourceKey(strings.TrimSpace(v[FieldKeySourceID]), natural)

	known, kerr := q.GetImportKey(ctx, db.GetImportKeyParams{Entity: c.entity, SourceKey: sourceKey})

	switch {
	case kerr == nil:
		// La cle ne vaut que si l'entite existe encore : supprimee depuis,
		// elle est recreee (et la cle repointee a l'ecriture).
		ok, eerr := c.exists(ctx, q, known.EntityID)
		if eerr != nil {
			return importResult{}, "", false, eerr
		}

		if ok {
			return importResult{Outcome: ImportRowSkipped, Reason: ReasonAlreadyImported, Label: c.label, EntityID: &known.EntityID}, sourceKey, true, nil
		}
	case !errors.Is(kerr, pgx.ErrNoRows):
		return importResult{}, "", false, fmt.Errorf("lecture de la cle d'import : %w", kerr)
	}

	return importResult{}, sourceKey, false, nil
}

// resolveClient retrouve un client vivant : par SIRET d'abord, puis par nom.
// Rend nil quand rien ne correspond ; un import ne cree jamais de client.
func resolveClient(ctx context.Context, q *db.Queries, siret, name string) (*db.Client, error) {
	siret = normalizeSiret(siret)

	if siret != "" {
		switch c, err := q.GetClientBySiret(ctx, siret); {
		case err == nil:
			return &c, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return nil, fmt.Errorf("recherche du client par SIRET : %w", err)
		}
	}

	if name = strings.TrimSpace(name); name != "" {
		switch c, err := q.GetClientByName(ctx, name); {
		case err == nil:
			// Un SIRET fourni mais inconnu ne se rabat sur le nom que si le
			// client trouve n'a pas de SIRET : sinon c'est un homonyme.
			if siret != "" && c.Siret != "" {
				return nil, nil
			}

			return &c, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return nil, fmt.Errorf("recherche du client par nom : %w", err)
		}
	}

	return nil, nil
}

// enumValue lit une valeur de liste fermee : le code, ou l'un de ses libelles,
// sans tenir compte de la casse, des accents ni de la ponctuation.
func enumValue(raw string, aliases map[string]string) (string, bool) {
	v, ok := aliases[csvio.NormalizeHeader(raw)]

	return v, ok
}
