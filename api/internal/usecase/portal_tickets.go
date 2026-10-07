package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// Priorites qu'un client peut choisir. Les deux dernieres de l'agence —
// urgente et critique — se decident cote agence, une fois la demande lue : un
// portail ou tout le monde peut crier « critique » ne trie plus rien.
var portalTicketPriorities = map[string]bool{"low": true, "normal": true, "high": true}

// portalTicketFilesMax borne les pieces jointes d'une demande.
const portalTicketFilesMax = 10

// PortalTicket est une ligne de « Support ».
type PortalTicket struct {
	ID       uuid.UUID      `json:"id"`
	Numero   int64          `json:"numero"`
	Subject  string         `json:"subject"`
	Tracker  string         `json:"tracker"`
	Status   string         `json:"status"`
	Project  DeliverableRef `json:"project"`
	Open     bool           `json:"open"`
	Created  time.Time      `json:"created_at"`
	Updated  time.Time      `json:"updated_at"`
	Priority string         `json:"priority"`
}

// PortalTicketPage est une page de demandes.
type PortalTicketPage struct {
	Items    []PortalTicket `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// PortalTicketEntry est une ligne du fil : un message, ou un changement de
// statut.
type PortalTicketEntry struct {
	// "message" ou "status".
	Kind string    `json:"kind"`
	At   time.Time `json:"at"`
	// Prenom de l'auteur ; vide pour un changement de statut.
	Author string `json:"author"`
	// Vrai quand le message vient de l'agence.
	FromAgency bool   `json:"from_agency"`
	Body       string `json:"body"`
	// Nouveau statut, pour un changement.
	Status string `json:"status"`
}

// PortalTicketDetail est la page d'une demande.
type PortalTicketDetail struct {
	ID          uuid.UUID           `json:"id"`
	Numero      int64               `json:"numero"`
	Subject     string              `json:"subject"`
	Description string              `json:"description"`
	Tracker     string              `json:"tracker"`
	Status      string              `json:"status"`
	Priority    string              `json:"priority"`
	Open        bool                `json:"open"`
	Project     DeliverableRef      `json:"project"`
	Reporter    string              `json:"reporter"`
	CreatedAt   time.Time           `json:"created_at"`
	Entries     []PortalTicketEntry `json:"entries"`
	Files       []PortalFile        `json:"files"`
}

// PortalTicketInput est une demande deposee dans le portail.
type PortalTicketInput struct {
	ProjectID   uuid.UUID
	Tracker     string
	Priority    string
	Subject     string
	Description string
}

func ticketOpen(status string) bool {
	return status != "done" && status != "annule"
}

// Tickets rend les demandes du client de l'appelant ; `open` filtre les
// ouvertes ou les closes, nul pour toutes.
func (s *PortalService) Tickets(ctx context.Context, userID uuid.UUID, open *bool, page int) (PortalTicketPage, error) {
	page, pageSize := clampPage(page, 20)

	rows, err := s.q.PortalListTickets(ctx, db.PortalListTicketsParams{
		UserID: userID, Open: open, PageSize: int32(pageSize), PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return PortalTicketPage{}, fmt.Errorf("lecture des demandes : %w", err)
	}
	total, err := s.q.PortalCountTickets(ctx, db.PortalCountTicketsParams{UserID: userID, Open: open})
	if err != nil {
		return PortalTicketPage{}, fmt.Errorf("comptage des demandes : %w", err)
	}

	items := make([]PortalTicket, 0, len(rows))
	for _, row := range rows {
		items = append(items, PortalTicket{
			ID: row.ID, Numero: row.Numero, Subject: row.Subject, Tracker: row.Tracker,
			Status: row.Status, Priority: row.Priority, Open: ticketOpen(row.Status),
			Project: DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
			Created: row.CreatedAt, Updated: row.UpdatedAt,
		})
	}

	return PortalTicketPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Ticket rend une demande du client et son fil public.
func (s *PortalService) Ticket(ctx context.Context, userID, ticketID uuid.UUID) (PortalTicketDetail, error) {
	row, err := s.q.PortalGetTicket(ctx, db.PortalGetTicketParams{UserID: userID, TicketID: ticketID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PortalTicketDetail{}, domain.ErrNotFound
		}

		return PortalTicketDetail{}, fmt.Errorf("lecture de la demande : %w", err)
	}

	messages, err := s.q.PortalListTicketMessages(ctx, ticketID)
	if err != nil {
		return PortalTicketDetail{}, fmt.Errorf("lecture des messages : %w", err)
	}
	events, err := s.q.PortalListTicketStatusEvents(ctx, ticketID)
	if err != nil {
		return PortalTicketDetail{}, fmt.Errorf("lecture des statuts : %w", err)
	}
	files, err := s.q.ListTicketFiles(ctx, &ticketID)
	if err != nil {
		return PortalTicketDetail{}, fmt.Errorf("lecture des pieces jointes : %w", err)
	}

	out := PortalTicketDetail{
		ID: row.ID, Numero: row.Numero, Subject: row.Subject, Description: row.Description,
		Tracker: row.Tracker, Status: row.Status, Priority: row.Priority, Open: ticketOpen(row.Status),
		Project:   DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
		Reporter:  firstnameOf(row.ReporterFirstname),
		CreatedAt: row.CreatedAt,
		Entries:   make([]PortalTicketEntry, 0, len(messages)+len(events)),
		Files:     make([]PortalFile, 0, len(files)),
	}

	for _, m := range messages {
		author := firstnameOf(m.AuthorFirstname)
		fromAgency := m.AuthorRole == nil || *m.AuthorRole != "client"
		// Ecrit par e-mail : sans compte, c'est le client — ou un collegue a
		// lui — qui a repondu ; avec un compte, son role le dit.
		if m.ViaEmail && m.AuthorRole == nil {
			author, fromAgency = firstWord(m.SenderName), false
		}
		out.Entries = append(out.Entries, PortalTicketEntry{
			Kind: "message", At: m.CreatedAt, Author: author,
			FromAgency: fromAgency, Body: m.Body,
		})
	}
	// Un changement de statut ne s'affiche que s'il change quelque chose aux
	// yeux du client : « a faire » apres « en attente » reste « recue ».
	for _, e := range events {
		if PortalTicketStatus(e.OldValue) == PortalTicketStatus(e.NewValue) {
			continue
		}
		out.Entries = append(out.Entries, PortalTicketEntry{Kind: "status", At: e.CreatedAt, Status: e.NewValue})
	}
	sort.SliceStable(out.Entries, func(i, j int) bool {
		if out.Entries[i].At.Equal(out.Entries[j].At) {
			return out.Entries[i].Kind == "message" && out.Entries[j].Kind == "status"
		}
		return out.Entries[i].At.Before(out.Entries[j].At)
	})

	for _, f := range files {
		out.Files = append(out.Files, PortalFile{
			ID: f.ID, Filename: f.Filename, ContentType: f.ContentType, SizeBytes: f.SizeBytes, CreatedAt: f.CreatedAt,
		})
	}

	return out, nil
}

// CreateTicket depose une demande sur un projet du client de l'appelant.
func (s *PortalService) CreateTicket(ctx context.Context, userID uuid.UUID, in PortalTicketInput) (PortalTicketDetail, error) {
	if _, err := s.q.PortalGetProject(ctx, db.PortalGetProjectParams{UserID: userID, ProjectID: in.ProjectID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PortalTicketDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"project_id": "Choisissez un de vos projets",
			})
		}

		return PortalTicketDetail{}, fmt.Errorf("lecture du projet : %w", err)
	}

	if in.Priority == "" {
		in.Priority = "normal"
	}
	if !portalTicketPriorities[in.Priority] {
		return PortalTicketDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"priority": "Priorité attendue : basse, normale ou haute",
		})
	}
	if strings.TrimSpace(in.Description) == "" {
		return PortalTicketDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"description": "Décrivez votre demande",
		})
	}

	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return PortalTicketDetail{}, fmt.Errorf("lecture du compte : %w", err)
	}

	item, err := s.tickets.Create(ctx, CreateTicketInput{
		ProjectID: in.ProjectID, Subject: in.Subject, Description: in.Description,
		Tracker: in.Tracker, Priority: in.Priority, CreatedBy: &userID,
		FromPortal: true, AuthorName: user.Firstname,
	})
	if err != nil {
		return PortalTicketDetail{}, err
	}

	return s.Ticket(ctx, userID, item.ID)
}

// Reply ajoute la reponse du client au fil d'une de ses demandes.
func (s *PortalService) Reply(ctx context.Context, userID, ticketID uuid.UUID, body string) (PortalTicketDetail, error) {
	if _, err := s.q.PortalGetTicket(ctx, db.PortalGetTicketParams{UserID: userID, TicketID: ticketID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PortalTicketDetail{}, domain.ErrNotFound
		}

		return PortalTicketDetail{}, fmt.Errorf("lecture de la demande : %w", err)
	}

	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return PortalTicketDetail{}, fmt.Errorf("lecture du compte : %w", err)
	}

	// Jamais interne, jamais de changement de statut : le client ecrit, il ne
	// pilote pas le ticket.
	if _, err := s.tickets.PostMessage(ctx, ticketID, PostMessageInput{
		Body: body, AuthorID: &userID, FromClient: true, AuthorName: user.Firstname,
	}); err != nil {
		return PortalTicketDetail{}, err
	}

	return s.Ticket(ctx, userID, ticketID)
}

// AttachToTicket joint un fichier a une demande du client.
func (s *PortalService) AttachToTicket(
	ctx context.Context,
	userID, ticketID uuid.UUID,
	filename, contentType string,
	content io.Reader,
) (PortalFile, error) {
	if _, err := s.q.PortalGetTicket(ctx, db.PortalGetTicketParams{UserID: userID, TicketID: ticketID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PortalFile{}, domain.ErrNotFound
		}

		return PortalFile{}, fmt.Errorf("lecture de la demande : %w", err)
	}

	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "" || filename == "." || filename == "/" {
		return PortalFile{}, domain.ErrValidation.WithDetails(map[string]any{"file": "Nom de fichier invalide"})
	}

	count, err := s.q.CountTicketFiles(ctx, &ticketID)
	if err != nil {
		return PortalFile{}, fmt.Errorf("comptage des pieces jointes : %w", err)
	}
	if count >= portalTicketFilesMax {
		return PortalFile{}, domain.ErrValidation.WithDetails(map[string]any{
			"file": fmt.Sprintf("Une demande porte au plus %d pièces jointes", portalTicketFilesMax),
		})
	}

	key, size, err := s.files.Save(content, s.maxFile)
	if err != nil {
		if errors.Is(err, storage.ErrTooLarge) {
			return PortalFile{}, domain.ErrValidation.WithDetails(map[string]any{
				"file": fmt.Sprintf("Le fichier dépasse %d Mo", s.maxFile/(1<<20)),
			})
		}

		return PortalFile{}, fmt.Errorf("ecriture de la piece jointe : %w", err)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	row, err := s.q.CreateTicketFile(ctx, db.CreateTicketFileParams{
		TicketID: &ticketID, Filename: filename, ContentType: contentType,
		SizeBytes: size, StorageKey: key, UploadedBy: &userID,
	})
	if err != nil {
		_ = s.files.Remove(key)
		return PortalFile{}, fmt.Errorf("enregistrement de la piece jointe : %w", err)
	}

	return PortalFile{ID: row.ID, Filename: row.Filename, ContentType: row.ContentType, SizeBytes: row.SizeBytes, CreatedAt: row.CreatedAt}, nil
}
