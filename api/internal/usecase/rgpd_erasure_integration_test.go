//go:build integration

// Tests d'integration de l'effacement RGPD : demande puis annulation, compte
// anonymise a l'echeance, client supprime avec ses fichiers, purge des
// inactifs (non-admins, une connexion annule), retour arriere complet, et pas
// de double execution entre deux instances.
package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

type erasureEnv struct {
	*rgpdEnv
	svc     *usecase.RgpdErasureService
	started time.Time
	users   []uuid.UUID
}

func newErasureEnv(t *testing.T, inactiveAfter time.Duration) *erasureEnv {
	t.Helper()

	e := &erasureEnv{rgpdEnv: newRgpdEnv(t, time.Hour, 64<<20), started: time.Now().Add(-time.Second)}
	e.svc = usecase.NewRgpdErasureService(e.pool, e.files, usecase.NewDiscordService(e.pool, []byte("test-secret-test-secret-test-secret")),
		720*time.Hour, inactiveAfter, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	t.Cleanup(func() {
		ctx := context.Background()

		_, _ = e.pool.Exec(ctx, `DELETE FROM rgpd_erasures WHERE created_at >= $1`, e.started)
		_, _ = e.pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, e.users)
	})

	return e
}

// user cree un compte et le retient pour le nettoyage (son adresse change a l'effacement).
func (e *erasureEnv) member(t *testing.T, tag, role string, clientID *uuid.UUID) uuid.UUID {
	t.Helper()

	id := e.user(t, tag, role, clientID)
	e.users = append(e.users, id)

	return id
}

func (e *erasureEnv) makeDue(t *testing.T, id uuid.UUID) {
	t.Helper()
	e.exec(t, `UPDATE rgpd_erasures SET scheduled_for = now() - interval '1 minute' WHERE id = $1`, id)
}

// drain execute tous les effacements arrives a echeance.
func (e *erasureEnv) drain(t *testing.T) int {
	t.Helper()

	n := 0

	for i := 0; i < 50; i++ {
		worked, err := e.svc.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute : %v", err)
		}

		if !worked {
			return n
		}

		n++
	}

	t.Fatal("Execute ne s'arrete pas")

	return n
}

func (e *erasureEnv) erasure(t *testing.T, id uuid.UUID) db.RgpdErasure {
	t.Helper()

	var er db.RgpdErasure

	row := e.pool.QueryRow(context.Background(), `SELECT id, status, error, subject_label, summary, executed_at FROM rgpd_erasures WHERE id = $1`, id)
	if err := row.Scan(&er.ID, &er.Status, &er.Error, &er.SubjectLabel, &er.Summary, &er.ExecutedAt); err != nil {
		t.Fatal(err)
	}

	return er
}

func (e *erasureEnv) count(t *testing.T, sql string, args ...any) int {
	t.Helper()

	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s : %v", sql, err)
	}

	return n
}

func (e *erasureEnv) fileExists(t *testing.T, key string) bool {
	t.Helper()

	rc, err := e.files.Open(key)
	if err != nil {
		return false
	}

	_ = rc.Close()

	return true
}

func (e *erasureEnv) save(t *testing.T, content string) string {
	t.Helper()

	key, _, err := e.files.Save(strings.NewReader(content), 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	return key
}

func discordID() string { return fmt.Sprintf("%d%017d", 1+rand.IntN(8), rand.Int64N(1e17)) }

// richUser cree un compte avec ce qu'un effacement doit retrouver partout.
func (e *erasureEnv) richUser(t *testing.T, tag string) (uuid.UUID, string, string) {
	t.Helper()

	ctx := context.Background()
	uid := e.member(t, tag, "team", nil)
	email := tag + "@" + e.prefix + ".test"
	did := discordID()

	client, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " c-" + tag, Kind: usecase.ClientProfessionnel, Status: "actif"})
	if err != nil {
		t.Fatal(err)
	}

	project, err := e.q.CreateProject(ctx, db.CreateProjectParams{ClientID: client.ID, Name: e.prefix + " p-" + tag, Status: "production", Priority: "medium"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := e.q.CreateTimeEntry(ctx, db.CreateTimeEntryParams{UserID: uid, ProjectID: project.ID, SpentOn: time.Now(), Minutes: 45, Note: "note"}); err != nil {
		t.Fatal(err)
	}

	avatarKey := e.save(t, "avatar")
	e.exec(t, `UPDATE users SET avatar_url = $2, phone = '0600000000', city = 'Nantes' WHERE id = $1`, uid, "/api/v1/auth/avatars/"+avatarKey)
	e.exec(t, `INSERT INTO discord_links (user_id, discord_user_id) VALUES ($1, $2)`, uid, did)
	e.exec(t, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 day')`, uid, []byte(uuid.NewString()))
	e.exec(t, `INSERT INTO oauth_grants (user_id, client_id, scopes) VALUES ($1, 'claude', '{read}')`, uid)
	e.exec(t, `INSERT INTO email_outbox (kind, to_address, subject, text_body, html_body) VALUES ('test', $1, 'sujet', 'corps', 'corps')`, email)

	ticket := e.scanID(t, `INSERT INTO tickets (project_id, subject, requester_email, requester_name) VALUES ($1, 'ticket', $2, 'Nom') RETURNING id`, project.ID, email)
	e.exec(t, `INSERT INTO ticket_messages (ticket_id, body, sender_name, sender_email) VALUES ($1, 'message', 'Nom', $2)`, ticket, email)

	return uid, did, avatarKey
}

func TestRgpdErasure_RequestThenCancel(t *testing.T) {
	e := newErasureEnv(t, 0)
	ctx := context.Background()

	uid, did, avatarKey := e.richUser(t, "alice")
	disabledBefore := e.member(t, "deja-desactive", "team", nil)
	e.exec(t, `UPDATE users SET disabled_at = now() - interval '2 days' WHERE id = $1`, disabledBefore)

	item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
	if err != nil {
		t.Fatalf("Request : %v", err)
	}

	if item.Status != usecase.RgpdErasureScheduled || item.Reason != usecase.RgpdReasonRequest {
		t.Fatalf("demande : %+v", item)
	}

	if d := time.Until(item.ScheduledFor); d < 719*time.Hour || d > 721*time.Hour {
		t.Errorf("echeance dans %s, attendu environ 720h", d)
	}

	if e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND disabled_at IS NOT NULL`, uid) != 1 {
		t.Error("le compte n'est pas desactive tout de suite")
	}

	if e.count(t, `SELECT count(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, uid) != 0 {
		t.Error("une session reste ouverte")
	}

	// Une seconde demande pour le meme sujet est refusee.
	if _, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("seconde demande : %v, attendu un conflit", err)
	}

	// Pas encore a echeance : rien ne part.
	e.drain(t)

	if got := e.erasure(t, item.ID); got.Status != usecase.RgpdErasureScheduled {
		t.Fatalf("effacement execute avant l'echeance : %s", got.Status)
	}

	cancelled, err := e.svc.Cancel(ctx, nil, item.ID)
	if err != nil {
		t.Fatalf("Cancel : %v", err)
	}

	if cancelled.Status != usecase.RgpdErasureCancelled || cancelled.CancelledAt == nil {
		t.Fatalf("annulation : %+v", cancelled)
	}

	if e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND disabled_at IS NULL AND deleted_at IS NULL AND city = 'Nantes'`, uid) != 1 {
		t.Error("le compte n'est pas reactive tel quel")
	}

	// Rien n'a ete efface.
	if e.count(t, `SELECT count(*) FROM discord_links WHERE discord_user_id = $1`, did) != 1 || !e.fileExists(t, avatarKey) {
		t.Error("la liaison Discord ou l'avatar a disparu")
	}

	if _, err := e.svc.Cancel(ctx, nil, item.ID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("seconde annulation : %v, attendu un conflit", err)
	}

	// Un compte deja desactive le reste apres l'annulation.
	other, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, disabledBefore)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := e.svc.Cancel(ctx, nil, other.ID); err != nil {
		t.Fatal(err)
	}

	if e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND disabled_at IS NOT NULL`, disabledBefore) != 1 {
		t.Error("un compte deja desactive a ete reactive")
	}

	// Un sujet inconnu est refuse.
	if _, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("sujet inconnu : %v", err)
	}
}

func TestRgpdErasure_UserAnonymizedAtDueDate(t *testing.T) {
	e := newErasureEnv(t, 0)
	ctx := context.Background()

	uid, did, avatarKey := e.richUser(t, "bruno")
	email := "bruno@" + e.prefix + ".test"

	exportKey := e.save(t, "archive")
	exportID := e.scanID(t, `INSERT INTO rgpd_exports (subject_type, subject_id, subject_label, status, file_key, size_bytes) VALUES ('user', $1, $2, 'ready', $3, 7) RETURNING id`,
		uid, e.prefix+" Bruno", exportKey)

	item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
	if err != nil {
		t.Fatal(err)
	}

	// La desactivation de la demande a deja retire les roles ; on compte ce
	// que l'execution ajoute.
	eventsBefore := e.count(t, `SELECT count(*) FROM bot_events WHERE type = $1 AND data->>'discord_user_id' = $2`, usecase.BotEventDiscordLinkRemoved, did)

	e.makeDue(t, item.ID)
	e.drain(t)

	got := e.erasure(t, item.ID)
	if got.Status != usecase.RgpdErasureDone || got.ExecutedAt == nil || got.SubjectLabel != "Supprimé" {
		t.Fatalf("effacement : %+v", got)
	}

	var (
		mail, first, phone, city, hash string
		deleted                        *time.Time
		avatar, totp                   *string
	)

	if err := e.pool.QueryRow(ctx, `SELECT email, firstname, phone, city, password_hash, deleted_at, avatar_url, totp_secret FROM users WHERE id = $1`, uid).
		Scan(&mail, &first, &phone, &city, &hash, &deleted, &avatar, &totp); err != nil {
		t.Fatal(err)
	}

	if mail != "supprime-"+uid.String()+"@invalid" || first != "" || phone != "" || city != "" || deleted == nil || avatar != nil || totp != nil || hash == "SECRET-HASH-XYZ" {
		t.Errorf("compte mal anonymise : %q %q %q %q %v %v %v", mail, first, phone, city, deleted, avatar, totp)
	}

	if security.VerifyPassword(hash, "SECRET-HASH-XYZ") || security.VerifyPassword(hash, "") {
		t.Error("le mot de passe reste utilisable")
	}

	if e.count(t, `SELECT count(*) FROM discord_links WHERE user_id = $1`, uid) != 0 {
		t.Error("discord_links n'est pas vide")
	}

	if e.count(t, `SELECT count(*) FROM bot_events WHERE type = $1 AND data->>'discord_user_id' = $2`, usecase.BotEventDiscordLinkRemoved, did) != eventsBefore+1 {
		t.Error("pas d'evenement de retrait dans bot_events")
	}

	if e.count(t, `SELECT coalesce(sum(minutes), 0) FROM time_entries WHERE user_id = $1`, uid) != 45 {
		t.Error("le temps saisi n'est pas conserve")
	}

	for name, n := range map[string]int{
		"sessions":         e.count(t, `SELECT count(*) FROM refresh_tokens WHERE user_id = $1`, uid),
		"jetons oauth":     e.count(t, `SELECT count(*) FROM oauth_grants WHERE user_id = $1`, uid),
		"e-mails":          e.count(t, `SELECT count(*) FROM email_outbox WHERE to_address = $1`, email),
		"demandeur":        e.count(t, `SELECT count(*) FROM tickets WHERE requester_email IS NOT NULL AND requester_email = $1`, email),
		"messages":         e.count(t, `SELECT count(*) FROM ticket_messages WHERE sender_email = $1 OR sender_name <> ''`, email),
		"nom du demandeur": e.count(t, `SELECT count(*) FROM tickets t WHERE requester_name = 'Nom' AND t.project_id IN (SELECT id FROM projects WHERE name LIKE $1)`, e.prefix+" p-%"),
	} {
		if n != 0 {
			t.Errorf("%s : %d ligne(s) restante(s)", name, n)
		}
	}

	if e.fileExists(t, avatarKey) || e.fileExists(t, exportKey) {
		t.Error("un fichier (avatar ou archive d'export) reste sur le disque")
	}

	if e.count(t, `SELECT count(*) FROM rgpd_exports WHERE id = $1 AND file_key = '' AND subject_label = 'Supprimé' AND status = 'expired'`, exportID) != 1 {
		t.Error("l'historique des exports garde le libelle ou le fichier")
	}

	var details []byte
	if err := e.pool.QueryRow(ctx, `SELECT details FROM audit_log WHERE action = 'rgpd.erasure_executed' AND target_id = $1`, item.ID.String()).Scan(&details); err != nil {
		t.Fatalf("ligne d'audit absente : %v", err)
	}

	for _, secret := range []string{"bruno", e.prefix, "Nantes", email} {
		if strings.Contains(string(details), secret) {
			t.Errorf("l'audit contient %q : %s", secret, details)
		}
	}

	var parsed struct {
		SubjectID uuid.UUID        `json:"subject_id"`
		Counts    map[string]int64 `json:"counts"`
	}

	if err := json.Unmarshal(details, &parsed); err != nil || parsed.SubjectID != uid || parsed.Counts["time_entries_kept"] != 1 {
		t.Errorf("details de l'audit : %s (%v)", details, err)
	}
}

func TestRgpdErasure_ClientDeletedWithFiles(t *testing.T) {
	e := newErasureEnv(t, 0)
	ctx := context.Background()

	client, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " Dupont SA", Kind: usecase.ClientProfessionnel, Status: "actif"})
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

	team := e.member(t, "equipe", "team", nil)
	portal := e.member(t, "portail", "client", &client.ID)
	portalEmail := "portail@" + e.prefix + ".test"

	if _, err := e.q.CreateTimeEntry(ctx, db.CreateTimeEntryParams{UserID: team, ProjectID: project.ID, SpentOn: time.Now(), Minutes: 60, Note: "t"}); err != nil {
		t.Fatal(err)
	}

	projectFile := e.save(t, "fichier projet")
	taskFile := e.save(t, "fichier tache")
	ticketFile := e.save(t, "fichier ticket")
	logo := e.save(t, "logo")

	task := e.scanID(t, `INSERT INTO tasks (project_id, title) VALUES ($1, 'tache') RETURNING id`, project.ID)
	ticket := e.scanID(t, `INSERT INTO tickets (project_id, subject, requester_email) VALUES ($1, 'ticket', $2) RETURNING id`, project.ID, email)
	e.exec(t, `INSERT INTO attachments (project_id, filename, content_type, size_bytes, storage_key) VALUES ($1, $2, 'text/plain', 1, $3)`, project.ID, e.prefix+"-p", projectFile)
	e.exec(t, `INSERT INTO attachments (task_id, filename, content_type, size_bytes, storage_key) VALUES ($1, $2, 'text/plain', 1, $3)`, task, e.prefix+"-t", taskFile)
	e.exec(t, `INSERT INTO attachments (ticket_id, filename, content_type, size_bytes, storage_key) VALUES ($1, $2, 'text/plain', 1, $3)`, ticket, e.prefix+"-k", ticketFile)
	e.exec(t, `UPDATE projects SET logo_key = $2 WHERE id = $1`, project.ID, logo)
	e.exec(t, `INSERT INTO inbound_emails (source, raw, from_address, subject, client_id) VALUES ('raw', 'brut', $1, $2, $3)`, email, e.prefix+" sujet", client.ID)
	e.exec(t, `INSERT INTO email_outbox (kind, to_address, subject, text_body, html_body) VALUES ('test', $1, 's', 'c', 'c')`, portalEmail)
	e.exec(t, `INSERT INTO client_interactions (client_id, kind, body) VALUES ($1, 'note', 'echange')`, client.ID)

	item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectClient, client.ID)
	if err != nil {
		t.Fatal(err)
	}

	if e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND disabled_at IS NOT NULL`, portal) != 1 {
		t.Error("le compte portail n'est pas desactive")
	}

	e.makeDue(t, item.ID)
	e.drain(t)

	if got := e.erasure(t, item.ID); got.Status != usecase.RgpdErasureDone {
		t.Fatalf("effacement : %+v", got)
	}

	for name, n := range map[string]int{
		"client":    e.count(t, `SELECT count(*) FROM clients WHERE id = $1`, client.ID),
		"contacts":  e.count(t, `SELECT count(*) FROM contacts WHERE client_id = $1`, client.ID),
		"projets":   e.count(t, `SELECT count(*) FROM projects WHERE client_id = $1`, client.ID),
		"temps":     e.count(t, `SELECT count(*) FROM time_entries WHERE project_id = $1`, project.ID),
		"tickets":   e.count(t, `SELECT count(*) FROM tickets WHERE project_id = $1`, project.ID),
		"fichiers":  e.count(t, `SELECT count(*) FROM attachments WHERE filename LIKE $1`, e.prefix+"%"),
		"e-mails":   e.count(t, `SELECT count(*) FROM inbound_emails WHERE subject LIKE $1`, e.prefix+"%"),
		"sortants":  e.count(t, `SELECT count(*) FROM email_outbox WHERE to_address = $1`, portalEmail),
		"echanges":  e.count(t, `SELECT count(*) FROM client_interactions WHERE client_id = $1`, client.ID),
		"compte ok": e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND (client_id IS NOT NULL OR deleted_at IS NULL OR email = $2)`, portal, portalEmail),
	} {
		if n != 0 {
			t.Errorf("%s : %d ligne(s) restante(s)", name, n)
		}
	}

	for name, key := range map[string]string{"projet": projectFile, "tache": taskFile, "ticket": ticketFile, "logo": logo} {
		if e.fileExists(t, key) {
			t.Errorf("le fichier %s reste sur le disque", name)
		}
	}

	// Le compte de l'equipe et son identite ne sont pas touches.
	if e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND deleted_at IS NULL AND disabled_at IS NULL`, team) != 1 {
		t.Error("le compte de l'equipe a ete modifie")
	}
}

// Un client supprime dans le CRM (suppression logique) garde ses donnees : il
// doit rester retrouvable, exportable et effacable, et un client supprime
// pendant le delai de retractation doit s'effacer quand meme.
func TestRgpdErasure_SoftDeletedClient(t *testing.T) {
	e := newErasureEnv(t, 0)
	ctx := context.Background()

	mk := func(name string) uuid.UUID {
		client, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " " + name, Kind: usecase.ClientProfessionnel, Status: "actif"})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := e.q.CreateProject(ctx, db.CreateProjectParams{ClientID: client.ID, Name: e.prefix + " projet " + name, Status: "production", Priority: "high"}); err != nil {
			t.Fatal(err)
		}

		return client.ID
	}

	t.Run("supprime avant la demande", func(t *testing.T) {
		id := mk("deja-supprime")
		e.exec(t, `UPDATE clients SET deleted_at = now() WHERE id = $1`, id)

		page, err := e.svc.DeletedClients(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}

		found := false

		for _, c := range page.Items {
			found = found || c.ID == id
		}

		if !found {
			t.Error("le client supprime n'est pas dans la liste des clients supprimes")
		}

		// Exportable (l'export est celui de l'ecran RGPD).
		exporter := newRgpdEnv(t, time.Hour, 64<<20)
		exporter.buildOne(t, usecase.RgpdSubjectClient, id)

		item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectClient, id)
		if err != nil {
			t.Fatalf("demande d'effacement d'un client supprime : %v", err)
		}

		e.makeDue(t, item.ID)
		e.drain(t)

		if got := e.erasure(t, item.ID); got.Status != usecase.RgpdErasureDone {
			t.Fatalf("effacement : %+v", got)
		}

		if e.count(t, `SELECT count(*) FROM clients WHERE id = $1`, id)+e.count(t, `SELECT count(*) FROM projects WHERE client_id = $1`, id) != 0 {
			t.Error("les donnees du client supprime sont conservees")
		}
	})

	t.Run("supprime pendant le delai", func(t *testing.T) {
		id := mk("supprime-pendant")

		item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectClient, id)
		if err != nil {
			t.Fatal(err)
		}

		e.exec(t, `UPDATE clients SET deleted_at = now() WHERE id = $1`, id)
		e.makeDue(t, item.ID)
		e.drain(t)

		if got := e.erasure(t, item.ID); got.Status != usecase.RgpdErasureDone {
			t.Fatalf("effacement : statut %q, erreur %q", got.Status, got.Error)
		}

		if e.count(t, `SELECT count(*) FROM clients WHERE id = $1`, id) != 0 {
			t.Error("le client supprime pendant le delai est toujours la")
		}
	})
}

// Annuler un effacement ne reactive pas un compte que l'effacement de son
// client (ou le sien) vise encore ; une fois le dernier annule, il retrouve
// son etat d'origine.
func TestRgpdErasure_CancelKeepsAccountDisabledWhileAnotherErasureTargetsIt(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) (*erasureEnv, uuid.UUID, uuid.UUID) {
		e := newErasureEnv(t, 0)

		client, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " Double", Kind: usecase.ClientProfessionnel, Status: "actif"})
		if err != nil {
			t.Fatal(err)
		}

		return e, client.ID, e.member(t, "portail-double", "client", &client.ID)
	}

	disabled := func(t *testing.T, e *erasureEnv, id uuid.UUID) bool {
		return e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND disabled_at IS NOT NULL`, id) == 1
	}

	t.Run("compte puis client, annulation du compte", func(t *testing.T) {
		e, clientID, user := setup(t)

		byUser, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, user)
		if err != nil {
			t.Fatal(err)
		}

		byClient, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectClient, clientID)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := e.svc.Cancel(ctx, nil, byUser.ID); err != nil {
			t.Fatal(err)
		}

		if !disabled(t, e, user) {
			t.Fatal("le compte a ete reactive alors que l'effacement de son client est planifie")
		}

		if _, err := e.svc.Cancel(ctx, nil, byClient.ID); err != nil {
			t.Fatal(err)
		}

		if disabled(t, e, user) {
			t.Error("le compte reste desactive apres l'annulation du dernier effacement")
		}
	})

	t.Run("client puis compte, annulation du client", func(t *testing.T) {
		e, clientID, user := setup(t)

		byClient, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectClient, clientID)
		if err != nil {
			t.Fatal(err)
		}

		byUser, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, user)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := e.svc.Cancel(ctx, nil, byClient.ID); err != nil {
			t.Fatal(err)
		}

		if !disabled(t, e, user) {
			t.Fatal("le compte a ete reactive alors que son propre effacement est planifie")
		}

		if _, err := e.svc.Cancel(ctx, nil, byUser.ID); err != nil {
			t.Fatal(err)
		}

		if disabled(t, e, user) {
			t.Error("le compte reste desactive apres l'annulation du dernier effacement")
		}
	})
}

func TestRgpdErasure_InactiveUsersAndLoginCancels(t *testing.T) {
	e := newErasureEnv(t, 48*time.Hour)
	ctx := context.Background()

	hash, err := security.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}

	stale := e.member(t, "inactif", "team", nil)
	staleClient := e.member(t, "inactif-portail", "client", nil)
	admin := e.member(t, "admin-inactif", "admin", nil)
	fresh := e.member(t, "actif", "team", nil)

	e.exec(t, `UPDATE users SET password_hash = $2, last_login_at = now() - interval '100 days' WHERE id = ANY($1)`, []uuid.UUID{stale, staleClient, admin}, hash)
	e.exec(t, `UPDATE users SET last_login_at = now() WHERE id = $1`, fresh)

	if _, err := e.svc.ScheduleInactive(ctx); err != nil {
		t.Fatalf("ScheduleInactive : %v", err)
	}

	scheduled := func(id uuid.UUID) int {
		return e.count(t, `SELECT count(*) FROM rgpd_erasures WHERE subject_type = 'user' AND subject_id = $1 AND reason = 'inactivity' AND status = 'scheduled'`, id)
	}

	if scheduled(stale) != 1 || scheduled(staleClient) != 1 {
		t.Error("un compte inactif n'est pas planifie")
	}

	if scheduled(admin) != 0 || scheduled(fresh) != 0 {
		t.Error("un admin ou un compte actif a ete planifie")
	}

	// Une seconde passe ne double rien.
	if _, err := e.svc.ScheduleInactive(ctx); err != nil {
		t.Fatal(err)
	}

	if scheduled(stale) != 1 {
		t.Error("deux effacements pour le meme compte")
	}

	// Le compte n'est pas desactive : il peut se connecter pour annuler.
	if e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND disabled_at IS NULL`, stale) != 1 {
		t.Error("un compte planifie pour inactivite a ete desactive")
	}

	auth, _ := newService(t)

	if _, err := auth.Login(ctx, usecase.LoginInput{Email: "inactif@" + e.prefix + ".test", Password: testPassword}); err != nil {
		t.Fatalf("connexion : %v", err)
	}

	if scheduled(stale) != 0 || e.count(t, `SELECT count(*) FROM rgpd_erasures WHERE subject_id = $1 AND status = 'cancelled'`, stale) != 1 {
		t.Error("la connexion n'a pas annule l'effacement")
	}

	if e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'rgpd.erasure_cancelled' AND actor_id = $1`, stale) != 1 {
		t.Error("annulation absente du journal d'audit")
	}

	// Jamais de purge avec le delai a zero.
	off := usecase.NewRgpdErasureService(e.pool, e.files, usecase.NewDiscordService(e.pool, []byte("test-secret-test-secret-test-secret")), time.Hour, 0, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if n, err := off.ScheduleInactive(ctx); err != nil || n != 0 {
		t.Errorf("purge desactivee : %d, %v", n, err)
	}
}

// Une personne qui reste connectee par rafraichissement n'est pas inactive :
// le refresh annule l'effacement planifie et met a jour la derniere activite.
func TestRgpdErasure_RefreshCancelsInactivityErasure(t *testing.T) {
	e := newErasureEnv(t, 48*time.Hour)
	ctx := context.Background()

	hash, err := security.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}

	id := e.member(t, "refresh", "team", nil)
	e.exec(t, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash)

	auth, _ := newService(t)

	session, err := auth.Login(ctx, usecase.LoginInput{Email: "refresh@" + e.prefix + ".test", Password: testPassword})
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}

	// Plus de mot de passe depuis 100 jours : seule la session vit.
	e.exec(t, `UPDATE users SET last_login_at = now() - interval '100 days' WHERE id = $1`, id)

	if _, err := e.svc.ScheduleInactive(ctx); err != nil {
		t.Fatalf("ScheduleInactive : %v", err)
	}

	pending := `SELECT count(*) FROM rgpd_erasures WHERE subject_type = 'user' AND subject_id = $1 AND status = 'scheduled'`
	if e.count(t, pending, id) != 1 {
		t.Fatal("le compte inactif n'est pas planifie")
	}

	if _, err := auth.Refresh(ctx, session.RefreshToken, "test", nil); err != nil {
		t.Fatalf("refresh : %v", err)
	}

	if e.count(t, pending, id) != 0 || e.count(t, `SELECT count(*) FROM rgpd_erasures WHERE subject_id = $1 AND status = 'cancelled'`, id) != 1 {
		t.Error("le refresh n'a pas annule l'effacement pour inactivite")
	}

	if e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND last_login_at > now() - interval '1 hour'`, id) != 1 {
		t.Error("le refresh n'a pas mis a jour la derniere activite")
	}

	if e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'rgpd.erasure_cancelled' AND actor_id = $1 AND details->>'via' = 'refresh'`, id) != 1 {
		t.Error("annulation par refresh absente du journal d'audit")
	}

	// Et la prochaine passe ne le replanifie pas.
	if _, err := e.svc.ScheduleInactive(ctx); err != nil {
		t.Fatal(err)
	}

	if e.count(t, pending, id) != 0 {
		t.Error("un compte actif a ete replanifie")
	}
}

func TestRgpdErasure_RollbackOnFailure(t *testing.T) {
	ctx := context.Background()

	t.Run("compte", func(t *testing.T) {
		e := newErasureEnv(t, 0)
		uid, did, avatarKey := e.richUser(t, "claire")
		item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
		if err != nil {
			t.Fatal(err)
		}

		events := e.count(t, `SELECT count(*) FROM bot_events`)
		e.makeDue(t, item.ID)
		e.svc.FailAfter("exports")

		if _, err := e.svc.Execute(ctx); !errors.Is(err, usecase.ErrInjected) {
			t.Fatalf("Execute : %v, attendu l'echec injecte", err)
		}

		if got := e.erasure(t, item.ID); got.Status != usecase.RgpdErasureFailed || got.Error == "" {
			t.Errorf("etat apres echec : %+v", got)
		}

		if e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND deleted_at IS NULL AND city = 'Nantes' AND email = $2`, uid, "claire@"+e.prefix+".test") != 1 {
			t.Error("le compte a ete modifie malgre l'echec")
		}

		if e.count(t, `SELECT count(*) FROM discord_links WHERE discord_user_id = $1`, did) != 1 ||
			e.count(t, `SELECT count(*) FROM bot_events`) != events ||
			e.count(t, `SELECT count(*) FROM refresh_tokens WHERE user_id = $1`, uid) != 1 ||
			e.count(t, `SELECT count(*) FROM email_outbox WHERE to_address = $1`, "claire@"+e.prefix+".test") != 1 ||
			e.count(t, `SELECT count(*) FROM tickets WHERE requester_email = $1`, "claire@"+e.prefix+".test") != 1 {
			t.Error("une etape n'a pas ete annulee")
		}

		if !e.fileExists(t, avatarKey) {
			t.Error("un fichier a ete supprime malgre l'echec")
		}

		if e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'rgpd.erasure_executed' AND target_id = $1`, item.ID.String()) != 0 {
			t.Error("audit ecrit malgre l'echec")
		}
	})

	t.Run("client", func(t *testing.T) {
		e := newErasureEnv(t, 0)

		client, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " Rollback", Kind: usecase.ClientProfessionnel, Status: "actif"})
		if err != nil {
			t.Fatal(err)
		}

		project, err := e.q.CreateProject(ctx, db.CreateProjectParams{ClientID: client.ID, Name: e.prefix + " projet", Status: "production", Priority: "high"})
		if err != nil {
			t.Fatal(err)
		}

		portal := e.member(t, "portail", "client", &client.ID)
		key := e.save(t, "fichier")
		e.exec(t, `INSERT INTO attachments (project_id, filename, content_type, size_bytes, storage_key) VALUES ($1, $2, 'text/plain', 1, $3)`, project.ID, e.prefix+"-f", key)

		item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectClient, client.ID)
		if err != nil {
			t.Fatal(err)
		}

		e.makeDue(t, item.ID)
		e.svc.FailAfter("projects")

		if _, err := e.svc.Execute(ctx); !errors.Is(err, usecase.ErrInjected) {
			t.Fatalf("Execute : %v", err)
		}

		if e.count(t, `SELECT count(*) FROM clients WHERE id = $1`, client.ID) != 1 ||
			e.count(t, `SELECT count(*) FROM projects WHERE id = $1`, project.ID) != 1 ||
			e.count(t, `SELECT count(*) FROM attachments WHERE storage_key = $1`, key) != 1 ||
			e.count(t, `SELECT count(*) FROM users WHERE id = $1 AND deleted_at IS NULL AND client_id = $2`, portal, client.ID) != 1 {
			t.Error("le client n'est pas intact apres l'echec")
		}

		if !e.fileExists(t, key) {
			t.Error("fichier supprime malgre l'echec")
		}
	})
}

func TestRgpdErasure_NoDoubleExecution(t *testing.T) {
	e := newErasureEnv(t, 0)
	ctx := context.Background()

	const total = 6

	var ids []uuid.UUID

	for i := 0; i < total; i++ {
		uid, _, _ := e.richUser(t, fmt.Sprintf("w%d", i))

		item, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, uid)
		if err != nil {
			t.Fatal(err)
		}

		e.makeDue(t, item.ID)
		ids = append(ids, item.ID)
	}

	// Deux instances (deux services), deux goroutines chacune.
	other := usecase.NewRgpdErasureService(e.pool, e.files, usecase.NewDiscordService(e.pool, []byte("test-secret-test-secret-test-secret")),
		720*time.Hour, 0, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	var (
		wg       sync.WaitGroup
		executed atomic.Int64
		failures atomic.Int64
	)

	for _, svc := range []*usecase.RgpdErasureService{e.svc, e.svc, other, other} {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				worked, err := svc.Execute(ctx)
				if err != nil {
					failures.Add(1)
					return
				}

				if !worked {
					return
				}

				executed.Add(1)
			}
		}()
	}

	wg.Wait()

	if failures.Load() != 0 {
		t.Fatalf("%d echec(s) d'execution", failures.Load())
	}

	if executed.Load() < total {
		t.Fatalf("%d execution(s), au moins %d attendues", executed.Load(), total)
	}

	for _, id := range ids {
		if got := e.erasure(t, id); got.Status != usecase.RgpdErasureDone {
			t.Errorf("effacement %s : %s (%s)", id, got.Status, got.Error)
		}

		if n := e.count(t, `SELECT count(*) FROM audit_log WHERE action = 'rgpd.erasure_executed' AND target_id = $1`, id.String()); n != 1 {
			t.Errorf("effacement %s execute %d fois", id, n)
		}
	}
}

// Ce que l'export d'un client archive est exactement ce que son effacement
// supprime : pieces jointes des projets, des taches et des tickets, et logos.
func TestRgpdErasure_ExportedFilesEqualErasedFiles(t *testing.T) {
	e := newErasureEnv(t, 0)
	ctx := context.Background()

	mk := func(name string) (uuid.UUID, uuid.UUID) {
		c, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{Name: e.prefix + " " + name, Kind: usecase.ClientProfessionnel, Status: "actif"})
		if err != nil {
			t.Fatal(err)
		}

		p, err := e.q.CreateProject(ctx, db.CreateProjectParams{ClientID: c.ID, Name: e.prefix + " p-" + name, Status: "production", Priority: "high"})
		if err != nil {
			t.Fatal(err)
		}

		return c.ID, p.ID
	}

	clientID, projectID := mk("cible")
	_, otherProject := mk("autre")

	contents := map[string]string{} // cle -> contenu
	attach := func(col string, owner uuid.UUID, name, content string) {
		key := e.save(t, content)
		contents[key] = content
		e.exec(t, `INSERT INTO attachments (`+col+`, filename, content_type, size_bytes, storage_key) VALUES ($1, $2, 'text/plain', 1, $3)`, owner, e.prefix+"-"+name, key)
	}

	task := e.scanID(t, `INSERT INTO tasks (project_id, title) VALUES ($1, 'tache') RETURNING id`, projectID)
	ticket := e.scanID(t, `INSERT INTO tickets (project_id, subject) VALUES ($1, 'ticket') RETURNING id`, projectID)

	attach("project_id", projectID, "projet", "contenu-projet")
	attach("task_id", task, "tache", "contenu-tache")
	attach("ticket_id", ticket, "ticket", "contenu-ticket")

	logoKey := e.save(t, "contenu-logo")
	contents[logoKey] = "contenu-logo"
	e.exec(t, `UPDATE projects SET logo_key = $2 WHERE id = $1`, projectID, logoKey)

	// Un fichier d'un autre client : ni exporte ni efface.
	decoy := e.save(t, "contenu-autre-client")
	e.exec(t, `INSERT INTO attachments (project_id, filename, content_type, size_bytes, storage_key) VALUES ($1, $2, 'text/plain', 1, $3)`, otherProject, e.prefix+"-autre", decoy)

	item := e.buildOne(t, usecase.RgpdSubjectClient, clientID)
	a := e.archive(t, item.ID)

	exported := map[string]bool{}

	for name, b := range a.entries {
		if strings.HasPrefix(name, "files/") {
			exported[string(b)] = true
		}
	}

	if got := a.manifest(t).Files; got.Included != 4 || len(got.Missing) != 0 {
		t.Errorf("manifeste : %+v, attendu 4 fichiers", got)
	}

	if got := len(a.rows(t, "attachments")); got != 3 {
		t.Errorf("liste des pieces jointes : %d, attendu 3", got)
	}

	erasure, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectClient, clientID)
	if err != nil {
		t.Fatal(err)
	}

	e.makeDue(t, erasure.ID)
	e.drain(t)

	erased := map[string]bool{}

	for key, content := range contents {
		if !e.fileExists(t, key) {
			erased[content] = true
		}
	}

	if len(exported) != len(erased) {
		t.Fatalf("exportes %v, effaces %v", exported, erased)
	}

	for c := range exported {
		if !erased[c] {
			t.Errorf("%q est exporte mais pas efface", c)
		}
	}

	if !e.fileExists(t, decoy) {
		t.Error("le fichier d'un autre client a ete supprime")
	}
}

func TestRgpdErasure_ListFilteredBySubject(t *testing.T) {
	e := newErasureEnv(t, 0)
	ctx := context.Background()

	first := e.member(t, "filtre-a", "team", nil)
	second := e.member(t, "filtre-b", "team", nil)

	one, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, first)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := e.svc.Request(ctx, nil, usecase.RgpdSubjectUser, second); err != nil {
		t.Fatal(err)
	}

	scheduled := usecase.RgpdErasureScheduled

	page, err := e.svc.List(ctx, usecase.RgpdErasureFilter{Status: scheduled, SubjectType: usecase.RgpdSubjectUser, SubjectID: &first}, 1)
	if err != nil {
		t.Fatalf("List : %v", err)
	}

	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != one.ID {
		t.Fatalf("filtre par sujet : %+v", page)
	}

	// Meme identifiant, mauvais type : rien.
	page, err = e.svc.List(ctx, usecase.RgpdErasureFilter{SubjectType: usecase.RgpdSubjectClient, SubjectID: &first}, 1)
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("mauvais type : %+v, %v", page, err)
	}

	// Combine avec le statut : une fois annule, le sujet n'est plus « scheduled ».
	if _, err := e.svc.Cancel(ctx, nil, one.ID); err != nil {
		t.Fatal(err)
	}

	page, err = e.svc.List(ctx, usecase.RgpdErasureFilter{Status: scheduled, SubjectID: &first}, 1)
	if err != nil || page.Total != 0 {
		t.Fatalf("apres annulation : %+v, %v", page, err)
	}

	page, err = e.svc.List(ctx, usecase.RgpdErasureFilter{Status: usecase.RgpdErasureCancelled, SubjectID: &first}, 1)
	if err != nil || page.Total != 1 {
		t.Fatalf("annule : %+v, %v", page, err)
	}

	if _, err := e.svc.List(ctx, usecase.RgpdErasureFilter{SubjectType: "robot"}, 1); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("type inconnu : %v, attendu une validation", err)
	}
}
