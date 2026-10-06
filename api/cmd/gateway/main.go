// Commande gateway : la passerelle placee devant l'application.
//
// Elle tient le port public et aiguille chaque requete vers une instance saine
// de l'application, ce qui permet a l'updater de remplacer celle-ci sans
// coupure. Voir internal/gateway.
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

	"github.com/plugiit/piilot-app/api/internal/gateway"
)

// version est injectee au build (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(log); err != nil {
		log.Error("arret de la passerelle", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g := gateway.New(gateway.Config{
		Upstream:      env("UPSTREAM", "server:8080"),
		ProbeInterval: duration("PROBE_INTERVAL", time.Second),
		Hold:          duration("HOLD_TIMEOUT", 15*time.Second),
	}, log)
	go g.Run(ctx)

	srv := &http.Server{
		Addr:              env("LISTEN_ADDR", ":8080"),
		Handler:           g,
		ReadHeaderTimeout: 10 * time.Second,
		// Pas de delai de lecture ni d'ecriture global : les pieces jointes
		// et le flux des notifications durent, et l'application borne deja
		// les siens.
		IdleTimeout: 2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("passerelle demarree", "addr", srv.Addr, "upstream", env("UPSTREAM", "server:8080"), "version", version)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return srv.Shutdown(shutdown)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(key)); err == nil && d > 0 {
		return d
	}
	return fallback
}
