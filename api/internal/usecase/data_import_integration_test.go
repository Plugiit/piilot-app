//go:build integration

// Tests d'integration de l'import et de l'export CSV des clients.
//
// Ce qu'ils figent : le rejeu qui ne cree rien, le doublon ignore sans etre
// mis a jour, la ligne invalide qui n'arrete pas les autres, la simulation
// qui n'ecrit rien, et l'aller-retour export puis import sans perte.
package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/csvio"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

type importEnv struct {
	svc    *usecase.DataImportService
	export *usecase.DataExportService
	pool   *pgxpool.Pool
	q      *db.Queries
	files  *storage.Local
	dir    string
	prefix string
	ids    []uuid.UUID
}

func newImportEnv(t *testing.T, maxBytes int64, maxRows int) *importEnv {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL est obligatoire")
	}

	pool, err := repository.NewPool(context.Background(), url, repository.DefaultPoolConfig())
	if err != nil {
		t.Fatalf("ouverture du pool : %v", err)
	}
	t.Cleanup(pool.Close)

	dir := t.TempDir()

	files, err := storage.NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}

	env := &importEnv{
		svc:    usecase.NewDataImportService(pool, files, maxBytes, maxRows, slog.New(slog.NewJSONHandler(io.Discard, nil))),
		export: usecase.NewDataExportService(db.New(pool)),
		pool:   pool, q: db.New(pool), files: files,
		prefix: "imp" + uuid.NewString()[:8],
	}

	t.Cleanup(func() {
		ctx := context.Background()
		for _, id := range env.ids {
			_, _ = pool.Exec(ctx, `DELETE FROM imports WHERE id = $1`, id)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM import_keys WHERE entity = 'clients' AND entity_id IN (SELECT id FROM clients WHERE name LIKE $1)`, env.prefix+"%")
		_, _ = pool.Exec(ctx, `DELETE FROM clients WHERE name LIKE $1`, env.prefix+"%")
	})

	return env
}

func (e *importEnv) name(suffix string) string { return e.prefix + " " + suffix }

func csvOf(t *testing.T, rows ...[]string) string {
	t.Helper()

	var buf bytes.Buffer

	w := csvio.NewWriter(&buf)
	for _, r := range rows {
		for i := range r {
			r[i] = csvio.Cell(r[i])
		}

		if err := w.Write(r); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	return buf.String()
}

func (e *importEnv) upload(t *testing.T, content string) usecase.ImportDetail {
	t.Helper()

	d, err := e.svc.Upload(context.Background(), nil, usecase.DataEntityClients, "clients.csv", strings.NewReader(content))
	if err != nil {
		t.Fatalf("Upload : %v", err)
	}

	e.ids = append(e.ids, d.ID)

	return d
}

// run lance l'import avec la correspondance proposee et attend la fin.
func (e *importEnv) run(t *testing.T, id uuid.UUID, mapping map[string]string, dry bool) usecase.ImportDetail {
	t.Helper()

	ctx := context.Background()

	if _, err := e.svc.Run(ctx, id, usecase.RunImportInput{Mapping: mapping, DryRun: dry}); err != nil {
		t.Fatalf("Run : %v", err)
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

	d, err := e.svc.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	if d.Status != usecase.ImportDone {
		t.Fatalf("statut %q (erreur %q), attendu done", d.Status, d.Error)
	}

	return d
}

func (e *importEnv) rows(t *testing.T, id uuid.UUID) map[int]usecase.ImportRowItem {
	t.Helper()

	out := map[int]usecase.ImportRowItem{}

	for page := 1; ; page++ {
		p, err := e.svc.Rows(context.Background(), id, "", page)
		if err != nil {
			t.Fatal(err)
		}

		for _, r := range p.Items {
			out[r.Line] = r
		}

		if len(p.Items) == 0 || len(out) >= int(p.Total) {
			return out
		}
	}
}

func (e *importEnv) countClients(t *testing.T) int {
	t.Helper()

	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM clients WHERE name LIKE $1 AND deleted_at IS NULL`, e.prefix+"%").Scan(&n); err != nil {
		t.Fatal(err)
	}

	return n
}

var clientHeader = []string{"Nom", "SIRET", "Statut", "Ville"}

func TestImportClients_ReplayCreatesNothing(t *testing.T) {
	e := newImportEnv(t, 1<<20, 1000)

	content := csvOf(t, clientHeader,
		[]string{e.name("A"), randomSiret(), "actif", "Lille"},
		[]string{e.name("B"), "", "lead", "Nancy"},
		[]string{e.name("C"), randomSiret(), "", "Metz"},
	)

	first := e.upload(t, content)
	if first.RowCount != 3 || first.SuggestedMapping["name"] != "Nom" {
		t.Fatalf("upload : %+v", first)
	}

	done := e.run(t, first.ID, first.SuggestedMapping, false)
	if done.CreatedN != 3 || done.SkippedN != 0 || done.ErrorN != 0 {
		t.Fatalf("premier passage : created=%d skipped=%d error=%d", done.CreatedN, done.SkippedN, done.ErrorN)
	}

	if e.countClients(t) != 3 {
		t.Fatalf("%d clients, attendu 3", e.countClients(t))
	}

	// Le fichier d'un import reel est supprime a la fin.
	if done.Status != usecase.ImportDone {
		t.Fatal(done.Status)
	}

	// Meme fichier, un second import : rien de neuf.
	second := e.upload(t, content)
	done2 := e.run(t, second.ID, second.SuggestedMapping, false)

	if done2.CreatedN != 0 || done2.SkippedN != 3 {
		t.Fatalf("rejeu : created=%d skipped=%d, attendu 0 et 3", done2.CreatedN, done2.SkippedN)
	}

	for line, r := range e.rows(t, second.ID) {
		if r.Outcome != usecase.ImportRowSkipped || r.Reason != usecase.ReasonAlreadyImported {
			t.Errorf("ligne %d : %s/%s, attendu skipped/already_imported", line, r.Outcome, r.Reason)
		}
	}

	if e.countClients(t) != 3 {
		t.Fatalf("%d clients apres rejeu, attendu 3", e.countClients(t))
	}

	// Un client supprime puis reimporte est recree : la cle ne protege que
	// ce qui existe encore.
	if _, err := e.pool.Exec(context.Background(), `UPDATE clients SET deleted_at = now(), name = name || ' (supprime)' WHERE name = $1`, e.name("B")); err != nil {
		t.Fatal(err)
	}

	third := e.upload(t, content)
	done3 := e.run(t, third.ID, third.SuggestedMapping, false)

	if done3.CreatedN != 1 || done3.SkippedN != 2 {
		t.Fatalf("apres suppression : created=%d skipped=%d, attendu 1 et 2", done3.CreatedN, done3.SkippedN)
	}
}

func TestImportClients_ExistingAndDuplicatesAreIgnoredNotUpdated(t *testing.T) {
	e := newImportEnv(t, 1<<20, 1000)
	ctx := context.Background()

	siret := randomSiret()

	existing, err := e.q.CreateImportedClient(ctx, db.CreateImportedClientParams{
		Name: e.name("Existant"), Kind: usecase.ClientProfessionnel, Siret: siret, Status: "actif", Phone: "0102030405",
	})
	if err != nil {
		t.Fatal(err)
	}

	content := csvOf(t,
		[]string{"Identifiant Piilot", "Nom", "SIRET", "Téléphone", "Ville"},
		// Meme SIRET, autre nom.
		[]string{"", e.name("Homonyme SIRET"), siret, "0999999999", "Paris"},
		// Meme nom (casse differente), autre SIRET.
		[]string{"", strings.ToUpper(e.name("Existant")), randomSiret(), "0999999999", "Paris"},
		// Identifiant Piilot d'un client vivant.
		[]string{existing.ID.String(), e.name("Autre nom"), "", "0999999999", "Paris"},
		// Meme nom que la ligne suivante : le doublon interne au fichier.
		[]string{"", e.name("Nouveau"), "", "", ""},
		[]string{"", e.name("Nouveau"), "", "", ""},
	)

	up := e.upload(t, content)
	done := e.run(t, up.ID, up.SuggestedMapping, false)

	if done.CreatedN != 1 || done.SkippedN != 4 || done.ErrorN != 0 {
		t.Fatalf("created=%d skipped=%d error=%d, attendu 1, 4, 0", done.CreatedN, done.SkippedN, done.ErrorN)
	}

	want := map[int]string{
		2: usecase.ReasonDuplicateSiret,
		3: usecase.ReasonDuplicateName,
		4: usecase.ReasonAlreadyExists,
		6: usecase.ReasonAlreadyImported,
	}

	rows := e.rows(t, up.ID)
	for line, reason := range want {
		if rows[line].Outcome != usecase.ImportRowSkipped || rows[line].Reason != reason {
			t.Errorf("ligne %d : %s/%s, attendu skipped/%s", line, rows[line].Outcome, rows[line].Reason, reason)
		}
	}

	if rows[5].Outcome != usecase.ImportRowCreated {
		t.Errorf("ligne 5 : %s, attendu created", rows[5].Outcome)
	}

	// Jamais mis a jour.
	after, err := e.q.GetClientByID(ctx, existing.ID)
	if err != nil {
		t.Fatal(err)
	}

	if after.Phone != "0102030405" || after.City != "" || after.Name != existing.Name {
		t.Errorf("le client existant a ete modifie : %+v", after)
	}
}

func TestImportClients_InvalidRowDoesNotStopTheOthers(t *testing.T) {
	e := newImportEnv(t, 1<<20, 1000)

	content := csvOf(t, clientHeader,
		[]string{e.name("Bon 1"), "", "actif", ""},
		[]string{e.name("SIRET faux"), "12345678901234", "", ""},
		[]string{"", "", "", ""},
		[]string{e.name("Statut inconnu"), "", "peut-etre", ""},
		[]string{e.name("Bon 2"), "", "devis", ""},
	)

	up := e.upload(t, content)
	done := e.run(t, up.ID, up.SuggestedMapping, false)

	if done.CreatedN != 2 || done.ErrorN != 3 {
		t.Fatalf("created=%d error=%d, attendu 2 et 3", done.CreatedN, done.ErrorN)
	}

	rows := e.rows(t, up.ID)

	for _, line := range []int{3, 4, 5} {
		if rows[line].Outcome != usecase.ImportRowError || rows[line].Reason != usecase.ReasonInvalid {
			t.Errorf("ligne %d : %+v, attendu error/invalid", line, rows[line])
		}
	}

	if rows[2].Outcome != usecase.ImportRowCreated || rows[6].Outcome != usecase.ImportRowCreated {
		t.Errorf("les lignes valides n'ont pas ete creees : %+v / %+v", rows[2], rows[6])
	}

	if e.countClients(t) != 2 {
		t.Errorf("%d clients, attendu 2", e.countClients(t))
	}
}

func TestImportClients_DryRunWritesNothing(t *testing.T) {
	e := newImportEnv(t, 1<<20, 1000)
	ctx := context.Background()

	content := csvOf(t, clientHeader,
		[]string{e.name("Sim 1"), randomSiret(), "actif", ""},
		[]string{e.name("Sim 2"), "", "", ""},
		// Doublon interne : la simulation doit le voir comme le ferait l'import.
		[]string{e.name("Sim 2"), "", "", ""},
		[]string{e.name("Sim invalide"), "00", "", ""},
	)

	up := e.upload(t, content)

	var keysBefore int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM import_keys`).Scan(&keysBefore)

	sim := e.run(t, up.ID, up.SuggestedMapping, true)

	if !sim.DryRun || sim.CreatedN != 2 || sim.SkippedN != 1 || sim.ErrorN != 1 {
		t.Fatalf("simulation : dry=%v created=%d skipped=%d error=%d, attendu 2, 1, 1", sim.DryRun, sim.CreatedN, sim.SkippedN, sim.ErrorN)
	}

	if n := e.countClients(t); n != 0 {
		t.Fatalf("la simulation a ecrit %d clients", n)
	}

	var keysAfter int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM import_keys`).Scan(&keysAfter)

	if keysAfter != keysBefore {
		t.Fatalf("la simulation a ecrit des cles d'import (%d -> %d)", keysBefore, keysAfter)
	}

	// Le rapport de la simulation, lui, est la.
	if rows := e.rows(t, up.ID); len(rows) != 4 {
		t.Fatalf("%d lignes de rapport, attendu 4", len(rows))
	}

	// Le fichier est conserve : l'import reel part du meme depot.
	real := e.run(t, up.ID, up.SuggestedMapping, false)
	if real.DryRun || real.CreatedN != 2 || e.countClients(t) != 2 {
		t.Fatalf("import reel apres simulation : dry=%v created=%d clients=%d", real.DryRun, real.CreatedN, e.countClients(t))
	}

	// Apres un import reel, plus de relance possible.
	if _, err := e.svc.Run(ctx, up.ID, usecase.RunImportInput{Mapping: up.SuggestedMapping}); err == nil {
		t.Fatal("un import termine ne doit pas etre relance")
	}
}

func TestImportClients_UploadLimits(t *testing.T) {
	ctx := context.Background()

	small := newImportEnv(t, 64, 1000)
	if _, err := small.svc.Upload(ctx, nil, usecase.DataEntityClients, "x.csv", strings.NewReader(strings.Repeat("a;b\n", 100))); !isValidation(err, "file") {
		t.Errorf("fichier trop gros : %v", err)
	}

	few := newImportEnv(t, 1<<20, 2)
	if _, err := few.svc.Upload(ctx, nil, usecase.DataEntityClients, "x.csv", strings.NewReader("Nom\na\nb\nc\n")); !isValidation(err, "file") {
		t.Errorf("trop de lignes : %v", err)
	}

	e := newImportEnv(t, 1<<20, 10)

	for name, content := range map[string]string{
		"vide":           "",
		"sans donnees":   "Nom;Ville\n",
		"en-tete double": "Nom;Nom\na;b\n",
		"utf-16":         "\xff\xfeN\x00",
	} {
		if _, err := e.svc.Upload(ctx, nil, usecase.DataEntityClients, "x.csv", strings.NewReader(content)); !isValidation(err, "file") {
			t.Errorf("%s : %v", name, err)
		}
	}

	if _, err := e.svc.Upload(ctx, nil, "inconnue", "x.csv", strings.NewReader("Nom\na\n")); !isValidation(err, "entity") {
		t.Errorf("entite inconnue : %v", err)
	}

	// Aucun fichier orphelin apres ces refus.
	entries, _ := os.ReadDir(e.filesDir(t))
	if len(entries) != 0 {
		t.Errorf("%d fichiers laisses apres des refus", len(entries))
	}

	// Mapping : un champ obligatoire manquant ou une colonne absente.
	up := e.upload(t, "Nom;Ville\na;b\n")
	if _, err := e.svc.Run(ctx, up.ID, usecase.RunImportInput{Mapping: map[string]string{"city": "Ville"}}); !isValidation(err, "mapping.name") {
		t.Errorf("champ obligatoire non mappe : %v", err)
	}

	if _, err := e.svc.Run(ctx, up.ID, usecase.RunImportInput{Mapping: map[string]string{"name": "Fantome"}}); !isValidation(err, "mapping.name") {
		t.Errorf("colonne absente : %v", err)
	}
}

func (e *importEnv) filesDir(t *testing.T) string {
	t.Helper()

	return e.dir
}

func isValidation(err error, field string) bool {
	var derr *domain.Error

	return errors.As(err, &derr) && derr.Code == domain.ErrValidation.Code && derr.Details[field] != nil
}

func TestImportClients_Windows1252AndSemicolonlessFiles(t *testing.T) {
	e := newImportEnv(t, 1<<20, 1000)

	// Virgule, Windows-1252 (\xe9 = é), en-tetes a la mode d'un autre outil.
	content := "Client,Ville\r\n" + e.name("Caf\xe9") + ",Lille\r\n"

	up := e.upload(t, content)
	if up.Encoding != csvio.EncodingWindows1252 || up.Delimiter != "," || up.SuggestedMapping["name"] != "Client" {
		t.Fatalf("detection : %+v", up)
	}

	e.run(t, up.ID, up.SuggestedMapping, false)

	if _, err := e.q.GetClientByName(context.Background(), e.name("Café")); err != nil {
		t.Fatalf("client « Café » introuvable : %v", err)
	}
}

func TestImportClients_SavedMappingIsProposedAgain(t *testing.T) {
	e := newImportEnv(t, 1<<20, 1000)

	// Le mapping memorise est global : un passage precedent ne doit pas
	// fausser celui-ci.
	if _, err := e.pool.Exec(context.Background(), `DELETE FROM import_mappings WHERE entity = 'clients'`); err != nil {
		t.Fatal(err)
	}

	// Les en-tetes ne disent rien a l'auto-detection : seul le mapping
	// choisi a la main peut les relier.
	first := e.upload(t, "col1;col2\n"+e.name("M1")+";Lille\n")
	if len(first.SuggestedMapping) != 0 {
		t.Fatalf("aucune suggestion attendue : %v", first.SuggestedMapping)
	}

	mapping := map[string]string{"name": "col1", "city": "col2"}
	e.run(t, first.ID, mapping, false)

	second := e.upload(t, "col1;col2\n"+e.name("M2")+";Nancy\n")
	if second.SuggestedMapping["name"] != "col1" || second.SuggestedMapping["city"] != "col2" {
		t.Fatalf("mapping memorise non repropose : %v", second.SuggestedMapping)
	}
}

func TestImportClients_DeleteKeepsReplayProtection(t *testing.T) {
	e := newImportEnv(t, 1<<20, 1000)
	ctx := context.Background()

	content := csvOf(t, clientHeader, []string{e.name("K"), "", "", ""})

	up := e.upload(t, content)

	// Pas encore lance : on peut le supprimer, et son fichier part avec.
	if err := e.svc.Delete(ctx, up.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := e.svc.Get(ctx, up.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("import supprime encore lisible : %v", err)
	}

	again := e.upload(t, content)
	e.run(t, again.ID, again.SuggestedMapping, false)

	if err := e.svc.Delete(ctx, again.ID); err != nil {
		t.Fatal(err)
	}

	third := e.upload(t, content)
	done := e.run(t, third.ID, third.SuggestedMapping, false)

	if done.CreatedN != 0 || done.SkippedN != 1 {
		t.Fatalf("apres suppression de l'historique : created=%d skipped=%d", done.CreatedN, done.SkippedN)
	}
}

func TestExportThenImportRoundTrip(t *testing.T) {
	e := newImportEnv(t, 1<<20, 1000)
	ctx := context.Background()

	siret := randomSiret()

	seed := []db.CreateImportedClientParams{
		{Name: e.name("=SUM(1)+x"), Kind: usecase.ClientProfessionnel, LegalName: "Société \"Dupont; Fils\"", LegalForm: "SARL",
			Siret: siret, VatNumber: "FR12345678901", Status: "actif", Website: "https://exemple.fr",
			Phone: "+33 1 02 03 04 05", Address: "1 rue de la Paix\n75002 Paris", PostalCode: "75002", City: "Paris", Country: "FR"},
		{Name: e.name("Café Crème – œuvre"), Kind: usecase.ClientParticulier, Status: "veille", Phone: "-0102"},
		{Name: e.name("@mention, virgule"), Kind: usecase.ClientProfessionnel, Status: "perdu"},
	}

	originals := map[string]db.Client{}

	for _, p := range seed {
		c, err := e.q.CreateImportedClient(ctx, p)
		if err != nil {
			t.Fatal(err)
		}

		originals[c.Name] = c
	}

	// Export, puis on ne garde que nos lignes.
	var buf bytes.Buffer
	if err := e.export.Write(ctx, usecase.DataEntityClients, &buf); err != nil {
		t.Fatal(err)
	}

	if !bytes.HasPrefix(buf.Bytes(), []byte(csvio.BOM)) {
		t.Fatal("l'export doit commencer par le BOM")
	}

	reader, err := csvio.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}

	rows := [][]string{reader.Headers()}

	for {
		row, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatal(err)
		}

		if strings.HasPrefix(row.Cells[2], e.prefix) {
			rows = append(rows, row.Cells)
		}
	}

	if len(rows) != 1+len(seed) {
		t.Fatalf("%d lignes exportees pour nos clients, attendu %d", len(rows)-1, len(seed))
	}

	filtered := csvOf(t, rows...)

	// Base « videe » de ces clients : suppression pour de bon.
	for _, c := range originals {
		if _, err := e.pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, c.ID); err != nil {
			t.Fatal(err)
		}
	}

	up := e.upload(t, filtered)

	// Les en-tetes de l'export se reconnaissent tous, sans correspondance a la main.
	fields, _ := usecase.DataSpec(usecase.DataEntityClients)
	for _, f := range fields {
		if up.SuggestedMapping[f.Key] != f.Header {
			t.Errorf("champ %s : colonne %q, attendu %q", f.Key, up.SuggestedMapping[f.Key], f.Header)
		}
	}

	done := e.run(t, up.ID, up.SuggestedMapping, false)
	if done.CreatedN != len(seed) || done.SkippedN != 0 || done.ErrorN != 0 {
		t.Fatalf("created=%d skipped=%d error=%d (%v)", done.CreatedN, done.SkippedN, done.ErrorN, e.rows(t, up.ID))
	}

	for name, want := range originals {
		got, err := e.q.GetClientByName(ctx, name)
		if err != nil {
			t.Fatalf("%q introuvable : %v", name, err)
		}

		if got.Kind != want.Kind || got.LegalName != want.LegalName || got.LegalForm != want.LegalForm ||
			got.Siret != want.Siret || got.VatNumber != want.VatNumber || got.Status != want.Status ||
			got.Website != want.Website || got.Phone != want.Phone || got.Address != want.Address ||
			got.PostalCode != want.PostalCode || got.City != want.City || got.Country != want.Country {
			t.Errorf("%q :\n relu    %+v\n attendu %+v", name, got, want)
		}
	}

	// Le meme fichier une seconde fois : toujours rien de neuf.
	again := e.upload(t, filtered)
	if d := e.run(t, again.ID, again.SuggestedMapping, false); d.CreatedN != 0 || d.SkippedN != len(seed) {
		t.Fatalf("rejeu apres aller-retour : created=%d skipped=%d", d.CreatedN, d.SkippedN)
	}
}
