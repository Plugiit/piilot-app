//go:build integration

// Tests d'integration de l'export RGPD : archive d'un compte (Discord et
// fichier lisible compris), archive d'un client (sans notes internes ni
// secrets), expiration qui supprime le fichier, taille maximale, et pas de
// double construction quand plusieurs instances reclament les memes exports.
package usecase_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

type rgpdEnv struct {
	svc    *usecase.RgpdExportService
	pool   *pgxpool.Pool
	q      *db.Queries
	files  *storage.Local
	dir    string
	prefix string
	ids    []uuid.UUID
}

func newRgpdEnv(t *testing.T, ttl time.Duration, maxBytes int64) *rgpdEnv {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL est obligatoire")
	}

	pool, err := repository.NewPool(context.Background(), url, repository.DefaultPoolConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	dir := t.TempDir()

	files, err := storage.NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}

	e := &rgpdEnv{
		pool: pool, q: db.New(pool), files: files, dir: dir, prefix: "rgpd" + uuid.NewString()[:8],
		svc: usecase.NewRgpdExportService(pool, files, usecase.NewDiscordService(pool, []byte("test-secret-test-secret-test-secret")),
			ttl, maxBytes, slog.New(slog.NewJSONHandler(io.Discard, nil))),
	}

	t.Cleanup(func() {
		ctx := context.Background()
		p := e.prefix + "%"

		_, _ = pool.Exec(ctx, `DELETE FROM rgpd_exports WHERE subject_label LIKE $1`, p)
		_, _ = pool.Exec(ctx, `DELETE FROM attachments WHERE filename LIKE $1`, p)
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE name LIKE $1`, p)
		_, _ = pool.Exec(ctx, `DELETE FROM client_interactions WHERE client_id IN (SELECT id FROM clients WHERE name LIKE $1)`, p)
		_, _ = pool.Exec(ctx, `DELETE FROM inbound_emails WHERE subject LIKE $1`, p)
		_, _ = pool.Exec(ctx, `UPDATE clients SET primary_contact_id = NULL WHERE name LIKE $1`, p)
		_, _ = pool.Exec(ctx, `DELETE FROM contacts WHERE firstname LIKE $1`, p)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email LIKE $1`, "%"+e.prefix+"%")
		_, _ = pool.Exec(ctx, `DELETE FROM clients WHERE name LIKE $1`, p)
	})

	return e
}

func (e *rgpdEnv) exec(t *testing.T, sql string, args ...any) {
	t.Helper()

	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s : %v", sql, err)
	}
}

func (e *rgpdEnv) scanID(t *testing.T, sql string, args ...any) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("%s : %v", sql, err)
	}

	return id
}

func (e *rgpdEnv) user(t *testing.T, tag, role string, clientID *uuid.UUID) uuid.UUID {
	t.Helper()

	return e.scanID(t, `INSERT INTO users (email, password_hash, firstname, lastname, role, client_id, totp_secret)
		VALUES ($1, 'SECRET-HASH-XYZ', $2, 'Test', $3, $4, 'TOTP-SECRET-XYZ') RETURNING id`,
		tag+"@"+e.prefix+".test", e.prefix+" "+tag, role, clientID)
}

// file range un fichier dans le stockage et l'enregistre comme piece jointe.
func (e *rgpdEnv) file(t *testing.T, name, content string, projectID, uploader *uuid.UUID) {
	t.Helper()

	key, size, err := e.files.Save(strings.NewReader(content), 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	e.exec(t, `INSERT INTO attachments (project_id, filename, content_type, size_bytes, storage_key, uploaded_by)
		VALUES ($1, $2, 'text/plain', $3, $4, $5)`, projectID, name, size, key, uploader)
}

func (e *rgpdEnv) buildOne(t *testing.T, subjectType string, subjectID uuid.UUID) usecase.RgpdExportItem {
	t.Helper()

	ctx := context.Background()

	item, err := e.svc.Request(ctx, nil, subjectType, subjectID)
	if err != nil {
		t.Fatalf("Request : %v", err)
	}

	for {
		worked, err := e.svc.Process(ctx)
		if err != nil {
			t.Fatalf("Process : %v", err)
		}

		if !worked {
			break
		}
	}

	got, err := e.q.GetRgpdExport(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != usecase.RgpdExportReady {
		t.Fatalf("statut %q (erreur %q), attendu ready", got.Status, got.Error)
	}

	if got.ExpiresAt == nil || got.SizeBytes <= 0 || got.FileKey == "" {
		t.Fatalf("export pret incomplet : %+v", got)
	}

	return usecase.RgpdExportItem{ID: got.ID, Status: got.Status}
}

type archive struct {
	entries map[string][]byte
}

func (e *rgpdEnv) archive(t *testing.T, id uuid.UUID) archive {
	t.Helper()

	_, rc, err := e.svc.Open(context.Background(), id)
	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	defer func() { _ = rc.Close() }()

	raw, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("zip illisible : %v", err)
	}

	a := archive{entries: map[string][]byte{}}

	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}

		b, err := io.ReadAll(r)
		_ = r.Close()

		if err != nil {
			t.Fatal(err)
		}

		a.entries[f.Name] = b
	}

	return a
}

func (a archive) rows(t *testing.T, section string) []map[string]any {
	t.Helper()

	raw, ok := a.entries["data/"+section+".json"]
	if !ok {
		t.Fatalf("section %q absente (%v)", section, a.names())
	}

	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("section %s : JSON invalide : %v\n%s", section, err, raw)
	}

	return out
}

func (a archive) names() []string {
	var n []string
	for k := range a.entries {
		n = append(n, k)
	}

	return n
}

func (a archive) mustNotContain(t *testing.T, needles ...string) {
	t.Helper()

	for name, b := range a.entries {
		for _, n := range needles {
			if bytes.Contains(b, []byte(n)) {
				t.Errorf("%s contient %q", name, n)
			}
		}
	}
}

type manifestView struct {
	Format   int `json:"format"`
	Sections []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"sections"`
	Files struct {
		Included int      `json:"included"`
		Missing  []string `json:"missing"`
	} `json:"files"`
	InternalNotes bool `json:"internal_ticket_notes_included"`
}

func (a archive) manifest(t *testing.T) manifestView {
	t.Helper()

	var m manifestView
	if err := json.Unmarshal(a.entries["manifest.json"], &m); err != nil {
		t.Fatalf("manifest : %v", err)
	}

	return m
}

func (m manifestView) count(section string) int {
	for _, s := range m.Sections {
		if s.Name == section {
			return s.Count
		}
	}

	return -1
}

func TestRgpdExport_UserArchive(t *testing.T) {
	e := newRgpdEnv(t, time.Hour, 64<<20)
	ctx := context.Background()

	client, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " client", Kind: usecase.ClientProfessionnel, Status: "actif"})
	if err != nil {
		t.Fatal(err)
	}

	project, err := e.q.CreateProject(ctx, db.CreateProjectParams{ClientID: client.ID, Name: e.prefix + " projet", Status: "production", Priority: "medium"})
	if err != nil {
		t.Fatal(err)
	}

	uid := e.user(t, "alice", "team", nil)
	other := e.user(t, "bob", "team", nil)

	if _, err := e.q.CreateTimeEntry(ctx, db.CreateTimeEntryParams{UserID: uid, ProjectID: project.ID, SpentOn: time.Now(), Minutes: 45, Note: "note-alice"}); err != nil {
		t.Fatal(err)
	}

	if _, err := e.q.CreateTimeEntry(ctx, db.CreateTimeEntryParams{UserID: other, ProjectID: project.ID, SpentOn: time.Now(), Minutes: 30, Note: "note-bob"}); err != nil {
		t.Fatal(err)
	}

	taskID := e.scanID(t, `INSERT INTO tasks (project_id, title, created_by) VALUES ($1, 'tache-alice', $2) RETURNING id`, project.ID, uid)
	e.exec(t, `INSERT INTO task_assignees (task_id, user_id) VALUES ($1, $2)`, taskID, uid)
	e.exec(t, `INSERT INTO task_comments (task_id, author_id, body) VALUES ($1, $2, 'commentaire-alice'), ($1, $3, 'commentaire-bob')`, taskID, uid, other)

	ticketID := e.scanID(t, `INSERT INTO tickets (project_id, subject) VALUES ($1, 'ticket') RETURNING id`, project.ID)
	e.exec(t, `INSERT INTO ticket_messages (ticket_id, author_id, body, is_internal) VALUES ($1, $2, 'message-alice', true), ($1, $3, 'message-bob', false)`, ticketID, uid, other)
	e.exec(t, `INSERT INTO notifications (user_id, kind, task_id) VALUES ($1, 'task_assigned', $2)`, uid, taskID)
	e.exec(t, `INSERT INTO oauth_grants (user_id, client_id, scopes) VALUES ($1, 'claude', '{read}')`, uid)
	e.exec(t, `INSERT INTO refresh_tokens (user_id, token_hash, user_agent, expires_at) VALUES ($1, '\xdeadbeefcafe', 'Firefox-alice', now() + interval '1 day')`, uid)
	e.exec(t, `INSERT INTO audit_log (actor_id, actor_email, action) VALUES ($1, $2, 'test.rgpd')`, uid, "alice@"+e.prefix+".test")
	e.exec(t, `INSERT INTO discord_links (user_id, discord_user_id) VALUES ($1, '123456789012345678')`, uid)

	e.file(t, e.prefix+"-brief.txt", "contenu lisible du fichier", &project.ID, &uid)

	item := e.buildOne(t, usecase.RgpdSubjectUser, uid)
	a := e.archive(t, item.ID)
	m := a.manifest(t)

	if m.Format != 1 {
		t.Errorf("format = %d", m.Format)
	}

	if got := a.entries["files/"+e.prefix+"-brief.txt"]; string(got) != "contenu lisible du fichier" {
		t.Errorf("fichier de l'archive : %q (%v)", got, a.names())
	}

	if m.Files.Included != 1 || len(m.Files.Missing) != 0 {
		t.Errorf("manifeste des fichiers : %+v", m.Files)
	}

	// Discord : la liaison, par DiscordService.Export.
	if d := a.rows(t, "discord"); len(d) != 1 || d[0]["discord_user_id"] != "123456789012345678" {
		t.Errorf("section discord : %v", d)
	}

	for section, want := range map[string]int{
		"profile": 1, "sessions": 1, "time_entries": 1, "tasks": 1, "task_comments": 1, "ticket_messages": 1,
		"notifications": 1, "claude_connections": 1, "audit": 1, "attachments": 1, "discord": 1,
	} {
		if got := m.count(section); got != want {
			t.Errorf("manifeste : %s = %d, attendu %d", section, got, want)
		}

		if got := len(a.rows(t, section)); got != want {
			t.Errorf("section %s : %d lignes, attendu %d", section, got, want)
		}
	}

	if p := a.rows(t, "profile")[0]; p["email"] != "alice@"+e.prefix+".test" {
		t.Errorf("profil : %v", p)
	}

	// Les donnees des autres ne sortent pas, les secrets non plus.
	a.mustNotContain(t, "note-bob", "commentaire-bob", "message-bob", "SECRET-HASH-XYZ", "TOTP-SECRET-XYZ", "deadbeef", "3q2+78r+", "password_hash", "totp_secret", "storage_key")

	// Le journal ne se confond pas : un export est demande une fois, retrouve ensuite.
	again, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
	if err != nil || again.Status != usecase.RgpdExportQueued {
		t.Fatalf("nouvelle demande : %+v (%v)", again, err)
	}

	same, _ := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
	if same.ID != again.ID {
		t.Errorf("deux demandes en attente ont cree deux exports")
	}
}

func TestRgpdExport_ClientArchive(t *testing.T) {
	e := newRgpdEnv(t, time.Hour, 64<<20)
	ctx := context.Background()

	client, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " Dupont SA", Kind: usecase.ClientProfessionnel, Status: "actif", Phone: "0102030405"})
	if err != nil {
		t.Fatal(err)
	}

	empty, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " vide", Kind: usecase.ClientProfessionnel, Status: "lead"})
	if err != nil {
		t.Fatal(err)
	}

	email := "contact@" + e.prefix + ".test"
	if _, err := e.q.CreateContact(ctx, db.CreateContactParams{ClientID: &client.ID, Firstname: e.prefix + " Jean", Lastname: "Dupont", Email: &email}); err != nil {
		t.Fatal(err)
	}

	project, err := e.q.CreateProject(ctx, db.CreateProjectParams{ClientID: client.ID, Name: e.prefix + " projet", Status: "production", Priority: "high"})
	if err != nil {
		t.Fatal(err)
	}

	team := e.user(t, "equipe", "team", nil)
	e.user(t, "portail", "client", &client.ID)

	if _, err := e.q.CreateTimeEntry(ctx, db.CreateTimeEntryParams{UserID: team, ProjectID: project.ID, SpentOn: time.Now(), Minutes: 60, Note: "temps-client"}); err != nil {
		t.Fatal(err)
	}

	e.exec(t, `INSERT INTO milestones (project_id, title) VALUES ($1, 'jalon-1')`, project.ID)
	deliverable := e.scanID(t, `INSERT INTO deliverables (project_id, title) VALUES ($1, 'livrable-1') RETURNING id`, project.ID)
	e.exec(t, `INSERT INTO deliverable_versions (deliverable_id, numero, url) VALUES ($1, 1, 'https://exemple.fr/v1')`, deliverable)

	ticket := e.scanID(t, `INSERT INTO tickets (project_id, subject, requester_email, requester_name) VALUES ($1, 'ticket-client', $2, 'Jean') RETURNING id`, project.ID, email)
	e.exec(t, `INSERT INTO ticket_messages (ticket_id, body, is_internal) VALUES ($1, 'message-public', false), ($1, 'NOTE-INTERNE-SECRETE', true)`, ticket)
	e.exec(t, `INSERT INTO client_interactions (client_id, kind, body) VALUES ($1, 'note', 'echange-telephone')`, client.ID)
	e.exec(t, `INSERT INTO inbound_emails (source, raw, from_address, subject, client_id) VALUES ('raw', 'CORPS-BRUT-SECRET', $1, $2, $3)`, email, e.prefix+" sujet", client.ID)
	e.exec(t, `INSERT INTO reports (project_id, month, plan_name, included_minutes, sections, payload) VALUES ($1, '2026-01-01', 'Plan', 60, '{a}', '{"k":"rapport-client"}')`, project.ID)

	e.file(t, e.prefix+"-maquette.txt", "maquette", &project.ID, &team)

	item := e.buildOne(t, usecase.RgpdSubjectClient, client.ID)
	a := e.archive(t, item.ID)
	m := a.manifest(t)

	for section, want := range map[string]int{
		"client": 1, "contacts": 1, "portal_users": 1, "projects": 1, "milestones": 1, "deliverables": 1,
		"deliverable_versions": 1, "tickets": 1, "ticket_messages": 1, "interactions": 1, "inbound_emails": 1,
		"reports": 1, "time_entries": 1, "attachments": 1,
	} {
		if got := len(a.rows(t, section)); got != want {
			t.Errorf("section %s : %d lignes, attendu %d", section, got, want)
		}

		if got := m.count(section); got != want {
			t.Errorf("manifeste : %s = %d, attendu %d", section, got, want)
		}
	}

	if got := string(a.entries["files/"+e.prefix+"-maquette.txt"]); got != "maquette" {
		t.Errorf("fichier : %q", got)
	}

	// Une seule decision, un seul point : les notes internes sont exclues.
	if m.InternalNotes {
		t.Error("le manifeste annonce des notes internes")
	}

	if msgs := a.rows(t, "ticket_messages"); len(msgs) != 1 || msgs[0]["body"] != "message-public" {
		t.Errorf("messages : %v", msgs)
	}

	a.mustNotContain(t, "NOTE-INTERNE-SECRETE", "CORPS-BRUT-SECRET", "SECRET-HASH-XYZ", "TOTP-SECRET-XYZ", "password_hash", "storage_key")

	// Un client sans rien : l'archive existe, ses sections sont vides.
	vide := e.archive(t, e.buildOne(t, usecase.RgpdSubjectClient, empty.ID).ID)
	if len(vide.rows(t, "projects")) != 0 || len(vide.rows(t, "client")) != 1 {
		t.Errorf("client vide : projets=%d fiche=%d", len(vide.rows(t, "projects")), len(vide.rows(t, "client")))
	}
}

func TestRgpdExport_UnknownSubjectAndDownloadStates(t *testing.T) {
	e := newRgpdEnv(t, time.Hour, 64<<20)
	ctx := context.Background()

	if _, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uuid.New()); !errors.Is(err, domain.ErrNotFound) && !isDomain(err, domain.ErrNotFound) {
		t.Errorf("sujet inconnu : %v", err)
	}

	if _, err := e.svc.Request(ctx, nil, "autre", uuid.New()); !isValidation(err, "subject_type") {
		t.Errorf("type inconnu : %v", err)
	}

	uid := e.user(t, "carol", "team", nil)

	item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
	if err != nil {
		t.Fatal(err)
	}

	// Pas telechargeable tant qu'il n'est pas pret.
	if _, _, err := e.svc.Open(ctx, item.ID); !isDomain(err, domain.ErrConflict) {
		t.Errorf("export en file : %v", err)
	}
}

func isDomain(err error, want *domain.Error) bool {
	var got *domain.Error

	return errors.As(err, &got) && got.Status == want.Status
}

func TestRgpdExport_ExpiryDeletesTheFile(t *testing.T) {
	e := newRgpdEnv(t, time.Hour, 64<<20)
	ctx := context.Background()

	uid := e.user(t, "dave", "team", nil)
	item := e.buildOne(t, usecase.RgpdSubjectUser, uid)

	row, _ := e.q.GetRgpdExport(ctx, item.ID)

	if _, err := e.files.Open(row.FileKey); err != nil {
		t.Fatalf("l'archive devrait exister : %v", err)
	}

	// Pas encore echu : rien ne part.
	if n, err := e.svc.Purge(ctx); err != nil || n != 0 {
		t.Fatalf("purge avant echeance : %d (%v)", n, err)
	}

	e.exec(t, `UPDATE rgpd_exports SET expires_at = now() - interval '1 minute' WHERE id = $1`, item.ID)

	// Echu mais pas encore purge : le telechargement est deja refuse.
	if _, _, err := e.svc.Open(ctx, item.ID); !isDomain(err, domain.ErrConflict) {
		t.Errorf("archive echue : %v", err)
	}

	if n, err := e.svc.Purge(ctx); err != nil || n < 1 {
		t.Fatalf("purge : %d (%v)", n, err)
	}

	if _, err := e.files.Open(row.FileKey); !os.IsNotExist(err) {
		t.Errorf("le fichier doit avoir ete supprime : %v", err)
	}

	got, _ := e.q.GetRgpdExport(ctx, item.ID)
	if got.Status != usecase.RgpdExportExpired || got.FileKey != "" {
		t.Errorf("apres purge : %s / %q", got.Status, got.FileKey)
	}

	// Un export « en cours » abandonne est ecarte par la meme tache.
	stale, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
	if err != nil {
		t.Fatal(err)
	}

	e.exec(t, `UPDATE rgpd_exports SET status = 'running', updated_at = now() - interval '2 hours' WHERE id = $1`, stale.ID)

	if _, err := e.svc.Purge(ctx); err != nil {
		t.Fatal(err)
	}

	if got, _ := e.q.GetRgpdExport(ctx, stale.ID); got.Status != usecase.RgpdExportFailed || got.Error != "interrupted" {
		t.Errorf("export abandonne : %s / %q", got.Status, got.Error)
	}
}

func dirCount(t *testing.T, dir string) int {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	return len(entries)
}

func TestRgpdExport_TooLargeFailsAndLeavesNoFile(t *testing.T) {
	e := newRgpdEnv(t, time.Hour, 200) // moins qu'un manifeste

	uid := e.user(t, "erin", "team", nil)

	item, err := e.svc.Request(context.Background(), nil, usecase.RgpdSubjectUser, uid)
	if err != nil {
		t.Fatal(err)
	}

	if worked, err := e.svc.Process(context.Background()); err != nil || !worked {
		t.Fatalf("Process : %v %v", worked, err)
	}

	got, _ := e.q.GetRgpdExport(context.Background(), item.ID)
	if got.Status != usecase.RgpdExportFailed || got.Error != "too_large" || got.FileKey != "" {
		t.Errorf("export trop gros : %s / %q / %q", got.Status, got.Error, got.FileKey)
	}

	if n := dirCount(t, e.dir); n != 0 {
		t.Errorf("%d fichiers restent apres l'echec", n)
	}
}

// Si le sujet est efface (ou la construction jugee abandonnee) pendant que
// l'archive se construit, le resultat ne s'ecrit pas et l'archive est retiree
// du disque : elle contient des donnees personnelles que plus rien ne reference.
func TestRgpdExport_NoOrphanArchiveWhenExportChangesDuringBuild(t *testing.T) {
	for name, newStatus := range map[string]string{"expire": "expired", "echec": "failed"} {
		t.Run(name, func(t *testing.T) {
			e := newRgpdEnv(t, time.Hour, 64<<20)
			ctx := context.Background()

			uid := e.user(t, "orph-"+name, "team", nil)

			item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
			if err != nil {
				t.Fatal(err)
			}

			e.svc.AfterBuild(func() {
				e.exec(t, `UPDATE rgpd_exports SET status = $2, file_key = '' WHERE id = $1`, item.ID, newStatus)
			})

			if worked, err := e.svc.Process(ctx); err != nil || !worked {
				t.Fatalf("Process : %v %v", worked, err)
			}

			got, _ := e.q.GetRgpdExport(ctx, item.ID)
			if got.Status != newStatus || got.FileKey != "" {
				t.Errorf("l'export a ete modifie : %s / %q", got.Status, got.FileKey)
			}

			if n := dirCount(t, e.dir); n != 0 {
				t.Errorf("%d archive(s) orpheline(s) sur disque", n)
			}
		})
	}
}

// Des demandes simultanees pour le meme sujet n'en font qu'un export.
func TestRgpdExport_ConcurrentRequestsCreateOneExport(t *testing.T) {
	e := newRgpdEnv(t, time.Hour, 64<<20)
	uid := e.user(t, "concurrent", "team", nil)

	const callers = 12

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
		ids   = make([]uuid.UUID, callers)
	)

	for i := range callers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			item, err := e.svc.Request(context.Background(), nil, usecase.RgpdSubjectUser, uid)
			if err != nil {
				t.Errorf("Request : %v", err)

				return
			}

			ids[i] = item.ID
		}()
	}

	close(start)
	wg.Wait()

	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("des identifiants differents : %v", ids)
		}
	}

	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM rgpd_exports WHERE subject_id = $1`, uid).Scan(&n); err != nil || n != 1 {
		t.Errorf("%d exports pour une rafale de demandes (%v)", n, err)
	}
}

func TestRgpdExport_TwoWorkersNeverBuildTheSameExportTwice(t *testing.T) {
	e := newRgpdEnv(t, time.Hour, 64<<20)
	ctx := context.Background()

	const exports = 4

	var want []uuid.UUID

	for i := range exports {
		uid := e.user(t, "w"+string(rune('a'+i)), "team", nil)

		item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
		if err != nil {
			t.Fatal(err)
		}

		want = append(want, item.ID)
	}

	var built atomic.Int32

	var wg sync.WaitGroup

	start := make(chan struct{})

	// Six « instances » sur le meme service, comme deux processus : la
	// reservation se joue en base, pas en memoire.
	for range 6 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			for {
				worked, err := e.svc.Process(ctx)
				if err != nil {
					t.Error(err)

					return
				}

				if !worked {
					return
				}

				built.Add(1)
			}
		}()
	}

	close(start)
	wg.Wait()

	if built.Load() != exports {
		t.Fatalf("%d constructions pour %d exports", built.Load(), exports)
	}

	if n := dirCount(t, e.dir); n != exports {
		t.Errorf("%d archives sur disque, attendu %d", n, exports)
	}

	keys := map[string]bool{}

	for _, id := range want {
		got, _ := e.q.GetRgpdExport(ctx, id)
		if got.Status != usecase.RgpdExportReady || got.FileKey == "" || keys[got.FileKey] {
			t.Errorf("export %s : %s / %q", id, got.Status, got.FileKey)
		}

		keys[got.FileKey] = true
	}
}
