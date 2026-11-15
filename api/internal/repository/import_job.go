package repository

import (
	"context"
	"log/slog"
	"time"
)

// Reglages du traitement des imports CSV.
const (
	// ImportInterval : un import lance depuis l'ecran est attendu par
	// quelqu'un ; quelques secondes de delai suffisent.
	ImportInterval = 5 * time.Second
	// ImportTimeout borne le traitement d'un fichier.
	ImportTimeout = 10 * time.Minute
)

// ImportWorker est ce que la tache attend du service d'import : traiter le
// prochain import en file, et dire s'il y en avait un.
type ImportWorker interface {
	Process(ctx context.Context) (bool, error)
}

// StartImports traite les imports en file et rend la main immediatement. La
// goroutine s'arrete avec le contexte. Chaque import est reserve sous verrou
// par le service : deux instances ne prennent jamais le meme.
func StartImports(ctx context.Context, w ImportWorker, log *slog.Logger) {
	go every(ctx, ImportInterval, func() {
		for {
			if ctx.Err() != nil {
				return
			}

			worked, err := processOneImport(ctx, w)
			if err != nil {
				log.Warn("import csv", "error", err)
			}

			if !worked {
				return
			}
		}
	})
}

func processOneImport(ctx context.Context, w ImportWorker) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, ImportTimeout)
	defer cancel()

	return w.Process(ctx)
}
