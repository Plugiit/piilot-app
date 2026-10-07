// Commande backup : sauvegarde la base et les fichiers, chaque jour, et sait
// restaurer.
//
// Elle tourne dans son propre conteneur (service « backup » du
// docker-compose), avec la base en lecture et les fichiers montes en lecture
// seule. Elle ne sert rien et ne parle a personne : elle ecrit des archives
// dans son volume, les copie sur S3 si on lui en donne un, et note chaque
// passage dans la table app_backups, que l'ecran des parametres lit.
//
//	backup              sauvegarde a l'heure dite, chaque jour ; au demarrage
//	                    aussi, s'il n'y en a pas eu depuis vingt-quatre heures
//	backup now          une sauvegarde, tout de suite, puis s'arrete
//	backup restore [f]  restaure l'archive f (la plus recente du dossier sans f)
//	backup list         les archives du dossier
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/backup"
	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/repository"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// version est injectee au build (-ldflags "-X main.version=...").
var version = "dev"

// Une sauvegarde entiere, dump et copie comprises, doit tenir la-dedans.
const backupTimeout = 2 * time.Hour

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(log, os.Args[1:]); err != nil {
		log.Error("arret", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, args []string) error {
	cfg, err := config.LoadBackup()
	if err != nil {
		return err
	}
	if cfg.LogLevel == "debug" {
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := &backup.Runner{
		DatabaseURL: cfg.DatabaseURL,
		FilesDir:    cfg.FilesDir,
		Dir:         cfg.Dir,
		Keep:        backup.Retention{Daily: cfg.KeepDaily, Weekly: cfg.KeepWeekly, Monthly: cfg.KeepMonthly},
		Log:         log,
	}
	if cfg.S3Endpoint != "" {
		runner.Remote = &backup.S3{
			Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, Region: cfg.S3Region,
			AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Prefix: cfg.S3Prefix,
		}
	}

	mode := "daemon"
	if len(args) > 0 {
		mode = args[0]
	}

	switch mode {
	case "list":
		local, err := runner.Local()
		if err != nil {
			return err
		}
		names := make([]string, 0, len(local))
		for n := range local {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Println(n)
		}
		return nil

	case "restore":
		archive := ""
		if len(args) > 1 {
			archive = args[1]
		} else if archive, err = runner.Latest(); err != nil {
			return err
		}
		if archive == "" {
			return errors.New("aucune archive a restaurer")
		}
		log.Info("restauration", "archive", archive)
		if err := runner.Restore(ctx, archive); err != nil {
			return err
		}
		log.Info("restauration terminee", "archive", archive)
		return nil

	case "now":
		pool, err := repository.NewPool(ctx, cfg.DatabaseURL, repository.DefaultPoolConfig())
		if err != nil {
			return err
		}
		defer pool.Close()
		return once(ctx, pool, runner, log)

	case "daemon":
		pool, err := repository.NewPool(ctx, cfg.DatabaseURL, repository.DefaultPoolConfig())
		if err != nil {
			return err
		}
		defer pool.Close()
		return daemon(ctx, pool, runner, cfg.At, log)

	default:
		return fmt.Errorf("commande inconnue : %s (now, restore, list)", mode)
	}
}

// daemon sauvegarde chaque jour a l'heure dite. Au demarrage, tout de suite
// s'il n'y a pas eu de sauvegarde reussie depuis vingt-quatre heures : une
// instance neuve, ou un conteneur reste arrete, n'attend pas la nuit.
func daemon(ctx context.Context, pool *pgxpool.Pool, runner *backup.Runner, at string, log *slog.Logger) error {
	log.Info("backup demarre", "version", version, "at", at, "dir", runner.Dir, "s3", runner.Remote.Configured())

	// Au premier demarrage d'une pile, l'application n'a pas encore cree la
	// table : on repasse toutes les deux minutes jusqu'a pouvoir la lire.
	for !catchUp(ctx, pool, runner, log) {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Minute):
		}
	}

	for {
		next := nextRun(time.Now(), at)
		log.Info("prochaine sauvegarde", "at", next.Format(time.RFC3339))
		select {
		case <-ctx.Done():
			log.Info("arret demande")
			return nil
		case <-time.After(time.Until(next)):
			if err := once(ctx, pool, runner, log); err != nil {
				log.Error("sauvegarde", "error", err)
			}
		}
	}
}

// catchUp fait une sauvegarde s'il n'y en a pas eu de reussie depuis
// vingt-quatre heures. Faux tant que le journal n'est pas lisible.
func catchUp(ctx context.Context, pool *pgxpool.Pool, runner *backup.Runner, log *slog.Logger) bool {
	last, err := db.New(pool).GetLastSuccessfulBackup(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		log.Warn("lecture de la derniere sauvegarde", "error", err)
		return false
	case time.Since(last.StartedAt) <= 24*time.Hour:
		return true
	}

	if err := once(ctx, pool, runner, log); err != nil {
		log.Error("sauvegarde de rattrapage", "error", err)
	}
	return true
}

// nextRun rend la prochaine occurrence de l'heure « HH:MM » apres now, en
// heure locale.
func nextRun(now time.Time, at string) time.Time {
	hm, _ := time.Parse("15:04", at)
	next := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// once fait une sauvegarde et la journalise : une ligne « en cours » avant,
// completee apres, pour que l'ecran des parametres voie aussi les echecs.
func once(ctx context.Context, pool *pgxpool.Pool, runner *backup.Runner, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, backupTimeout)
	defer cancel()

	q := db.New(pool)
	row, err := q.CreateBackup(ctx)
	if err != nil {
		return fmt.Errorf("journal : %w", err)
	}

	started := time.Now()
	result, err := runner.Run(ctx, started)
	status, problem := "done", ""
	if err != nil {
		status, problem = "failed", err.Error()
	}
	if err := q.FinishBackup(ctx, db.FinishBackupParams{
		ID: row.ID, Status: status, Location: result.Location, SizeBytes: result.Size, Error: problem,
	}); err != nil {
		log.Warn("journal de la sauvegarde", "error", err)
	}
	if err := q.PruneBackupRows(ctx); err != nil {
		log.Warn("menage du journal", "error", err)
	}

	if problem != "" {
		return errors.New(problem)
	}
	log.Info("sauvegarde faite", "archive", result.Path, "octets", result.Size, "duree", time.Since(started).Round(time.Second).String())
	return nil
}
