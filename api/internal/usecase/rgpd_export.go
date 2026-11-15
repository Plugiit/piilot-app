package usecase

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// Sujets d'un export RGPD.
const (
	RgpdSubjectClient = "client"
	RgpdSubjectUser   = "user"
)

// Etats d'un export.
const (
	RgpdExportQueued  = "queued"
	RgpdExportRunning = "running"
	RgpdExportReady   = "ready"
	RgpdExportFailed  = "failed"
	RgpdExportExpired = "expired"
)

// Codes d'echec ecrits sur l'export (le detail va dans les logs).
const (
	rgpdFailTooLarge    = "too_large"
	rgpdFailInternal    = "internal"
	rgpdFailInterrupted = "interrupted"
)

// rgpdIncludeInternalNotes decide si l'archive d'un CLIENT contient les notes
// internes des tickets (messages `is_internal`), ecrites par l'equipe et jamais
// montrees au client. Decision en attente : exclues par defaut. C'est le seul
// endroit a changer pour les inclure.
const rgpdIncludeInternalNotes = false

const (
	// rgpdPageSize : lignes lues par requete, pagination par cle.
	rgpdPageSize = 1000
	// RgpdPageSize : taille des pages de l'historique des exports.
	RgpdPageSize = 25
	// rgpdManifestFormat : version du format de l'archive.
	rgpdManifestFormat = 1
	maxZipNameLen      = 200
)

// RgpdExportItem est un export, tel que l'historique le montre. La cle du
// fichier ne sort jamais.
type RgpdExportItem struct {
	ID           uuid.UUID  `json:"id"`
	SubjectType  string     `json:"subject_type"`
	SubjectID    uuid.UUID  `json:"subject_id"`
	SubjectLabel string     `json:"subject_label"`
	Status       string     `json:"status"`
	SizeBytes    int64      `json:"size_bytes"`
	RequestedBy  *uuid.UUID `json:"requested_by"`
	ReadyAt      *time.Time `json:"ready_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
	Error        string     `json:"error"`
	CreatedAt    time.Time  `json:"created_at"`
}

// RgpdExportPage est une page de l'historique.
type RgpdExportPage struct {
	Items    []RgpdExportItem `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// RgpdDiscord est ce que l'export attend de la liaison Discord.
type RgpdDiscord interface {
	Export(ctx context.Context, userID uuid.UUID) (*DiscordLinkExport, error)
}

// RgpdExportService demande, construit et sert les archives RGPD.
type RgpdExportService struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	files    storage.Store
	discord  RgpdDiscord
	ttl      time.Duration
	maxBytes int64
	log      *slog.Logger

	// afterBuild, nil en production, laisse un test agir entre la fin de la
	// construction de l'archive et l'ecriture du resultat.
	afterBuild func()
}

// NewRgpdExportService construit le service. ttl : duree de vie d'une archive
// prete ; maxBytes borne sa taille.
func NewRgpdExportService(pool *pgxpool.Pool, files storage.Store, discord RgpdDiscord, ttl time.Duration, maxBytes int64, log *slog.Logger) *RgpdExportService {
	return &RgpdExportService{pool: pool, q: db.New(pool), files: files, discord: discord, ttl: ttl, maxBytes: maxBytes, log: log}
}

func rgpdItemOf(e db.RgpdExport) RgpdExportItem {
	return RgpdExportItem{
		ID: e.ID, SubjectType: e.SubjectType, SubjectID: e.SubjectID, SubjectLabel: e.SubjectLabel,
		Status: e.Status, SizeBytes: e.SizeBytes, RequestedBy: e.RequestedBy, ReadyAt: e.ReadyAt,
		ExpiresAt: e.ExpiresAt, Error: e.Error, CreatedAt: e.CreatedAt,
	}
}

// Request met un export en file. Un export deja en attente ou en cours pour le
// meme sujet est rendu tel quel, sans en creer un second.
func (s *RgpdExportService) Request(ctx context.Context, actor *uuid.UUID, subjectType string, subjectID uuid.UUID) (RgpdExportItem, error) {
	var label string

	switch subjectType {
	case RgpdSubjectClient:
		name, err := s.q.RgpdClientLabel(ctx, subjectID)
		if err != nil {
			return RgpdExportItem{}, s.subjectErr(err)
		}

		label = name
	case RgpdSubjectUser:
		u, err := s.q.RgpdUserLabel(ctx, subjectID)
		if err != nil {
			return RgpdExportItem{}, s.subjectErr(err)
		}

		if label = u.Name; label == "" {
			label = u.Email
		}
	default:
		return RgpdExportItem{}, domain.ErrValidation.WithDetails(map[string]any{"subject_type": "Type inconnu : client ou user"})
	}

	switch active, err := s.q.GetActiveRgpdExport(ctx, db.GetActiveRgpdExportParams{SubjectType: subjectType, SubjectID: subjectID}); {
	case err == nil:
		return rgpdItemOf(active), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return RgpdExportItem{}, fmt.Errorf("lecture des exports en cours : %w", err)
	}

	e, err := s.q.CreateRgpdExport(ctx, db.CreateRgpdExportParams{
		SubjectType: subjectType, SubjectID: subjectID, SubjectLabel: truncateRunes(label, 200), RequestedBy: actor,
	})
	if err != nil {
		// Deux demandes simultanees : l'index unique n'en laisse passer qu'une,
		// l'autre retrouve l'export de la premiere.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			active, gerr := s.q.GetActiveRgpdExport(ctx, db.GetActiveRgpdExportParams{SubjectType: subjectType, SubjectID: subjectID})
			if gerr != nil {
				return RgpdExportItem{}, fmt.Errorf("lecture de l'export concurrent : %w", gerr)
			}

			return rgpdItemOf(active), nil
		}

		return RgpdExportItem{}, fmt.Errorf("creation de l'export : %w", err)
	}

	return rgpdItemOf(e), nil
}

func (s *RgpdExportService) subjectErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound.WithDetails(map[string]any{"subject_id": "Sujet introuvable"})
	}

	return fmt.Errorf("lecture du sujet : %w", err)
}

// List rend une page de l'historique, les plus recents d'abord.
func (s *RgpdExportService) List(ctx context.Context, page int) (RgpdExportPage, error) {
	if page < 1 {
		page = 1
	}

	rows, err := s.q.ListRgpdExports(ctx, db.ListRgpdExportsParams{PageSize: RgpdPageSize, PageOffset: int32((page - 1) * RgpdPageSize)})
	if err != nil {
		return RgpdExportPage{}, fmt.Errorf("lecture des exports : %w", err)
	}

	total, err := s.q.CountRgpdExports(ctx)
	if err != nil {
		return RgpdExportPage{}, fmt.Errorf("compte des exports : %w", err)
	}

	items := make([]RgpdExportItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, rgpdItemOf(r))
	}

	return RgpdExportPage{Items: items, Total: total, Page: page, PageSize: RgpdPageSize}, nil
}

// Open ouvre l'archive d'un export pret et non expire.
func (s *RgpdExportService) Open(ctx context.Context, id uuid.UUID) (RgpdExportItem, io.ReadCloser, error) {
	e, err := s.q.GetRgpdExport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return RgpdExportItem{}, nil, domain.ErrNotFound
	}

	if err != nil {
		return RgpdExportItem{}, nil, fmt.Errorf("lecture de l'export : %w", err)
	}

	if e.Status != RgpdExportReady || e.FileKey == "" || (e.ExpiresAt != nil && !e.ExpiresAt.After(time.Now())) {
		return RgpdExportItem{}, nil, domain.ErrConflict.WithMessage("Cette archive n'est pas disponible (en cours, échouée ou expirée)")
	}

	f, err := s.files.Open(e.FileKey)
	if err != nil {
		return RgpdExportItem{}, nil, fmt.Errorf("ouverture de l'archive : %w", err)
	}

	return rgpdItemOf(e), f, nil
}

// Process construit le prochain export en file. Il rend vrai s'il y en avait
// un. Chaque export est reserve par une seule instruction SKIP LOCKED : deux
// instances ne construisent jamais la meme archive.
func (s *RgpdExportService) Process(ctx context.Context) (bool, error) {
	e, err := s.q.ClaimQueuedRgpdExport(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("reservation d'un export : %w", err)
	}

	// L'echec d'un export n'est pas celui du job : il s'ecrit sur la ligne.
	key, size, err := s.build(ctx, e)
	if err != nil {
		code := rgpdFailInternal
		if errors.Is(err, storage.ErrTooLarge) {
			code = rgpdFailTooLarge
		}

		s.log.Warn("export rgpd", "id", e.ID, "error", err)

		if ferr := s.q.FailRgpdExport(context.WithoutCancel(ctx), db.FailRgpdExportParams{ID: e.ID, Error: code}); ferr != nil {
			return true, fmt.Errorf("ecriture de l'echec : %w", ferr)
		}

		return true, nil
	}

	if s.afterBuild != nil {
		s.afterBuild()
	}

	n, err := s.q.FinishRgpdExport(context.WithoutCancel(ctx), db.FinishRgpdExportParams{
		ID: e.ID, FileKey: key, SizeBytes: size, ExpiresAt: ptrTime(time.Now().Add(s.ttl)),
	})
	if err != nil {
		_ = s.files.Remove(key)

		return true, fmt.Errorf("ecriture du resultat : %w", err)
	}

	// Aucune ligne : l'export n'est plus « en cours » (le sujet a ete efface,
	// ou la construction a ete jugee abandonnee). Personne ne saura que
	// l'archive existe : elle contient des donnees personnelles, on la retire.
	if n == 0 {
		if err := s.files.Remove(key); err != nil {
			s.log.Warn("export rgpd : archive orpheline non supprimee", "id", e.ID, "error", err)
		}
	}

	return true, nil
}

func ptrTime(t time.Time) *time.Time { return &t }

// Purge supprime les archives expirees et marque « failed » les constructions
// abandonnees. Le fichier part avant que la ligne change d'etat : un arret
// entre les deux est repris au tour suivant.
func (s *RgpdExportService) Purge(ctx context.Context) (int, error) {
	if _, err := s.q.FailStaleRgpdExports(ctx); err != nil {
		return 0, fmt.Errorf("export interrompu : %w", err)
	}

	due, err := s.q.ListExpiredRgpdExports(ctx)
	if err != nil {
		return 0, fmt.Errorf("lecture des archives expirees : %w", err)
	}

	n := 0

	for _, d := range due {
		if d.FileKey != "" {
			if err := s.files.Remove(d.FileKey); err != nil {
				return n, fmt.Errorf("suppression de l'archive : %w", err)
			}
		}

		if err := s.q.MarkRgpdExportExpired(ctx, d.ID); err != nil {
			return n, fmt.Errorf("expiration de l'export : %w", err)
		}

		n++
	}

	return n, nil
}

// build ecrit l'archive en flux : zip.Writer -> io.Pipe -> storage.Save. Rien
// n'est garde en memoire au-dela d'une page.
func (s *RgpdExportService) build(ctx context.Context, e db.RgpdExport) (string, int64, error) {
	pr, pw := io.Pipe()
	done := make(chan error, 1)

	go func() {
		err := s.writeArchive(ctx, pw, e)
		_ = pw.CloseWithError(err) // nil : fin de flux normale

		done <- err
	}()

	key, size, err := s.files.Save(pr, s.maxBytes)
	if err != nil {
		// Debloque l'ecrivain s'il attend encore un lecteur.
		_ = pr.CloseWithError(err)
		<-done

		return "", 0, err
	}

	if werr := <-done; werr != nil {
		_ = s.files.Remove(key)

		return "", 0, werr
	}

	return key, size, nil
}

// rgpdRow est une ligne de section : sa cle de pagination et son JSON.
type rgpdRow struct {
	ID   uuid.UUID `json:"id"`
	Data []byte    `json:"data"`
}

// rgpdFile est un fichier a ajouter a l'archive.
type rgpdFile struct {
	ID         uuid.UUID
	Filename   string
	StorageKey string
}

type rgpdSection struct {
	name string
	page func(ctx context.Context, after uuid.UUID) ([]rgpdRow, error)
}

// rowsOf convertit les lignes d'une requete sqlc (meme forme : id, data).
func rowsOf[T ~struct {
	ID   uuid.UUID `json:"id"`
	Data []byte    `json:"data"`
}](rows []T, err error) ([]rgpdRow, error) {
	if err != nil {
		return nil, err
	}

	out := make([]rgpdRow, len(rows))
	for i, r := range rows {
		out[i] = rgpdRow(r)
	}

	return out, nil
}

// sectionsOf dit quelles sections et quels fichiers composent l'archive d'un
// sujet. C'est ici, et nulle part ailleurs, que l'on decide ce qui en fait
// partie.
func (s *RgpdExportService) sectionsOf(subjectType string, id uuid.UUID) ([]rgpdSection, func(ctx context.Context, after uuid.UUID) ([]rgpdFile, error), error) {
	q := s.q
	n := int32(rgpdPageSize)

	switch subjectType {
	case RgpdSubjectUser:
		return []rgpdSection{
			{"profile", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserProfile(ctx, db.RgpdUserProfileParams{Subject: id, After: a, PageSize: n}))
			}},
			{"sessions", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserSessions(ctx, db.RgpdUserSessionsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"time_entries", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserTimeEntries(ctx, db.RgpdUserTimeEntriesParams{Subject: id, After: a, PageSize: n}))
			}},
			{"tasks", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserTasks(ctx, db.RgpdUserTasksParams{Subject: id, After: a, PageSize: n}))
			}},
			{"task_comments", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserTaskComments(ctx, db.RgpdUserTaskCommentsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"ticket_messages", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserTicketMessages(ctx, db.RgpdUserTicketMessagesParams{Subject: id, After: a, PageSize: n}))
			}},
			{"notifications", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserNotifications(ctx, db.RgpdUserNotificationsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"claude_connections", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserOauthGrants(ctx, db.RgpdUserOauthGrantsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"audit", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserAudit(ctx, db.RgpdUserAuditParams{Subject: id, After: a, PageSize: n}))
			}},
			{"attachments", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdUserAttachments(ctx, db.RgpdUserAttachmentsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"discord", s.discordSection(id)},
		}, func(ctx context.Context, a uuid.UUID) ([]rgpdFile, error) {
			rows, err := q.RgpdUserFiles(ctx, db.RgpdUserFilesParams{Subject: id, After: a, PageSize: n})
			if err != nil {
				return nil, err
			}

			out := make([]rgpdFile, len(rows))
			for i, r := range rows {
				out[i] = rgpdFile{r.ID, r.Filename, r.StorageKey}
			}

			return out, nil
		}, nil

	case RgpdSubjectClient:
		return []rgpdSection{
			{"client", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientRecord(ctx, db.RgpdClientRecordParams{Subject: id, After: a, PageSize: n}))
			}},
			{"contacts", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientContacts(ctx, db.RgpdClientContactsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"portal_users", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientPortalUsers(ctx, db.RgpdClientPortalUsersParams{Subject: id, After: a, PageSize: n}))
			}},
			{"projects", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientProjects(ctx, db.RgpdClientProjectsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"milestones", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientMilestones(ctx, db.RgpdClientMilestonesParams{Subject: id, After: a, PageSize: n}))
			}},
			{"deliverables", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientDeliverables(ctx, db.RgpdClientDeliverablesParams{Subject: id, After: a, PageSize: n}))
			}},
			{"deliverable_versions", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientDeliverableVersions(ctx, db.RgpdClientDeliverableVersionsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"tickets", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientTickets(ctx, db.RgpdClientTicketsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"ticket_messages", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientTicketMessages(ctx, db.RgpdClientTicketMessagesParams{
					Subject: id, WithInternal: rgpdIncludeInternalNotes, After: a, PageSize: n,
				}))
			}},
			{"interactions", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientInteractions(ctx, db.RgpdClientInteractionsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"inbound_emails", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientInboundEmails(ctx, db.RgpdClientInboundEmailsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"reports", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientReports(ctx, db.RgpdClientReportsParams{Subject: id, After: a, PageSize: n}))
			}},
			{"time_entries", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientTimeEntries(ctx, db.RgpdClientTimeEntriesParams{Subject: id, After: a, PageSize: n}))
			}},
			{"attachments", func(ctx context.Context, a uuid.UUID) ([]rgpdRow, error) {
				return rowsOf(q.RgpdClientAttachments(ctx, db.RgpdClientAttachmentsParams{Subject: id, After: a, PageSize: n}))
			}},
		}, func(ctx context.Context, a uuid.UUID) ([]rgpdFile, error) {
			rows, err := q.RgpdClientFiles(ctx, db.RgpdClientFilesParams{Subject: id, After: a, PageSize: n})
			if err != nil {
				return nil, err
			}

			out := make([]rgpdFile, len(rows))
			for i, r := range rows {
				out[i] = rgpdFile{r.ID, r.Filename, r.StorageKey}
			}

			return out, nil
		}, nil
	}

	return nil, nil, fmt.Errorf("sujet %q inconnu", subjectType)
}

// discordSection : la liaison Discord du compte, une seule ligne au plus.
func (s *RgpdExportService) discordSection(userID uuid.UUID) func(ctx context.Context, after uuid.UUID) ([]rgpdRow, error) {
	return func(ctx context.Context, after uuid.UUID) ([]rgpdRow, error) {
		// Une seule « page » : la liaison n'a pas de cle de pagination.
		if after != uuid.Nil || s.discord == nil {
			return nil, nil
		}

		link, err := s.discord.Export(ctx, userID)
		if err != nil || link == nil {
			return nil, err
		}

		data, err := json.Marshal(link)
		if err != nil {
			return nil, fmt.Errorf("encodage de la liaison Discord : %w", err)
		}

		return []rgpdRow{{ID: userID, Data: data}}, nil
	}
}

type manifestSection struct {
	Name  string `json:"name"`
	File  string `json:"file"`
	Count int    `json:"count"`
}

type rgpdManifest struct {
	Format       int               `json:"format"`
	SubjectType  string            `json:"subject_type"`
	SubjectID    uuid.UUID         `json:"subject_id"`
	SubjectLabel string            `json:"subject_label"`
	GeneratedAt  time.Time         `json:"generated_at"`
	Sections     []manifestSection `json:"sections"`
	Files        struct {
		Included int         `json:"included"`
		Missing  []uuid.UUID `json:"missing"`
	} `json:"files"`
	// InternalTicketNotesIncluded : valeur de rgpdIncludeInternalNotes (client).
	InternalTicketNotesIncluded bool `json:"internal_ticket_notes_included"`
}

// writeArchive ecrit le zip : data/<section>.json, files/<nom>, manifest.json.
func (s *RgpdExportService) writeArchive(ctx context.Context, out io.Writer, e db.RgpdExport) error {
	sections, files, err := s.sectionsOf(e.SubjectType, e.SubjectID)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(out)
	now := time.Now().UTC()

	manifest := rgpdManifest{
		Format: rgpdManifestFormat, SubjectType: e.SubjectType, SubjectID: e.SubjectID, SubjectLabel: e.SubjectLabel,
		GeneratedAt: now, Sections: []manifestSection{}, InternalTicketNotesIncluded: e.SubjectType == RgpdSubjectClient && rgpdIncludeInternalNotes,
	}
	manifest.Files.Missing = []uuid.UUID{}

	for _, sec := range sections {
		file := "data/" + sec.name + ".json"

		w, err := zw.CreateHeader(&zip.FileHeader{Name: file, Method: zip.Deflate, Modified: now})
		if err != nil {
			return err
		}

		count, err := writeSection(ctx, w, sec)
		if err != nil {
			return fmt.Errorf("section %s : %w", sec.name, err)
		}

		manifest.Sections = append(manifest.Sections, manifestSection{Name: sec.name, File: file, Count: count})
	}

	used := map[string]bool{}
	after := uuid.Nil

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		batch, err := files(ctx, after)
		if err != nil {
			return fmt.Errorf("liste des fichiers : %w", err)
		}

		for _, f := range batch {
			after = f.ID

			ok, err := s.addFile(zw, f, used, now)
			if err != nil {
				return err
			}

			if ok {
				manifest.Files.Included++
			} else {
				manifest.Files.Missing = append(manifest.Files.Missing, f.ID)
			}
		}

		if len(batch) < rgpdPageSize {
			break
		}
	}

	mw, err := zw.CreateHeader(&zip.FileHeader{Name: "manifest.json", Method: zip.Deflate, Modified: now})
	if err != nil {
		return err
	}

	enc := json.NewEncoder(mw)
	enc.SetIndent("", "  ")

	if err := enc.Encode(manifest); err != nil {
		return err
	}

	return zw.Close()
}

// writeSection ecrit un tableau JSON page par page et rend le nombre de lignes.
func writeSection(ctx context.Context, w io.Writer, sec rgpdSection) (int, error) {
	if _, err := io.WriteString(w, "["); err != nil {
		return 0, err
	}

	count := 0
	after := uuid.Nil

	for {
		if err := ctx.Err(); err != nil {
			return count, err
		}

		rows, err := sec.page(ctx, after)
		if err != nil {
			return count, err
		}

		for _, r := range rows {
			sep := "\n"
			if count > 0 {
				sep = ",\n"
			}

			if _, err := io.WriteString(w, sep); err != nil {
				return count, err
			}

			if _, err := w.Write(r.Data); err != nil {
				return count, err
			}

			count++
			after = r.ID
		}

		if len(rows) < rgpdPageSize {
			break
		}
	}

	_, err := io.WriteString(w, "\n]\n")

	return count, err
}

// addFile copie un fichier du stockage dans files/. Un fichier absent du
// disque n'arrete pas l'export : il est note au manifeste.
func (s *RgpdExportService) addFile(zw *zip.Writer, f rgpdFile, used map[string]bool, now time.Time) (bool, error) {
	src, err := s.files.Open(f.StorageKey)
	if err != nil {
		return false, nil //nolint:nilerr // fichier manquant : consigne au manifeste
	}
	defer func() { _ = src.Close() }()

	name := zipName(f.Filename)
	if used[name] {
		name = f.ID.String()[:8] + "-" + name
	}

	used[name] = true

	w, err := zw.CreateHeader(&zip.FileHeader{Name: "files/" + name, Method: zip.Deflate, Modified: now})
	if err != nil {
		return false, err
	}

	if _, err := io.Copy(w, src); err != nil {
		return false, fmt.Errorf("copie du fichier : %w", err)
	}

	return true, nil
}

// zipName rend un nom de fichier sur : pas de repertoire, pas de caractere de
// controle, longueur bornee.
func zipName(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}

		return r
	}, name)
	name = strings.TrimSpace(name)

	if name == "" || name == "." || name == ".." || name == "/" {
		return "fichier"
	}

	return truncateRunes(name, maxZipNameLen)
}
