//go:build integration

package repository

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/updates"
)

// La verification des versions : une nouvelle version s'enregistre et
// s'annonce une seule fois aux admins ; un « rien de nouveau » ne fait que
// dater le passage ; une demande d'admin rend la verification due.
func TestLaVerificationDesVersionsAnnonceUneFois(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL est obligatoire")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, url, DefaultPoolConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Etat de depart propre, rendu tel quel a la fin.
	_, _ = pool.Exec(ctx, `DELETE FROM app_release_check`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM app_release_check`) })

	var adminID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, firstname, lastname, role) VALUES ($1, 'x', 'Ada', 'Admin', 'admin') RETURNING id`,
		"maj-"+uuid.NewString()+"@piilot.test",
	).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, adminID) })

	var calls, notModified atomic.Int32
	version := "9.0.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		etag := `"` + version + `"`
		if r.Header.Get("If-None-Match") == etag {
			notModified.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(`{"tag_name":"v` + version + `","name":"Piilot v` + version + `","html_url":"https://example.test/r"}`))
	}))
	defer srv.Close()
	client := updates.NewWithAPI(srv.URL)
	q := db.New(pool)

	announced := func() int {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM notifications WHERE user_id = $1 AND kind = 'update_available'`, adminID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if !due(ctx, pool, time.Hour, time.Now(), log) {
		t.Fatal("jamais verifie : la verification doit etre due")
	}
	checkRelease(ctx, pool, client, "Plugiit/piilot-app", "0.9.2", log)

	check, err := q.GetReleaseCheck(ctx)
	if err != nil || check.Version != "9.0.0" || check.Etag != `"9.0.0"` {
		t.Fatalf("premier passage : %+v %v", check, err)
	}
	if announced() != 1 {
		t.Fatalf("annonces apres le premier passage : %d", announced())
	}
	if due(ctx, pool, time.Hour, time.Now(), log) {
		t.Fatal("verification juste faite : elle ne doit pas etre due")
	}

	// Rien de nouveau : 304, aucune nouvelle annonce.
	checkRelease(ctx, pool, client, "Plugiit/piilot-app", "0.9.2", log)
	if notModified.Load() != 1 || announced() != 1 {
		t.Fatalf("passage sans nouveaute : %d 304, %d annonces", notModified.Load(), announced())
	}

	// Un admin demande une verification : elle devient due tout de suite.
	if err := q.RequestReleaseCheck(ctx); err != nil {
		t.Fatal(err)
	}
	if !due(ctx, pool, time.Hour, time.Now(), log) {
		t.Fatal("verification demandee : elle doit etre due")
	}

	// Une version plus recente encore : une seconde annonce.
	version = "9.1.0"
	checkRelease(ctx, pool, client, "Plugiit/piilot-app", "0.9.2", log)
	if announced() != 2 {
		t.Fatalf("nouvelle version : %d annonces, attendu 2", announced())
	}
	if check, _ := q.GetReleaseCheck(ctx); check.CheckRequestedAt != nil {
		t.Fatal("la demande doit etre levee apres le passage")
	}

	// Une instance deja a jour n'annonce rien.
	_, _ = pool.Exec(ctx, `UPDATE app_release_check SET etag = '', notified_version = ''`)
	checkRelease(ctx, pool, client, "Plugiit/piilot-app", "9.1.0", log)
	if announced() != 2 {
		t.Fatalf("instance a jour : %d annonces, attendu 2", announced())
	}
	_ = calls.Load()
}
