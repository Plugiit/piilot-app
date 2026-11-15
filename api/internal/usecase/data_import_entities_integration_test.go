//go:build integration

// Tests d'integration de l'import et de l'export des contacts, des projets et
// du temps : aller-retour sans perte, rejeu a zero cree, refus unknown_*,
// heures du projet recalculees, doublon jamais mis a jour.
package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/csvio"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// newEntitiesEnv pose en plus le menage des contacts, projets, saisies, comptes
// et services du test. Les clients sont nettoyes par newImportEnv ; celui-ci
// passe avant (les projets retiennent leur client).
func newEntitiesEnv(t *testing.T, maxRows int) *importEnv {
	t.Helper()

	e := newImportEnv(t, 8<<20, maxRows)

	t.Cleanup(func() {
		ctx := context.Background()
		p := e.prefix + "%"

		_, _ = e.pool.Exec(ctx, `DELETE FROM import_keys WHERE import_id = ANY($1)`, e.ids)
		_, _ = e.pool.Exec(ctx, `DELETE FROM time_entries WHERE project_id IN (SELECT id FROM projects WHERE name LIKE $1)`, p)
		_, _ = e.pool.Exec(ctx, `DELETE FROM projects WHERE name LIKE $1`, p)
		_, _ = e.pool.Exec(ctx, `UPDATE clients SET primary_contact_id = NULL WHERE name LIKE $1`, p)
		_, _ = e.pool.Exec(ctx, `DELETE FROM contacts WHERE firstname LIKE $1 OR lastname LIKE $1 OR email LIKE $2`, p, "%"+e.prefix+"%")
		_, _ = e.pool.Exec(ctx, `DELETE FROM users WHERE email LIKE $1`, "%"+e.prefix+"%")
		_, _ = e.pool.Exec(ctx, `DELETE FROM services WHERE name LIKE $1`, p)
	})

	return e
}

func (e *importEnv) uploadEntity(t *testing.T, entity, content string) usecase.ImportDetail {
	t.Helper()

	d, err := e.svc.Upload(context.Background(), nil, entity, entity+".csv", strings.NewReader(content))
	if err != nil {
		t.Fatalf("Upload %s : %v", entity, err)
	}

	e.ids = append(e.ids, d.ID)

	return d
}

// importCSV depose, importe pour de bon avec la correspondance proposee.
func (e *importEnv) importCSV(t *testing.T, entity, content string) (usecase.ImportDetail, map[int]usecase.ImportRowItem) {
	t.Helper()

	up := e.uploadEntity(t, entity, content)
	done := e.run(t, up.ID, up.SuggestedMapping, false)

	return done, e.rows(t, up.ID)
}

// exportOurs exporte l'entite et ne garde que les lignes qui portent le
// prefixe du test, en-tetes compris.
func (e *importEnv) exportOurs(t *testing.T, entity string) [][]string {
	t.Helper()

	var buf bytes.Buffer
	if err := e.export.Write(context.Background(), entity, &buf); err != nil {
		t.Fatal(err)
	}

	r, err := csvio.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}

	rows := [][]string{r.Headers()}

	for {
		row, err := r.Next()
		if errors.Is(err, io.EOF) {
			return rows
		}

		if err != nil {
			t.Fatal(err)
		}

		if strings.Contains(strings.Join(row.Cells, "\x00"), e.prefix) {
			rows = append(rows, row.Cells)
		}
	}
}

func (e *importEnv) mustExec(t *testing.T, sql string, args ...any) {
	t.Helper()

	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s : %v", sql, err)
	}
}

func (e *importEnv) newClient(t *testing.T, suffix, siret string) db.Client {
	t.Helper()

	c, err := e.q.CreateImportedClient(context.Background(), db.CreateImportedClientParams{
		Name: e.name(suffix), Kind: usecase.ClientProfessionnel, Siret: siret, Status: "actif",
	})
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func (e *importEnv) newUser(t *testing.T, tag, role string) string {
	t.Helper()

	email := tag + "@" + e.prefix + ".test"
	e.mustExec(t, `INSERT INTO users (email, password_hash, role) VALUES ($1, 'x', $2)`, email, role)

	return email
}

func (e *importEnv) newProject(t *testing.T, client db.Client, suffix string) db.Project {
	t.Helper()

	p, err := e.q.CreateProject(context.Background(), db.CreateProjectParams{
		ClientID: client.ID, Name: e.name(suffix), Status: "production", Priority: "medium",
	})
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func expectReasons(t *testing.T, rows map[int]usecase.ImportRowItem, want map[int][2]string) {
	t.Helper()

	for line, w := range want {
		got := rows[line]
		if got.Outcome != w[0] || got.Reason != w[1] {
			t.Errorf("ligne %d : %s/%s (%q), attendu %s/%s", line, got.Outcome, got.Reason, got.Label, w[0], w[1])
		}
	}
}

func TestImportContacts(t *testing.T) {
	e := newEntitiesEnv(t, 1000)
	ctx := context.Background()

	withSiret := randomSiret()
	a := e.newClient(t, "A", withSiret)
	b := e.newClient(t, "B", "")

	header := []string{"Prénom", "Nom", "Fonction", "E-mail", "Téléphone", "Client", "SIRET client"}
	mail := func(tag string) string { return tag + "@" + e.prefix + ".test" }

	content := csvOf(t, header,
		[]string{e.name("Camille"), "Durand", "DG", mail("camille"), "0102030405", "", withSiret}, // client par SIRET
		[]string{e.name("Léa"), "Martin", "", "", "", b.Name, ""},                                 // client par nom, sans e-mail
		[]string{e.name("Libre"), "Sans-Client", "", mail("libre"), "", "", ""},                   // contact libre
		[]string{e.name("Inconnu"), "X", "", mail("x"), "", "Client Fantome " + e.prefix, ""},     // unknown_client
		[]string{"", "", "", mail("vide"), "", "", ""},                                            // invalide
	)

	done, rows := e.importCSV(t, usecase.DataEntityContacts, content)
	if done.CreatedN != 3 || done.SkippedN != 0 || done.ErrorN != 2 {
		t.Fatalf("created=%d skipped=%d error=%d : %v", done.CreatedN, done.SkippedN, done.ErrorN, rows)
	}

	expectReasons(t, rows, map[int][2]string{
		2: {"created", ""}, 3: {"created", ""}, 4: {"created", ""},
		5: {usecase.ImportRowError, usecase.ReasonUnknownClient}, 6: {usecase.ImportRowError, usecase.ReasonInvalid},
	})

	if !strings.Contains(rows[5].Label, "Inconnu") || !strings.Contains(rows[5].Label, " : client : ") {
		t.Errorf("libelle d'une ligne refusee : %q", rows[5].Label)
	}

	camille, err := e.q.GetContactByEmail(ctx, mail("camille"))
	if err != nil || camille.ClientID == nil || *camille.ClientID != a.ID || camille.Role != "DG" {
		t.Fatalf("camille : %+v (%v)", camille, err)
	}

	libre, err := e.q.GetContactByEmail(ctx, mail("libre"))
	if err != nil || libre.ClientID != nil {
		t.Fatalf("contact libre : %+v (%v)", libre, err)
	}

	// Le premier contact d'un client en devient l'interlocuteur principal.
	if got, _ := e.q.GetClientByID(ctx, a.ID); got.PrimaryContactID == nil || *got.PrimaryContactID != camille.ID {
		t.Errorf("contact principal : %v", got.PrimaryContactID)
	}

	// Rejeu : rien de neuf.
	again, rows2 := e.importCSV(t, usecase.DataEntityContacts, content)
	if again.CreatedN != 0 || again.SkippedN != 3 {
		t.Fatalf("rejeu : created=%d skipped=%d", again.CreatedN, again.SkippedN)
	}

	for _, line := range []int{2, 3, 4} {
		if rows2[line].Reason != usecase.ReasonAlreadyImported {
			t.Errorf("rejeu ligne %d : %s", line, rows2[line].Reason)
		}
	}

	// Doublons metier, jamais mis a jour : meme e-mail en casse differente,
	// et meme prenom + nom chez le meme client sans e-mail.
	// Contacts saisis a la main : seuls les doublons metier les attrapent.
	natMail := mail("natif")
	mustContact := func(first, last string, email *string) {
		if _, err := e.q.CreateContact(ctx, db.CreateContactParams{ClientID: &b.ID, Firstname: first, Lastname: last, Role: "Original", Email: email}); err != nil {
			t.Fatal(err)
		}
	}
	mustContact(e.name("Natif"), "Mail", &natMail)
	mustContact(e.name("Natifb"), "Nom", nil)

	dup := csvOf(t, header,
		[]string{e.name("Autre"), "Prenom", "Stagiaire", strings.ToUpper(natMail), "", a.Name, ""},
		[]string{strings.ToUpper(e.name("Natifb")), "NOM", "Changee", "", "0999", b.Name, ""},
	)

	dupDone, dupRows := e.importCSV(t, usecase.DataEntityContacts, dup)
	if dupDone.CreatedN != 0 || dupDone.SkippedN != 2 {
		t.Fatalf("doublons : created=%d skipped=%d %v", dupDone.CreatedN, dupDone.SkippedN, dupRows)
	}

	expectReasons(t, dupRows, map[int][2]string{
		2: {usecase.ImportRowSkipped, usecase.ReasonDuplicateEmail}, 3: {usecase.ImportRowSkipped, usecase.ReasonDuplicateName},
	})

	if got, _ := e.q.GetContactByEmail(ctx, natMail); got.Role != "Original" || got.Phone != "" || got.Firstname != e.name("Natif") {
		t.Errorf("le doublon a modifie le contact : %+v", got)
	}

	// Aller-retour : export, suppression pour de bon, import, comparaison.
	type snap struct{ first, last, role, phone, email, client string }

	read := func() map[string]snap {
		out := map[string]snap{}

		list, err := e.pool.Query(ctx, `SELECT ct.firstname, ct.lastname, ct.role, ct.phone, coalesce(ct.email, ''), coalesce(c.name, '')
			FROM contacts ct LEFT JOIN clients c ON c.id = ct.client_id WHERE ct.firstname LIKE $1 AND ct.deleted_at IS NULL`, e.prefix+"%")
		if err != nil {
			t.Fatal(err)
		}
		defer list.Close()

		for list.Next() {
			var s snap
			if err := list.Scan(&s.first, &s.last, &s.role, &s.phone, &s.email, &s.client); err != nil {
				t.Fatal(err)
			}

			out[s.first+"|"+s.last] = s
		}

		return out
	}

	before := read()
	exported := e.exportOurs(t, usecase.DataEntityContacts)

	if len(exported) != 6 || len(before) != 5 {
		t.Fatalf("%d contacts exportes, %d en base, attendu 5", len(exported)-1, len(before))
	}

	e.mustExec(t, `UPDATE clients SET primary_contact_id = NULL WHERE name LIKE $1`, e.prefix+"%")
	e.mustExec(t, `DELETE FROM contacts WHERE firstname LIKE $1`, e.prefix+"%")

	rt, _ := e.importCSV(t, usecase.DataEntityContacts, csvOf(t, exported...))
	if rt.CreatedN != 5 || rt.SkippedN != 0 || rt.ErrorN != 0 {
		t.Fatalf("aller-retour : created=%d skipped=%d error=%d", rt.CreatedN, rt.SkippedN, rt.ErrorN)
	}

	after := read()
	if len(after) != len(before) {
		t.Fatalf("%d contacts apres, %d avant", len(after), len(before))
	}

	for k, want := range before {
		if after[k] != want {
			t.Errorf("%s : %+v, attendu %+v", k, after[k], want)
		}
	}

	replay, _ := e.importCSV(t, usecase.DataEntityContacts, csvOf(t, exported...))
	if replay.CreatedN != 0 {
		t.Fatalf("rejeu apres aller-retour : %d crees", replay.CreatedN)
	}
}

func TestImportProjects(t *testing.T) {
	e := newEntitiesEnv(t, 1000)
	ctx := context.Background()

	siret := randomSiret()
	a := e.newClient(t, "A", siret)
	b := e.newClient(t, "B", "")

	header := []string{"Nom", "Client", "SIRET client", "Statut", "Priorité", "Description", "Début", "Échéance", "Heures vendues"}

	content := csvOf(t, header,
		[]string{e.name("Refonte"), b.Name, "", "En production", "Haute", "Site vitrine\n=cmd", "01/02/2026", "2026-06-30", "12,5"},
		[]string{e.name("Boutique"), "", siret, "attente", "", "", "", "", ""},
		[]string{e.name("Sans client"), "", "", "", "", "", "", "", ""},
		[]string{e.name("Client fantome"), "Fantome " + e.prefix, "", "", "", "", "", "", ""},
		[]string{e.name("Statut faux"), b.Name, "", "gelé", "", "", "", "", ""},
		[]string{e.name("Dates inversees"), b.Name, "", "", "", "", "2026-05-01", "2026-04-01", ""},
	)

	done, rows := e.importCSV(t, usecase.DataEntityProjects, content)
	if done.CreatedN != 2 || done.ErrorN != 4 {
		t.Fatalf("created=%d skipped=%d error=%d : %v", done.CreatedN, done.SkippedN, done.ErrorN, rows)
	}

	expectReasons(t, rows, map[int][2]string{
		// La description de la premiere ligne tient sur deux lignes du fichier.
		2: {"created", ""}, 4: {"created", ""},
		5: {usecase.ImportRowError, usecase.ReasonUnknownClient}, 6: {usecase.ImportRowError, usecase.ReasonUnknownClient},
		7: {usecase.ImportRowError, usecase.ReasonInvalid}, 8: {usecase.ImportRowError, usecase.ReasonInvalid},
	})

	// Jamais de client cree a la volee.
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM clients WHERE name LIKE '%Fantome%' AND name LIKE $1`, "%"+e.prefix+"%").Scan(&n); err != nil || n != 0 {
		t.Fatalf("client fantome cree : %d (%v)", n, err)
	}

	refonte, err := e.q.GetProjectByClientAndName(ctx, db.GetProjectByClientAndNameParams{ClientID: b.ID, Name: e.name("refonte")})
	if err != nil {
		t.Fatal(err)
	}

	if refonte.Status != "production" || refonte.Priority != "high" || refonte.HoursSold != 12.5 ||
		refonte.StartsOn == nil || refonte.StartsOn.Format("2006-01-02") != "2026-02-01" || refonte.Description != "Site vitrine\n=cmd" {
		t.Errorf("refonte : %+v", refonte)
	}

	boutique, err := e.q.GetProjectByClientAndName(ctx, db.GetProjectByClientAndNameParams{ClientID: a.ID, Name: e.name("Boutique")})
	if err != nil || boutique.Status != "attente" || boutique.Priority != "medium" {
		t.Fatalf("boutique : %+v (%v)", boutique, err)
	}

	// Rejeu.
	again, _ := e.importCSV(t, usecase.DataEntityProjects, content)
	if again.CreatedN != 0 || again.SkippedN != 2 {
		t.Fatalf("rejeu : created=%d skipped=%d", again.CreatedN, again.SkippedN)
	}

	// Doublon (client, nom en minuscules) d'un projet cree a la main : ignore,
	// pas mis a jour.
	native := e.newProject(t, b, "Natif")

	dup, dupRows := e.importCSV(t, usecase.DataEntityProjects, csvOf(t, header,
		[]string{strings.ToUpper(e.name("Natif")), b.Name, "", "livre", "basse", "", "", "", "99"},
	))
	if dup.CreatedN != 0 || dupRows[2].Reason != usecase.ReasonDuplicateName || *dupRows[2].EntityID != native.ID {
		t.Fatalf("doublon : %+v %v", dup, dupRows)
	}

	if got, _ := e.q.GetProjectByClientAndName(ctx, db.GetProjectByClientAndNameParams{ClientID: b.ID, Name: e.name("natif")}); got.Status != "production" || got.HoursSold != 0 || got.Priority != "medium" {
		t.Errorf("le doublon a modifie le projet : %+v", got)
	}

	// Le meme nom chez un autre client n'est pas un doublon.
	other, _ := e.importCSV(t, usecase.DataEntityProjects, csvOf(t, header, []string{e.name("Refonte"), a.Name, "", "", "", "", "", "", ""}))
	if other.CreatedN != 1 {
		t.Fatalf("meme nom chez un autre client : created=%d", other.CreatedN)
	}

	// Aller-retour.
	type snap struct {
		status, priority, desc, start, due string
		sold                               float64
	}

	read := func() map[string]snap {
		out := map[string]snap{}

		list, err := e.pool.Query(ctx, `SELECT c.name || '|' || p.name, p.status, p.priority, p.description,
			coalesce(p.starts_on::text, ''), coalesce(p.due_on::text, ''), p.hours_sold
			FROM projects p JOIN clients c ON c.id = p.client_id WHERE p.name LIKE $1 AND p.deleted_at IS NULL`, e.prefix+"%")
		if err != nil {
			t.Fatal(err)
		}
		defer list.Close()

		for list.Next() {
			var k string
			var s snap
			if err := list.Scan(&k, &s.status, &s.priority, &s.desc, &s.start, &s.due, &s.sold); err != nil {
				t.Fatal(err)
			}

			out[k] = s
		}

		return out
	}

	before := read()
	exported := e.exportOurs(t, usecase.DataEntityProjects)

	if len(exported) != 5 || len(before) != 4 {
		t.Fatalf("%d projets exportes, %d en base, attendu 4", len(exported)-1, len(before))
	}

	e.mustExec(t, `DELETE FROM projects WHERE name LIKE $1`, e.prefix+"%")

	rt, _ := e.importCSV(t, usecase.DataEntityProjects, csvOf(t, exported...))
	if rt.CreatedN != 4 || rt.SkippedN != 0 || rt.ErrorN != 0 {
		t.Fatalf("aller-retour : created=%d skipped=%d error=%d", rt.CreatedN, rt.SkippedN, rt.ErrorN)
	}

	after := read()
	for k, want := range before {
		if after[k] != want {
			t.Errorf("%s : %+v, attendu %+v", k, after[k], want)
		}
	}

	if len(after) != len(before) {
		t.Fatalf("%d projets apres, %d avant", len(after), len(before))
	}

	if replay, _ := e.importCSV(t, usecase.DataEntityProjects, csvOf(t, exported...)); replay.CreatedN != 0 {
		t.Fatalf("rejeu apres aller-retour : %d crees", replay.CreatedN)
	}
}

func (e *importEnv) hoursSpent(t *testing.T, projectID uuid.UUID) float64 {
	t.Helper()

	var h float64
	if err := e.pool.QueryRow(context.Background(), `SELECT hours_spent FROM projects WHERE id = $1`, projectID).Scan(&h); err != nil {
		t.Fatal(err)
	}

	return h
}

func TestImportTimeEntries(t *testing.T) {
	e := newEntitiesEnv(t, 1000)
	ctx := context.Background()

	client := e.newClient(t, "A", "")
	other := e.newClient(t, "B", "")
	proj := e.newProject(t, client, "Site")
	e.newProject(t, other, "Site") // meme nom chez un autre client
	solo := e.newProject(t, client, "Solo")

	team := e.newUser(t, "equipe", "team")
	admin := e.newUser(t, "admin", "admin")
	portal := e.newUser(t, "portail", "client")
	gone := e.newUser(t, "parti", "team")
	e.mustExec(t, `UPDATE users SET deleted_at = now() WHERE email = $1`, gone)

	e.mustExec(t, `INSERT INTO services (name) VALUES ($1)`, e.name("Dev"))

	header := []string{"E-mail", "Client", "Projet", "Service", "Date", "Durée (min)", "Note"}
	day := "2026-03-02"

	content := csvOf(t, header,
		[]string{team, client.Name, proj.Name, e.name("dev"), day, "90", "Maquettes"},        // 2 : cree
		[]string{admin, client.Name, proj.Name, "", "03/03/2026", "1h", ""},                  // 3 : cree, heures
		[]string{"inconnu@" + e.prefix + ".test", client.Name, proj.Name, "", day, "30", ""}, // 4 : unknown_user
		[]string{portal, client.Name, proj.Name, "", day, "30", ""},                          // 5 : compte client refuse
		[]string{gone, client.Name, proj.Name, "", day, "30", ""},                            // 6 : compte supprime
		[]string{team, client.Name, e.name("Fantome"), "", day, "30", ""},                    // 7 : unknown_project
		[]string{team, "", proj.Name, "", day, "30", ""},                                     // 8 : ambigu sans client
		[]string{team, client.Name, proj.Name, "", day, "0", ""},                             // 9 : 0 min
		[]string{team, client.Name, proj.Name, "", day, "1441", ""},                          // 10 : > 24 h
		[]string{team, client.Name, proj.Name, e.name("Inconnu"), day, "30", ""},             // 11 : service inconnu
		[]string{team, "", solo.Name, "", day, "45", "Sans client"},                          // 12 : projet unique sans client
	)

	done, rows := e.importCSV(t, usecase.DataEntityTimeEntries, content)
	if done.CreatedN != 3 || done.SkippedN != 0 || done.ErrorN != 8 {
		t.Fatalf("created=%d skipped=%d error=%d : %v", done.CreatedN, done.SkippedN, done.ErrorN, rows)
	}

	expectReasons(t, rows, map[int][2]string{
		2: {"created", ""}, 3: {"created", ""}, 12: {"created", ""},
		4: {usecase.ImportRowError, usecase.ReasonUnknownUser}, 5: {usecase.ImportRowError, usecase.ReasonUnknownUser},
		6: {usecase.ImportRowError, usecase.ReasonUnknownUser}, 7: {usecase.ImportRowError, usecase.ReasonUnknownProject},
		8: {usecase.ImportRowError, usecase.ReasonInvalid}, 9: {usecase.ImportRowError, usecase.ReasonInvalid},
		10: {usecase.ImportRowError, usecase.ReasonInvalid}, 11: {usecase.ImportRowError, usecase.ReasonInvalid},
	})

	// 90 + 60 minutes : 2,5 h, recalculees par le declencheur.
	if h := e.hoursSpent(t, proj.ID); h != 2.5 {
		t.Errorf("hours_spent = %v, attendu 2.5", h)
	}

	if h := e.hoursSpent(t, solo.ID); h != 0.75 {
		t.Errorf("hours_spent du projet unique = %v, attendu 0.75", h)
	}

	// Rejeu : rien de neuf, heures inchangees.
	again, _ := e.importCSV(t, usecase.DataEntityTimeEntries, content)
	if again.CreatedN != 0 || again.SkippedN != 3 {
		t.Fatalf("rejeu : created=%d skipped=%d", again.CreatedN, again.SkippedN)
	}

	if h := e.hoursSpent(t, proj.ID); h != 2.5 {
		t.Errorf("hours_spent apres rejeu = %v", h)
	}

	// Aller-retour : export, suppression pour de bon (le declencheur ramene
	// les heures a 0), import, memes saisies et memes heures.
	type snap struct {
		user, project, service, day, note string
		minutes                           int
	}

	read := func() []snap {
		var out []snap

		list, err := e.pool.Query(ctx, `SELECT u.email, p.name, coalesce(s.name, ''), te.spent_on::text, te.note, te.minutes
			FROM time_entries te JOIN users u ON u.id = te.user_id JOIN projects p ON p.id = te.project_id
			LEFT JOIN services s ON s.id = te.service_id
			WHERE p.name LIKE $1 AND te.deleted_at IS NULL ORDER BY te.spent_on, p.name, u.email, te.minutes`, e.prefix+"%")
		if err != nil {
			t.Fatal(err)
		}
		defer list.Close()

		for list.Next() {
			var s snap
			if err := list.Scan(&s.user, &s.project, &s.service, &s.day, &s.note, &s.minutes); err != nil {
				t.Fatal(err)
			}

			out = append(out, s)
		}

		return out
	}

	before := read()
	exported := e.exportOurs(t, usecase.DataEntityTimeEntries)

	if len(before) != 3 || len(exported) != 4 {
		t.Fatalf("%d saisies en base, %d exportees, attendu 3", len(before), len(exported)-1)
	}

	e.mustExec(t, `DELETE FROM time_entries WHERE project_id IN (SELECT id FROM projects WHERE name LIKE $1)`, e.prefix+"%")

	if h := e.hoursSpent(t, proj.ID); h != 0 {
		t.Fatalf("hours_spent apres suppression = %v", h)
	}

	rt, _ := e.importCSV(t, usecase.DataEntityTimeEntries, csvOf(t, exported...))
	if rt.CreatedN != 3 || rt.SkippedN != 0 || rt.ErrorN != 0 {
		t.Fatalf("aller-retour : created=%d skipped=%d error=%d", rt.CreatedN, rt.SkippedN, rt.ErrorN)
	}

	after := read()
	if fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("apres : %+v\navant : %+v", after, before)
	}

	if h := e.hoursSpent(t, proj.ID); h != 2.5 {
		t.Errorf("hours_spent apres aller-retour = %v, attendu 2.5", h)
	}

	if replay, _ := e.importCSV(t, usecase.DataEntityTimeEntries, csvOf(t, exported...)); replay.CreatedN != 0 {
		t.Fatalf("rejeu apres aller-retour : %d crees", replay.CreatedN)
	}

	// Un doublon n'est jamais mis a jour : meme identifiant d'origine, duree
	// et note changees, la saisie existante garde ses valeurs.
	withID := []string{"Identifiant d'origine", "E-mail", "Client", "Projet", "Date", "Durée (min)", "Note"}

	first, _ := e.importCSV(t, usecase.DataEntityTimeEntries, csvOf(t, withID,
		[]string{"ext-1-" + e.prefix, team, client.Name, proj.Name, "2026-04-01", "20", "v1"}))
	if first.CreatedN != 1 {
		t.Fatalf("saisie avec identifiant d'origine : created=%d", first.CreatedN)
	}

	second, secondRows := e.importCSV(t, usecase.DataEntityTimeEntries, csvOf(t, withID,
		[]string{"ext-1-" + e.prefix, team, client.Name, proj.Name, "2026-04-01", "200", "v2"}))
	if second.CreatedN != 0 || secondRows[2].Reason != usecase.ReasonAlreadyImported {
		t.Fatalf("doublon : %+v %v", second, secondRows)
	}

	var minutes int
	var note string
	if err := e.pool.QueryRow(ctx, `SELECT minutes, note FROM time_entries WHERE note = 'v1' AND project_id = $1`, proj.ID).Scan(&minutes, &note); err != nil || minutes != 20 {
		t.Fatalf("saisie modifiee par un doublon : %d %q (%v)", minutes, note, err)
	}
}

// TestImportTimeEntriesBench mesure l'import de 5 000 lignes de temps. Pas de
// seuil : il donne un chiffre. Lance avec PIILOT_BENCH_IMPORT=1.
func TestImportTimeEntriesBench(t *testing.T) {
	if os.Getenv("PIILOT_BENCH_IMPORT") == "" {
		t.Skip("PIILOT_BENCH_IMPORT non defini")
	}

	const total = 5000

	e := newEntitiesEnv(t, 10000)

	client := e.newClient(t, "Bench", "")
	proj := e.newProject(t, client, "Bench")
	user := e.newUser(t, "bench", "team")

	rows := [][]string{{"E-mail", "Client", "Projet", "Date", "Durée (min)", "Note"}}
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range total {
		rows = append(rows, []string{user, client.Name, proj.Name, start.AddDate(0, 0, i/10).Format("2006-01-02"), "30", fmt.Sprintf("ligne %d", i)})
	}

	content := csvOf(t, rows...)
	up := e.uploadEntity(t, usecase.DataEntityTimeEntries, content)

	t0 := time.Now()
	done := e.run(t, up.ID, up.SuggestedMapping, false)
	real := time.Since(t0)

	if done.CreatedN != total {
		t.Fatalf("created=%d", done.CreatedN)
	}

	up2 := e.uploadEntity(t, usecase.DataEntityTimeEntries, content)

	t0 = time.Now()
	replay := e.run(t, up2.ID, up2.SuggestedMapping, false)
	rejeu := time.Since(t0)

	if replay.CreatedN != 0 {
		t.Fatalf("rejeu : created=%d", replay.CreatedN)
	}

	t.Logf("BENCH import reel de %d lignes de temps : %s (%.0f lignes/s) ; rejeu : %s", total, real, total/real.Seconds(), rejeu)
}
