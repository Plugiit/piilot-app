package repository

import (
	"context"
	"log/slog"
	"time"
)

// Reglages de la tache RGPD.
const (
	// RGPDInterval : un export demande est attendu par un admin, mais pas a la
	// seconde ; la meme tache supprime les archives expirees et execute les
	// effacements arrives a echeance.
	RGPDInterval = time.Minute
	// RGPDExportTimeout borne la construction d'une archive. Plus court que le
	// delai apres lequel un export « en cours » est tenu pour abandonne.
	RGPDExportTimeout = 25 * time.Minute
	// RGPDErasureTimeout borne l'execution d'un effacement (une transaction).
	RGPDErasureTimeout = 10 * time.Minute
	// RGPDInactiveInterval : la purge des comptes inactifs passe une fois par jour.
	RGPDInactiveInterval = 24 * time.Hour
)

// RGPDWorker est ce que la tache attend du service d'export : construire le
// prochain export en file (vrai s'il y en avait un), et supprimer ce qui a
// expire.
type RGPDWorker interface {
	Process(ctx context.Context) (bool, error)
	Purge(ctx context.Context) (int, error)
}

// RGPDErasureWorker est ce que la tache attend du service d'effacement :
// executer le prochain effacement arrive a echeance (vrai s'il y en avait un),
// et planifier les comptes inactifs.
type RGPDErasureWorker interface {
	Execute(ctx context.Context) (bool, error)
	ScheduleInactive(ctx context.Context) (int, error)
}

// StartRGPD construit les exports en file, supprime les archives expirees,
// execute les effacements arrives a echeance et, une fois par jour, planifie
// ceux des comptes inactifs, puis rend la main immediatement. La goroutine
// s'arrete avec le contexte. Chaque export et chaque effacement est reserve
// sous verrou par son service : deux instances ne font jamais le meme.
func StartRGPD(ctx context.Context, w RGPDWorker, erasures RGPDErasureWorker, log *slog.Logger) {
	var lastInactivePass time.Time

	go every(ctx, RGPDInterval, func() {
		runRGPDExports(ctx, w, log)
		runRGPDErasures(ctx, erasures, log)

		if ctx.Err() == nil && time.Since(lastInactivePass) >= RGPDInactiveInterval {
			lastInactivePass = time.Now()

			if n, err := erasures.ScheduleInactive(ctx); err != nil {
				log.Warn("purge rgpd des comptes inactifs", "error", err)
			} else if n > 0 {
				log.Info("effacements rgpd planifies pour inactivite", "count", n)
			}
		}
	})
}

func runRGPDExports(ctx context.Context, w RGPDWorker, log *slog.Logger) {
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
}

func runRGPDErasures(ctx context.Context, erasures RGPDErasureWorker, log *slog.Logger) {
	for ctx.Err() == nil {
		worked, err := executeOneRGPDErasure(ctx, erasures)
		if err != nil {
			log.Warn("effacement rgpd", "error", err)
		}

		// Un effacement en echec est marque tel quel et ne se rejoue pas ; le
		// suivant passe a la minute d'apres.
		if !worked || err != nil {
			return
		}
	}
}

func processOneRGPD(ctx context.Context, w RGPDWorker) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, RGPDExportTimeout)
	defer cancel()

	return w.Process(ctx)
}

func executeOneRGPDErasure(ctx context.Context, w RGPDErasureWorker) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, RGPDErasureTimeout)
	defer cancel()

	return w.Execute(ctx)
}
