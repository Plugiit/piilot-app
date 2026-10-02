// Commande updater : met l'application a jour quand un admin le demande.
//
// Elle tourne dans son propre conteneur, a cote de l'application, avec le
// socket Docker monte — et elle est la seule. L'application, exposee sur
// Internet, n'a jamais acces a Docker : une faille chez elle ne donne pas la
// main sur le serveur.
//
// Les deux ne se parlent que par la base. L'updater :
//   - signale sa presence toutes les trente secondes (app_updater), avec le
//     conteneur qu'il mettra a jour ou ce qui l'en empeche ;
//   - prend les demandes en attente (app_update_requests), tire la nouvelle
//     image, recree le conteneur de l'application et note le resultat.
//
// Il n'expose aucun port et n'obeit qu'a une seule commande : mettre a jour le
// service « app » de son propre projet Compose vers l'image publiee.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/docker"
	"github.com/plugiit/piilot-app/api/internal/repository"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/updater"
)

// version est injectee au build (-ldflags "-X main.version=...").
var version = "dev"

// Rythme de l'updater.
const (
	// pollInterval : delai entre le clic d'un admin et le debut de la mise a
	// jour. La requete ne coute rien quand il n'y a rien a faire.
	pollInterval = 10 * time.Second
	// heartbeatInterval : l'API juge l'updater absent apres deux minutes sans
	// signe de vie.
	heartbeatInterval = 30 * time.Second
	// updateTimeout borne une mise a jour entiere : tirage de l'image,
	// redemarrage, migrations et verification de sante.
	updateTimeout = 15 * time.Minute
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(log); err != nil {
		log.Error("arret de l'updater", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.LoadUpdater()
	if err != nil {
		return err
	}
	if cfg.LogLevel == "debug" {
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL, repository.DefaultPoolConfig())
	if err != nil {
		return err
	}
	defer pool.Close()

	u := &updater.Updater{
		Docker:  docker.NewUnix(cfg.DockerSocket),
		Service: cfg.Service,
		Self:    cfg.Self,
	}

	q := db.New(pool)

	// Une demande restee « en cours » vient d'un updater interrompu en pleine
	// mise a jour : elle ne bloque pas les suivantes. Echoue sans dommage si
	// l'application n'a pas encore cree les tables : ce sera pour plus tard.
	if err := q.AbandonStaleUpdateRequests(ctx); err != nil {
		log.Warn("cloture des demandes interrompues", "error", err)
	}

	log.Info("updater demarre", "version", version, "service", cfg.Service)

	heartbeat(ctx, q, u, log)
	beat := time.NewTicker(heartbeatInterval)
	poll := time.NewTicker(pollInterval)
	defer beat.Stop()
	defer poll.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("arret demande")
			return nil
		case <-beat.C:
			heartbeat(ctx, q, u, log)
		case <-poll.C:
			process(ctx, pool, u, log)
		}
	}
}

// heartbeat signale la presence de l'updater, avec le conteneur qu'il mettra a
// jour ou ce qui l'en empeche — un socket Docker non monte, typiquement.
func heartbeat(ctx context.Context, q *db.Queries, u *updater.Updater, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var target, problem string
	if c, err := u.Target(ctx); err != nil {
		problem = err.Error()
	} else {
		target = strings.TrimPrefix(c.Name, "/")
	}

	if err := q.SaveUpdaterHeartbeat(ctx, db.SaveUpdaterHeartbeatParams{
		Version: version, Target: target, Error: problem,
	}); err != nil {
		log.Warn("signal de presence", "error", err)
	}
}

// process execute la demande en attente, s'il y en a une.
func process(ctx context.Context, pool *pgxpool.Pool, u *updater.Updater, log *slog.Logger) {
	q := db.New(pool)

	request, err := q.StartPendingUpdateRequest(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		log.Warn("lecture des demandes", "error", err)
		return
	}

	log.Info("mise a jour demandee", "request", request.ID,
		"from", request.FromVersion, "to", request.TargetVersion)

	// Contexte propre a la mise a jour, detache de l'arret de l'updater : un
	// SIGTERM en plein milieu ne doit pas laisser l'application arretee. Le
	// retour arriere de l'updater a besoin de pouvoir finir.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), updateTimeout)
	defer cancel()

	step := func(label string) {
		log.Info("mise a jour", "request", request.ID, "etape", label)
		if err := q.SetUpdateRequestStep(runCtx, db.SetUpdateRequestStepParams{ID: request.ID, Step: label}); err != nil {
			log.Warn("enregistrement de l'etape", "error", err)
		}
	}

	status, message := "done", ""
	target, err := u.Target(runCtx)
	if err == nil {
		err = u.Update(runCtx, target, step)
	}
	if err != nil {
		status, message = "failed", err.Error()
		log.Error("mise a jour echouee", "request", request.ID, "error", err)
	} else {
		log.Info("mise a jour terminee", "request", request.ID, "to", request.TargetVersion)
	}

	if err := q.FinishUpdateRequest(runCtx, db.FinishUpdateRequestParams{
		ID: request.ID, Status: status, Error: message,
	}); err != nil {
		log.Warn("enregistrement du resultat", "error", err)
	}
}
