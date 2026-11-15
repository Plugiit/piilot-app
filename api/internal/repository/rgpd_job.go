package repository

import (
	"context"
	"log/slog"
	"time"
)

// Reglages de la tache RGPD.
const (
	// RGPDInterval : un export demande est attendu par un admin, mais pas a la
	// seconde ; la meme tache supprime les archives expirees.
	RGPDInterval = time.Minute
	// RGPDExportTimeout borne la construction d'une archive. Plus court que le
	// delai apres lequel un export « en cours » est tenu pour abandonne.
	RGPDExportTimeout = 25 * time.Minute
)

// RGPDWorker est ce que la tache attend du service d'export : construire le
// prochain export en file (vrai s'il y en avait un), et supprimer ce qui a
// expire.
type RGPDWorker interface {
	Process(ctx context.Context) (bool, error)
	Purge(ctx context.Context) (int, error)
}

// StartRGPD construit les exports en file et supprime les archives expirees,
// puis rend la main immediatement. La goroutine s'arrete avec le contexte.
// Chaque export est reserve sous verrou par le service : deux instances ne
// construisent jamais la meme archive.
func StartRGPD(ctx context.Context, w RGPDWorker, log *slog.Logger) {
	go every(ctx, RGPDInterval, func() {
		if n, err := w.Purge(ctx); err != nil {
			log.Warn("purge des exports rgpd", "error", err)
		} else if n > 0 {
			log.Info("archives rgpd expirees supprimees", "count", n)
		}

		for ctx.Err() == nil {
			worked, err := processOneRGPD(ctx, w)
			if err != nil {
				log.Warn("export rgpd", "error", err)
			}

			if !worked || err != nil {
				return
			}
		}
	})
}

func processOneRGPD(ctx context.Context, w RGPDWorker) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, RGPDExportTimeout)
	defer cancel()

	return w.Process(ctx)
}
