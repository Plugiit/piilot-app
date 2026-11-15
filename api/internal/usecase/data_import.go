package usecase

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/csvio"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// Etats d'un import.
const (
	ImportUploaded  = "uploaded"
	ImportQueued    = "queued"
	ImportRunning   = "running"
	ImportDone      = "done"
	ImportFailed    = "failed"
	ImportCancelled = "cancelled"
)

// Issues d'une ligne, et leurs codes. Le front branche sur ces codes, jamais
// sur le texte.
const (
	ImportRowCreated = "created"
	ImportRowSkipped = "skipped"
	ImportRowError   = "error"

	ReasonInvalid          = "invalid"
	ReasonAlreadyExists    = "already_exists"
	ReasonAlreadyImported  = "already_imported"
	ReasonDuplicateSiret   = "duplicate_siret"
	ReasonDuplicateName    = "duplicate_name"
	ReasonDuplicateEmail   = "duplicate_email"
	ReasonUnknownClient    = "unknown_client"
	ReasonUnknownProject   = "unknown_project"
	ReasonUnknownUser      = "unknown_user"
	importFailedInterrupt  = "interrupted"
	importFailedUnreadable = "file_unreadable"
	importFailedInternal   = "internal"
)

const (
	// ImportPreviewRows : lignes montrees avant de lancer.
	ImportPreviewRows = 10
	// ImportPageSize : taille des pages de l'historique et du rapport.
	ImportPageSize = 25
	// importBatchSize : lignes par transaction d'un vrai import.
	importBatchSize = 500
	// importMaxCell borne une cellule : un fichier n'a pas a porter un roman.
	importMaxCell  = 2000
	maxFileNameLen = 200
)

// ImportItem est un import, tel que l'historique le montre.
type ImportItem struct {
	ID         uuid.UUID         `json:"id"`
	Entity     string            `json:"entity"`
	Status     string            `json:"status"`
	DryRun     bool              `json:"dry_run"`
	FileName   string            `json:"file_name"`
	SizeBytes  int64             `json:"size_bytes"`
	Delimiter  string            `json:"delimiter"`
	Encoding   string            `json:"encoding"`
	Headers    []string          `json:"headers"`
	RowCount   int               `json:"row_count"`
	Mapping    map[string]string `json:"mapping"`
	CreatedN   int               `json:"created_n"`
	SkippedN   int               `json:"skipped_n"`
	ErrorN     int               `json:"error_n"`
	Error      string            `json:"error"`
	CreatedBy  *uuid.UUID        `json:"created_by"`
	StartedAt  *time.Time        `json:"started_at"`
	FinishedAt *time.Time        `json:"finished_at"`
	CreatedAt  time.Time         `json:"created_at"`
}

// ImportDetail ajoute ce que l'ecran de correspondance affiche.
type ImportDetail struct {
	ImportItem
	// Fields : les colonnes de Piilot pour cette entite.
	Fields []Field `json:"fields"`
	// Preview : les premieres lignes du fichier, tant qu'il est la.
	Preview [][]string `json:"preview"`
	// SuggestedMapping : champ -> en-tete du fichier. Le dernier mapping
	// utilise si les en-tetes concordent, sinon deduit des noms.
	SuggestedMapping map[string]string `json:"suggested_mapping"`
}

// ImportPage est une page de l'historique.
type ImportPage struct {
	Items    []ImportItem `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

// ImportRowItem est une ligne du rapport.
type ImportRowItem struct {
	Line     int        `json:"line"`
	Outcome  string     `json:"outcome"`
	Reason   string     `json:"reason"`
	Label    string     `json:"label"`
	EntityID *uuid.UUID `json:"entity_id"`
}

// ImportRowPage est une page du rapport.
type ImportRowPage struct {
	Items    []ImportRowItem `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// RunImportInput lance un import.
type RunImportInput struct {
	// Mapping : champ de Piilot -> en-tete du fichier.
	Mapping map[string]string
	// DryRun : tout est execute, rien n'est conserve.
	DryRun  bool
	ActorID *uuid.UUID
}

// importResult est ce qu'une ligne a donne.
type importResult struct {
	Outcome  string
	Reason   string
	Label    string
	EntityID *uuid.UUID
}

// entityImporter sait importer une ligne d'une entite, dans la transaction
// (ou le point de sauvegarde) qu'on lui passe. Il rend une erreur seulement
// pour un incident technique : une donnee refusee est un importResult.
type entityImporter interface {
	importRow(ctx context.Context, q *db.Queries, importID uuid.UUID, values map[string]string) (importResult, error)
}

// DataImportService depose, simule et execute les imports CSV.
type DataImportService struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	files     storage.Store
	maxBytes  int64
	maxRows   int
	log       *slog.Logger
	importers map[string]entityImporter
	fileTTL   time.Duration
}

// DefaultImportFileTTL : duree pendant laquelle un fichier d'import inutilise
// est garde (7 jours).
const DefaultImportFileTTL = 7 * 24 * time.Hour

// importPurgeBatch borne le nombre de fichiers retires a chaque passage.
const importPurgeBatch = 100

// SetFileTTL regle la duree de garde d'un fichier d'import inutilise (zero :
// DefaultImportFileTTL).
func (s *DataImportService) SetFileTTL(d time.Duration) { s.fileTTL = d }

// PurgeFiles supprime les fichiers d'import deposes ou simules depuis plus de
// la duree de garde sans avoir ete lances. Un fichier est donnee personnelle :
// sans cela il resterait indefiniment. Le fichier part avant que la ligne
// change d'etat ; un arret entre les deux est repris au passage suivant.
func (s *DataImportService) PurgeFiles(ctx context.Context) (int, error) {
	ttl := s.fileTTL
	if ttl <= 0 {
		ttl = DefaultImportFileTTL
	}

	due, err := s.q.ListExpiredImportFiles(ctx, db.ListExpiredImportFilesParams{
		Before: time.Now().Add(-ttl), Batch: importPurgeBatch,
	})
	if err != nil {
		return 0, fmt.Errorf("lecture des fichiers d'import expires : %w", err)
	}

	n := 0

	for _, d := range due {
		// On reclame la ligne d'abord : une confirmation concurrente ne la
		// trouve plus a l'etat « done » et ne lit jamais un fichier efface.
		fileKey, err := s.q.ClaimImportFile(ctx, d.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}

		if err != nil {
			return n, fmt.Errorf("expiration de l'import : %w", err)
		}

		if err := s.files.Remove(fileKey); err != nil {
			return n, fmt.Errorf("suppression du fichier d'import : %w", err)
		}

		n++
	}

	return n, nil
}

// NewDataImportService construit le service. maxBytes borne le fichier,
// maxRows ses lignes de donnees.
func NewDataImportService(pool *pgxpool.Pool, files storage.Store, maxBytes int64, maxRows int, log *slog.Logger) *DataImportService {
	return &DataImportService{
		pool: pool, q: db.New(pool), files: files, maxBytes: maxBytes, maxRows: maxRows, log: log,
		importers: map[string]entityImporter{
			DataEntityClients:     clientImporter{},
			DataEntityContacts:    contactImporter{},
			DataEntityProjects:    projectImporter{},
			DataEntityTimeEntries: timeEntryImporter{},
		},
	}
}

// Upload range le fichier, le lit une premiere fois (encodage, separateur,
// en-tetes, nombre de lignes) et propose une correspondance.
func (s *DataImportService) Upload(ctx context.Context, actor *uuid.UUID, entity, fileName string, content io.Reader) (ImportDetail, error) {
	if _, ok := s.importers[entity]; !ok {
		return ImportDetail{}, domain.ErrValidation.WithDetails(map[string]any{"entity": "Entité inconnue ou pas encore importable"})
	}

	// Le depot refuse un fichier vide avec une erreur technique : on le dit
	// avant, comme une donnee refusee.
	buffered := bufio.NewReader(content)
	if _, err := buffered.Peek(1); err != nil {
		if errors.Is(err, io.EOF) {
			return ImportDetail{}, domain.ErrValidation.WithDetails(map[string]any{"file": "Le fichier est vide"})
		}

		return ImportDetail{}, fmt.Errorf("lecture du fichier : %w", err)
	}

	key, size, err := s.files.Save(buffered, s.maxBytes)
	if err != nil {
		if errors.Is(err, storage.ErrTooLarge) {
			return ImportDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"file": fmt.Sprintf("Le fichier dépasse %d Mio", s.maxBytes>>20),
			})
		}

		return ImportDetail{}, fmt.Errorf("ecriture du fichier : %w", err)
	}

	keep := false

	defer func() {
		if !keep {
			_ = s.files.Remove(key)
		}
	}()

	reader, err := s.openCSV(key)
	if err != nil {
		return ImportDetail{}, err
	}

	headers := reader.Headers()
	if err := checkHeaders(headers); err != nil {
		return ImportDetail{}, err
	}

	rows := 0
	preview := make([][]string, 0, ImportPreviewRows)

	for {
		row, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		var parseErr bool

		if err != nil {
			if row.Line == 0 {
				return ImportDetail{}, domain.ErrValidation.WithDetails(map[string]any{"file": "Fichier illisible"}).WithCause(err)
			}

			parseErr = true
		}

		rows++
		if rows > s.maxRows {
			return ImportDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"file": fmt.Sprintf("Le fichier dépasse %d lignes", s.maxRows),
			})
		}

		if !parseErr && len(preview) < ImportPreviewRows {
			preview = append(preview, row.Cells)
		}
	}

	if rows == 0 {
		return ImportDetail{}, domain.ErrValidation.WithDetails(map[string]any{"file": "Le fichier ne contient aucune ligne de données"})
	}

	imp, err := s.q.CreateImport(ctx, db.CreateImportParams{
		Entity: entity, FileKey: key, FileName: cleanFileName(fileName), SizeBytes: size,
		Delimiter: string(reader.Delimiter()), Encoding: reader.Encoding(), Headers: headers,
		RowCount: int32(rows), CreatedBy: actor,
	})
	if err != nil {
		return ImportDetail{}, fmt.Errorf("creation de l'import : %w", err)
	}

	keep = true

	return s.detail(ctx, imp, preview)
}

// Get rend un import ; tant qu'il attend sa correspondance, avec l'apercu.
func (s *DataImportService) Get(ctx context.Context, id uuid.UUID) (ImportDetail, error) {
	imp, err := s.q.GetImport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ImportDetail{}, domain.ErrNotFound
		}

		return ImportDetail{}, fmt.Errorf("lecture de l'import : %w", err)
	}

	var preview [][]string

	if imp.FileKey != "" && (imp.Status == ImportUploaded || imp.Status == ImportDone) {
		preview = s.preview(imp.FileKey)
	}

	return s.detail(ctx, imp, preview)
}

// List rend l'historique, du plus recent au plus ancien.
func (s *DataImportService) List(ctx context.Context, page int) (ImportPage, error) {
	if page < 1 {
		page = 1
	}

	rows, err := s.q.ListImports(ctx, db.ListImportsParams{PageSize: ImportPageSize, PageOffset: int32((page - 1) * ImportPageSize)})
	if err != nil {
		return ImportPage{}, fmt.Errorf("liste des imports : %w", err)
	}

	total, err := s.q.CountImports(ctx)
	if err != nil {
		return ImportPage{}, fmt.Errorf("compte des imports : %w", err)
	}

	items := make([]ImportItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, importItemOf(r))
	}

	return ImportPage{Items: items, Total: total, Page: page, PageSize: ImportPageSize}, nil
}

// Rows rend une page du rapport, filtree par issue si on le demande.
func (s *DataImportService) Rows(ctx context.Context, id uuid.UUID, outcome string, page int) (ImportRowPage, error) {
	if outcome != "" && !slices.Contains([]string{ImportRowCreated, ImportRowSkipped, ImportRowError}, outcome) {
		return ImportRowPage{}, domain.ErrValidation.WithDetails(map[string]any{"outcome": "Issue inconnue"})
	}

	if _, err := s.q.GetImport(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ImportRowPage{}, domain.ErrNotFound
		}

		return ImportRowPage{}, fmt.Errorf("lecture de l'import : %w", err)
	}

	if page < 1 {
		page = 1
	}

	var filter *string
	if outcome != "" {
		filter = &outcome
	}

	rows, err := s.q.ListImportRows(ctx, db.ListImportRowsParams{
		ImportID: id, Outcome: filter, PageSize: ImportPageSize, PageOffset: int32((page - 1) * ImportPageSize),
	})
	if err != nil {
		return ImportRowPage{}, fmt.Errorf("lecture du rapport : %w", err)
	}

	total, err := s.q.CountImportRows(ctx, db.CountImportRowsParams{ImportID: id, Outcome: filter})
	if err != nil {
		return ImportRowPage{}, fmt.Errorf("compte du rapport : %w", err)
	}

	items := make([]ImportRowItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, ImportRowItem{Line: int(r.Line), Outcome: r.Outcome, Reason: r.Reason, Label: r.Label, EntityID: r.EntityID})
	}

	return ImportRowPage{Items: items, Total: total, Page: page, PageSize: ImportPageSize}, nil
}

// Run met l'import en file. Le traitement est fait par la tache de fond.
func (s *DataImportService) Run(ctx context.Context, id uuid.UUID, in RunImportInput) (ImportItem, error) {
	imp, err := s.q.GetImport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ImportItem{}, domain.ErrNotFound
		}

		return ImportItem{}, fmt.Errorf("lecture de l'import : %w", err)
	}

	fields, _ := DataSpec(imp.Entity)

	if errs := validateMapping(fields, imp.Headers, in.Mapping); len(errs) > 0 {
		return ImportItem{}, domain.ErrValidation.WithDetails(errs)
	}

	mapping, err := json.Marshal(in.Mapping)
	if err != nil {
		return ImportItem{}, fmt.Errorf("encodage du mapping : %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ImportItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	queued, err := q.QueueImport(ctx, db.QueueImportParams{ID: id, DryRun: in.DryRun, Mapping: mapping})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ImportItem{}, domain.ErrConflict.WithMessage("Cet import ne peut plus être lancé")
		}

		return ImportItem{}, fmt.Errorf("mise en file : %w", err)
	}

	if err := q.DeleteImportRows(ctx, id); err != nil {
		return ImportItem{}, fmt.Errorf("effacement du rapport precedent : %w", err)
	}

	if err := q.UpsertImportMapping(ctx, db.UpsertImportMappingParams{Entity: imp.Entity, Mapping: mapping, UpdatedBy: in.ActorID}); err != nil {
		return ImportItem{}, fmt.Errorf("memorisation du mapping : %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ImportItem{}, fmt.Errorf("validation : %w", err)
	}

	return importItemOf(queued), nil
}

// Delete retire un import de l'historique, avec son rapport et son fichier.
// Les cles d'import restent : rejouer le fichier ne recree rien.
func (s *DataImportService) Delete(ctx context.Context, id uuid.UUID) error {
	imp, err := s.q.GetImport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}

		return fmt.Errorf("lecture de l'import : %w", err)
	}

	if imp.Status == ImportQueued || imp.Status == ImportRunning {
		return domain.ErrConflict.WithMessage("Cet import est en cours")
	}

	if err := s.q.DeleteImport(ctx, id); err != nil {
		return fmt.Errorf("suppression de l'import : %w", err)
	}

	if imp.FileKey != "" {
		if err := s.files.Remove(imp.FileKey); err != nil {
			s.log.Warn("suppression du fichier d'import", "import", id, "error", err)
		}
	}

	return nil
}

// Process traite le prochain import en file, s'il y en a un, et dit si l'on a
// travaille. Appele par la tache de fond ; deux instances ne prennent jamais
// le meme import.
func (s *DataImportService) Process(ctx context.Context) (bool, error) {
	stale, err := s.q.FailStaleImports(ctx)
	if err != nil {
		return false, fmt.Errorf("imports abandonnes : %w", err)
	}

	for _, st := range stale {
		if !st.DryRun && st.FileKey != "" {
			_ = s.files.Remove(st.FileKey)
		}
	}

	if _, err := s.PurgeFiles(ctx); err != nil {
		s.log.Warn("purge des fichiers d'import", "error", err)
	}

	imp, err := s.q.ClaimQueuedImport(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("reservation d'un import : %w", err)
	}

	code, procErr := s.process(ctx, imp)

	// Le contexte peut etre annule (arret, delai) : le constat se fait quand
	// meme, sinon l'import resterait « en cours » jusqu'au prochain nettoyage.
	fin := context.WithoutCancel(ctx)

	status, fileKey := ImportDone, imp.FileKey
	if procErr != nil {
		status = ImportFailed
	}

	// Une simulation reussie garde son fichier : l'import reel le reutilise.
	if procErr != nil || !imp.DryRun {
		if fileKey != "" {
			if err := s.files.Remove(fileKey); err != nil {
				s.log.Warn("suppression du fichier d'import", "import", imp.ID, "error", err)
			}
		}

		fileKey = ""
	}

	if err := s.q.FinishImport(fin, db.FinishImportParams{
		ID: imp.ID, Status: status, Error: code, FileKey: fileKey,
	}); err != nil {
		return true, fmt.Errorf("cloture de l'import : %w", err)
	}

	if procErr != nil {
		return true, fmt.Errorf("import %s : %w", imp.ID, procErr)
	}

	return true, nil
}

// process execute l'import. Il rend le code d'echec a ecrire sur l'import.
//
// Simulation : une seule transaction sur tout le fichier, annulee a la fin,
// pour que les doublons internes au fichier se voient comme en vrai. Import
// reel : une transaction par lot de 500 lignes.
//
// Limite connue, acceptee : la simulation garde ses ecritures (et donc les
// verrous d'unicite pris par ses INSERT : nom, SIRET, e-mail) jusqu'au
// ROLLBACK final. Pendant une simulation longue, une saisie concurrente d'un
// meme client, contact ou projet attend la fin de la simulation (ou echoue au
// delai de verrou). Le fichier est borne par IMPORT_MAX_ROWS et la simulation
// n'a lieu que sur demande d'un administrateur ; le vrai import, lui, ne tient
// ses verrous que le temps d'un lot. On garde donc ce comportement : il est
// ce qui fait voir les doublons internes au fichier, comme en vrai.
func (s *DataImportService) process(ctx context.Context, imp db.Import) (string, error) {
	importer, ok := s.importers[imp.Entity]
	if !ok {
		return importFailedInternal, fmt.Errorf("entite %q sans importeur", imp.Entity)
	}

	fields, _ := DataSpec(imp.Entity)

	var mapping map[string]string
	if err := json.Unmarshal(imp.Mapping, &mapping); err != nil {
		return importFailedInternal, fmt.Errorf("mapping illisible : %w", err)
	}

	reader, err := s.openCSV(imp.FileKey)
	if err != nil {
		return importFailedUnreadable, err
	}

	index := columnIndex(fields, reader.Headers(), mapping)

	var dryTx pgx.Tx

	if imp.DryRun {
		if dryTx, err = s.pool.Begin(ctx); err != nil {
			return importFailedInternal, fmt.Errorf("ouverture de la transaction : %w", err)
		}

		defer func() { _ = dryTx.Rollback(context.WithoutCancel(ctx)) }()
	}

	for {
		if err := ctx.Err(); err != nil {
			return importFailedInterrupt, err
		}

		batch, done, err := readBatch(reader, importBatchSize)
		if err != nil {
			return importFailedUnreadable, err
		}

		if len(batch) > 0 {
			tx := dryTx
			if tx == nil {
				if tx, err = s.pool.Begin(ctx); err != nil {
					return importFailedInternal, fmt.Errorf("ouverture de la transaction : %w", err)
				}
			}

			if err := s.runBatch(ctx, tx, imp, importer, index, batch, dryTx == nil); err != nil {
				if dryTx == nil {
					_ = tx.Rollback(context.WithoutCancel(ctx))
				}

				return importFailedInternal, err
			}

			if dryTx == nil {
				if err := tx.Commit(ctx); err != nil {
					return importFailedInternal, fmt.Errorf("validation du lot : %w", err)
				}
			}

			_ = s.q.TouchImport(ctx, imp.ID)
		}

		if done {
			return "", nil
		}
	}
}

// batchRow est une ligne lue : ses cellules, ou l'erreur de lecture.
type batchRow struct {
	line    int
	cells   []string
	readErr error
}

// readBatch lit jusqu'a n lignes. done : fin du fichier atteinte.
func readBatch(r *csvio.Reader, n int) (batch []batchRow, done bool, err error) {
	for len(batch) < n {
		row, err := r.Next()
		if errors.Is(err, io.EOF) {
			return batch, true, nil
		}

		if err != nil {
			if row.Line == 0 {
				return nil, false, fmt.Errorf("lecture du fichier : %w", err)
			}

			batch = append(batch, batchRow{line: row.Line, readErr: err})

			continue
		}

		batch = append(batch, batchRow{line: row.Line, cells: row.Cells})
	}

	return batch, false, nil
}

// runBatch traite un lot dans tx, une ligne par point de sauvegarde : une
// ligne refusee n'abime pas les autres. Le rapport s'ecrit dans tx pour un
// import reel (atomique avec les donnees), hors tx pour une simulation (les
// donnees sont annulees, pas le rapport).
func (s *DataImportService) runBatch(
	ctx context.Context, tx pgx.Tx, imp db.Import, importer entityImporter,
	index map[string]int, batch []batchRow, reportInTx bool,
) error {
	type reportRow struct {
		Line     int    `json:"line"`
		Outcome  string `json:"outcome"`
		Reason   string `json:"reason"`
		Label    string `json:"label"`
		EntityID string `json:"entity_id"`
	}

	report := make([]reportRow, 0, len(batch))

	for _, row := range batch {
		res, err := s.runRow(ctx, tx, imp.ID, importer, index, row)
		if err != nil {
			return fmt.Errorf("ligne %d : %w", row.line, err)
		}

		r := reportRow{Line: row.line, Outcome: res.Outcome, Reason: res.Reason, Label: truncateRunes(res.Label, 200)}
		if res.EntityID != nil {
			r.EntityID = res.EntityID.String()
		}

		report = append(report, r)
	}

	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("encodage du rapport : %w", err)
	}

	q := s.q
	if reportInTx {
		q = s.q.WithTx(tx)
	}

	if err := q.InsertImportRows(ctx, db.InsertImportRowsParams{ImportID: imp.ID, Rows: payload}); err != nil {
		return fmt.Errorf("ecriture du rapport : %w", err)
	}

	return nil
}

func (s *DataImportService) runRow(
	ctx context.Context, tx pgx.Tx, importID uuid.UUID, importer entityImporter,
	index map[string]int, row batchRow,
) (importResult, error) {
	if row.readErr != nil {
		return importResult{Outcome: ImportRowError, Reason: ReasonInvalid, Label: "Ligne illisible"}, nil
	}

	values := make(map[string]string, len(index))

	for key, i := range index {
		if i < len(row.cells) {
			values[key] = row.cells[i]
		}
	}

	for key, v := range values {
		if len(v) > importMaxCell {
			return importResult{Outcome: ImportRowError, Reason: ReasonInvalid, Label: fmt.Sprintf("%s : valeur trop longue", key)}, nil
		}
	}

	sp, err := tx.Begin(ctx)
	if err != nil {
		return importResult{}, fmt.Errorf("point de sauvegarde : %w", err)
	}

	res, err := importer.importRow(ctx, db.New(sp), importID, values)
	if err != nil {
		_ = sp.Rollback(ctx)

		// Deux imports simultanes ont pu creer le meme nom entre notre
		// verification et notre ecriture : c'est un doublon, pas un incident.
		if isUniqueViolation(err) {
			return importResult{Outcome: ImportRowSkipped, Reason: uniqueViolationReason(err), Label: res.Label}, nil
		}

		return importResult{}, err
	}

	if res.Outcome != ImportRowCreated {
		return res, sp.Rollback(ctx)
	}

	if err := sp.Commit(ctx); err != nil {
		return importResult{}, fmt.Errorf("liberation du point de sauvegarde : %w", err)
	}

	return res, nil
}

// uniqueViolationReason donne le motif de doublon selon la contrainte violee.
// Une contrainte inconnue se range sous le motif generique du nom.
func uniqueViolationReason(err error) string {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return ReasonDuplicateName
	}

	switch pgErr.ConstraintName {
	case "contacts_email_unique":
		return ReasonDuplicateEmail
	case "import_keys_pkey":
		return ReasonAlreadyImported
	default:
		return ReasonDuplicateName
	}
}

// openCSV ouvre un fichier range et en lit les en-tetes.
func (s *DataImportService) openCSV(key string) (*csvio.Reader, error) {
	f, err := s.files.Open(key)
	if err != nil {
		return nil, fmt.Errorf("ouverture du fichier : %w", err)
	}
	defer func() { _ = f.Close() }()

	reader, err := csvio.NewReader(f)
	if err != nil {
		if errors.Is(err, csvio.ErrEmpty) || errors.Is(err, csvio.ErrUnsupportedEncoding) {
			msg := "Le fichier est vide"
			if errors.Is(err, csvio.ErrUnsupportedEncoding) {
				msg = "Encodage non pris en charge : enregistrez le fichier en CSV UTF-8"
			}

			return nil, domain.ErrValidation.WithDetails(map[string]any{"file": msg})
		}

		return nil, domain.ErrValidation.WithDetails(map[string]any{"file": "Fichier CSV illisible"}).WithCause(err)
	}

	return reader, nil
}

// preview relit les premieres lignes ; vide si le fichier n'est plus lisible.
func (s *DataImportService) preview(key string) [][]string {
	reader, err := s.openCSV(key)
	if err != nil {
		return nil
	}

	out := make([][]string, 0, ImportPreviewRows)

	for len(out) < ImportPreviewRows {
		row, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			continue
		}

		out = append(out, row.Cells)
	}

	return out
}

func (s *DataImportService) detail(ctx context.Context, imp db.Import, preview [][]string) (ImportDetail, error) {
	fields, _ := DataSpec(imp.Entity)

	d := ImportDetail{ImportItem: importItemOf(imp), Fields: fields, Preview: preview}
	if d.Preview == nil {
		d.Preview = [][]string{}
	}

	suggested := suggestMapping(fields, imp.Headers)

	// Le dernier mapping vaut quand ses colonnes sont toutes dans ce fichier
	// et qu'il relie au moins autant de champs que la deduction par les noms :
	// un vieux mapping partiel ne doit pas masquer un export complet.
	if saved, err := s.q.GetImportMapping(ctx, imp.Entity); err == nil {
		var m map[string]string
		if json.Unmarshal(saved.Mapping, &m) == nil && mappingFits(fields, imp.Headers, m) && len(m) >= len(suggested) {
			suggested = m
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ImportDetail{}, fmt.Errorf("lecture du mapping memorise : %w", err)
	}

	d.SuggestedMapping = suggested

	return d, nil
}

func importItemOf(i db.Import) ImportItem {
	mapping := map[string]string{}
	_ = json.Unmarshal(i.Mapping, &mapping)

	headers := i.Headers
	if headers == nil {
		headers = []string{}
	}

	return ImportItem{
		ID: i.ID, Entity: i.Entity, Status: i.Status, DryRun: i.DryRun, FileName: i.FileName,
		SizeBytes: i.SizeBytes, Delimiter: i.Delimiter, Encoding: i.Encoding, Headers: headers,
		RowCount: int(i.RowCount), Mapping: mapping, CreatedN: int(i.CreatedN), SkippedN: int(i.SkippedN),
		ErrorN: int(i.ErrorN), Error: i.Error, CreatedBy: i.CreatedBy,
		StartedAt: i.StartedAt, FinishedAt: i.FinishedAt, CreatedAt: i.CreatedAt,
	}
}

// checkHeaders refuse les en-tetes en double : deux colonnes « Nom » rendent
// toute correspondance ambigue.
func checkHeaders(headers []string) error {
	seen := map[string]bool{}

	for _, h := range headers {
		n := csvio.NormalizeHeader(h)
		if n == "" {
			continue
		}

		if seen[n] {
			return domain.ErrValidation.WithDetails(map[string]any{
				"file": fmt.Sprintf("L'en-tête « %s » apparaît deux fois", h),
			})
		}

		seen[n] = true
	}

	return nil
}

// suggestMapping devine champ -> en-tete : d'abord l'en-tete exact de
// l'export, puis les alias. Une colonne ne sert qu'a un champ.
func suggestMapping(fields []Field, headers []string) map[string]string {
	byNorm := map[string]string{}

	for _, h := range headers {
		if n := csvio.NormalizeHeader(h); n != "" {
			if _, dup := byNorm[n]; !dup {
				byNorm[n] = h
			}
		}
	}

	out := map[string]string{}
	used := map[string]bool{}

	pick := func(f Field, names []string) bool {
		for _, name := range names {
			if h, ok := byNorm[csvio.NormalizeHeader(name)]; ok && !used[h] {
				out[f.Key] = h
				used[h] = true

				return true
			}
		}

		return false
	}

	// Deux passes : les en-tetes exacts avant les alias, pour qu'un alias
	// d'un champ ne vole pas la colonne exacte d'un autre.
	for _, f := range fields {
		pick(f, []string{f.Header})
	}

	for _, f := range fields {
		if _, done := out[f.Key]; !done {
			pick(f, f.Aliases)
		}
	}

	return out
}

// mappingFits : le mapping memorise ne vaut que si chaque colonne qu'il cite
// est dans ce fichier et chaque champ existe encore.
func mappingFits(fields []Field, headers []string, mapping map[string]string) bool {
	if len(mapping) == 0 {
		return false
	}

	return len(validateMapping(fields, headers, mapping)) == 0
}

// validateMapping verifie un mapping champ -> en-tete.
func validateMapping(fields []Field, headers []string, mapping map[string]string) map[string]any {
	errs := map[string]any{}
	known := map[string]Field{}

	for _, f := range fields {
		known[f.Key] = f
	}

	for key, header := range mapping {
		if _, ok := known[key]; !ok {
			errs["mapping."+key] = "Champ inconnu"

			continue
		}

		if header != "" && !slices.Contains(headers, header) {
			errs["mapping."+key] = "Colonne absente du fichier"
		}
	}

	for _, f := range fields {
		if f.Required && mapping[f.Key] == "" {
			errs["mapping."+f.Key] = "Colonne obligatoire : « " + f.Header + " »"
		}
	}

	return errs
}

// columnIndex donne l'indice de colonne de chaque champ mappe.
func columnIndex(fields []Field, headers []string, mapping map[string]string) map[string]int {
	index := map[string]int{}

	for _, f := range fields {
		h := mapping[f.Key]
		if h == "" {
			continue
		}

		if i := slices.Index(headers, h); i >= 0 {
			index[f.Key] = i
		}
	}

	return index
}

func cleanFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" {
		name = ""
	}

	return truncateRunes(name, maxFileNameLen)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}

	return string(r[:n])
}
