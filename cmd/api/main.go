// Commande api : point d'entree du backend Plugiit.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/joho/godotenv"

	"github.com/plugiit/plugiit-api-go/internal/config"
	"github.com/plugiit/plugiit-api-go/internal/handler"
	"github.com/plugiit/plugiit-api-go/internal/middleware"
	"github.com/plugiit/plugiit-api-go/internal/repository"
	"github.com/plugiit/plugiit-api-go/internal/security"
	"github.com/plugiit/plugiit-api-go/internal/storage"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

// Injectes au build via -ldflags (voir Dockerfile et Makefile).
var (
	version = "dev"
	commit  = "none"
)

func main() {
	// En local uniquement : en conteneur, les variables viennent de Coolify.
	_ = godotenv.Load()

	log := newLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Error("configuration invalide", "error", err)
		os.Exit(1)
	}

	if err := run(cfg, log); err != nil {
		log.Error("arret sur erreur", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Les migrations tournent avant l'ouverture du pool applicatif : si le
	// schema ne peut pas etre amene a jour, le conteneur echoue au demarrage et
	// Coolify garde la version precedente en ligne.
	if cfg.RunMigrations {
		if err := repository.Migrate(cfg.DatabaseURL, log); err != nil {
			return err
		}
	}

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL, repository.DefaultPoolConfig())
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := repository.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	app := newApp(cfg, log)

	signer := security.NewTokenSigner(cfg.JWTSecret, "plugiit-api")
	files, err := storage.NewLocal(cfg.FilesDir)
	if err != nil {
		return err
	}

	// Une photo de profil se borne plus serre qu'une piece jointe : elle est
	// relue a chaque affichage de liste, et deux megaoctets suffisent largement
	// a un portrait.
	authService := usecase.NewAuthService(
		pool, signer, cfg.AccessTTL, cfg.RefreshTTL, files, 2*(1<<20),
	)

	projectService := usecase.NewProjectService(pool, files, cfg.MaxUploadMiB*(1<<20))
	// Le bus porte les notifications jusqu'aux flux ouverts ; le service les
	// ecrit et les relit.
	notifyBus := repository.NewNotifyBus(rdb)
	notificationService := usecase.NewNotificationService(pool, notifyBus)

	taskService := usecase.NewTaskService(pool, files, cfg.MaxUploadMiB*(1<<20), notifyBus)

	clientService := usecase.NewClientService(pool)
	contactService := usecase.NewContactService(pool)

	cookies := handler.CookieConfig{
		Domain: cfg.CookieDomain,
		// Secure partout sauf en developpement : en local l'API est servie en
		// clair sur localhost, ou un cookie Secure ne serait jamais renvoye.
		Secure: !cfg.IsDevelopment(),
	}

	handler.Register(app, handler.Deps{
		Health:   handler.NewHealth(pool, rdb, handler.BuildInfo{Version: version, Commit: commit}),
		Auth:     handler.NewAuth(authService, cookies, repository.NewRateLimiter(rdb), log),
		Projects: handler.NewProjects(projectService),
		Tasks:    handler.NewTasks(taskService),
		Clients:  handler.NewClients(clientService),
		Contacts: handler.NewContacts(contactService),

		Notifications: handler.NewNotifications(notificationService, notifyBus),
		Tickets:       handler.NewTickets(usecase.NewTicketService(pool)),
		Guard:         middleware.NewGuard(signer, authService),
	})

	// Purge des jetons expires en tache de fond. S'arrete avec le contexte,
	// donc au premier signal d'arret.
	repository.StartRefreshTokenPurge(ctx, pool, log)

	// Le serveur tourne dans sa goroutine pour que main puisse attendre le
	// signal d'arret et fermer proprement les connexions en cours.
	errCh := make(chan error, 1)
	go func() {
		log.Info("api demarree", "addr", cfg.Addr(), "env", cfg.Env, "version", version)
		if err := app.Listen(cfg.Addr(), fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		log.Info("arret demande, fermeture en cours", "timeout", cfg.ShutdownTimeout)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		return err
	}

	log.Info("arret propre")

	return nil
}

func newApp(cfg config.Config, log *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "plugiit-api",
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		ErrorHandler: middleware.ErrorHandler(log),
		// Le plafond du serveur suit celui des pieces jointes, avec un Mo de
		// marge pour l'enveloppe multipart. Sans ce reglage, Fiber refuserait
		// a 4 Mo par defaut, bien avant la limite annoncee a l'utilisateur.
		BodyLimit: int(cfg.MaxUploadMiB+1) * (1 << 20),
	})

	app.Use(requestid.New())
	app.Use(recover.New())

	if cfg.IsDevelopment() {
		app.Use(logger.New())
	}

	// L'admin est servi depuis un autre origine (Cloudflare Pages) et
	// s'authentifie par cookie httpOnly : CORS avec credentials, et une liste
	// d'origines explicite — un wildcard est refuse par les navigateurs des
	// que AllowCredentials est actif.
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AdminOrigins,
		AllowMethods:     middleware.AllowedMethods,
		AllowHeaders:     []string{fiber.HeaderContentType, fiber.HeaderAccept, "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           int((12 * time.Hour).Seconds()),
	}))

	return app
}

// newLogger emet du JSON structure, lisible par Loki ou par les logs Coolify.
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
