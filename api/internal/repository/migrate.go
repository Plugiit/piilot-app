package repository

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/plugiit/piilot-app/api/migrations"
)

// Migrate applique les migrations en attente au demarrage.
//
// golang-migrate prend un verrou consultatif Postgres : si Coolify demarre
// plusieurs replicas en meme temps, un seul migre et les autres attendent, au
// lieu d'appliquer le meme DDL en concurrence.
func Migrate(databaseURL string, log *slog.Logger) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("lecture des migrations embarquees : %w", err)
	}

	// Le driver pgx/v5 attend un DSN en schema pgx5://.
	m, err := migrate.NewWithSourceInstance("iofs", source, pgxDSN(databaseURL))
	if err != nil {
		return fmt.Errorf("initialisation du migrateur : %w", err)
	}
	defer func() {
		if sourceErr, dbErr := m.Close(); sourceErr != nil || dbErr != nil {
			log.Warn("fermeture du migrateur", "source_error", sourceErr, "db_error", dbErr)
		}
	}()

	before, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("lecture de la version courante : %w", err)
	}

	if dirty {
		return fmt.Errorf("schema en etat dirty a la version %d : intervention manuelle requise", before)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("schema a jour", "version", before)
			return nil
		}
		return fmt.Errorf("application des migrations : %w", err)
	}

	after, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("lecture de la version finale : %w", err)
	}

	log.Info("migrations appliquees", "from", before, "to", after)

	return nil
}

// pgxDSN convertit postgres://... en pgx5://... attendu par le driver.
func pgxDSN(databaseURL string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if len(databaseURL) > len(prefix) && databaseURL[:len(prefix)] == prefix {
			return "pgx5://" + databaseURL[len(prefix):]
		}
	}
	return databaseURL
}

// assertion de compilation : le driver pgx/v5 doit etre enregistre.
var _ = pgx.Postgres{}
