package usecase

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/mailin"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// L'e-mail entrant : ce qu'un client ecrit a l'adresse de support devient un
// ticket, ce qu'il repond a un e-mail de ticket rejoint le ticket.
//
// Deux portes, une file. La releve IMAP (tache de fond) et les webhooks des
// fournisseurs deposent le message tel quel dans inbound_emails ; une seconde
// tache le range. Aucune requete HTTP n'attend qu'un e-mail soit lu, et un
// meme e-mail arrive par les deux portes n'est range qu'une fois.

// Raisons de mise de cote, lues par l'ecran de tri.
const (
	reasonUnknownSender   = "unknown_sender"
	reasonAmbiguousSender = "ambiguous_sender"
	reasonProjectToChoose = "project_to_choose"
	reasonSenderMismatch  = "sender_mismatch"
	reasonFromTeam        = "from_team"
	reasonRateLimited     = "rate_limited"
	reasonTicketGone      = "ticket_unmatched"
	reasonClosedTicket    = "closed_ticket"
)

// emailTicketsPerHour : au-dela, une adresse qui ouvre ticket sur ticket est
// sans doute un robot qui repond a nos accuses de reception.
const emailTicketsPerHour = 10

// Erreurs propres a l'e-mail entrant.
var (
	ErrInboundForbidden = &domain.Error{Status: http.StatusUnauthorized, Code: "INVALID_TOKEN", Message: "Jeton du webhook invalide"}
	ErrInboundNotHeld   = &domain.Error{Status: http.StatusConflict, Code: "ALREADY_SORTED", Message: "Cet e-mail a déjà été trié"}
)

// InboundService porte l'e-mail entrant.
type InboundService struct {
	pool    *pgxpool.Pool
	q       *db.Queries
	tickets *TicketService
	files   storage.Store
	maxFile int64
	bus     Bus
	env     config.InboundEnv
	thread  threading
	// Cle de chiffrement du mot de passe IMAP en base.
	sealKey []byte
	// Adresse publique des webhooks, et expediteur des e-mails de Piilot :
	// un e-mail de Piilot qui revient n'ouvre pas de ticket.
	hookBase string
	ownFrom  string
	log      *slog.Logger
}

// NewInboundService construit le service.
func NewInboundService(
	pool *pgxpool.Pool, tickets *TicketService, files storage.Store, maxFile int64, bus Bus,
	env config.InboundEnv, secret []byte, hookBase, ownFrom string, log *slog.Logger,
) *InboundService {
	return &InboundService{
		pool: pool, q: db.New(pool), tickets: tickets, files: files, maxFile: maxFile, bus: bus, env: env,
		thread: newThreading(secret, env.Address), sealKey: security.DeriveKey(secret, "inbound-imap-password"),
		hookBase: strings.TrimRight(hookBase, "/"), ownFrom: strings.ToLower(ownFrom), log: log,
	}
}

// --- Reglages ----------------------------------------------------------------

// InboundIMAPView est la releve IMAP vue par l'ecran. Le mot de passe n'en
// sort jamais : seulement s'il est renseigne.
type InboundIMAPView struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Security    string `json:"security"`
	Username    string `json:"username"`
	PasswordSet bool   `json:"password_set"`
	Folder      string `json:"folder"`
	FromEnv     bool   `json:"from_env"`
}

// InboundWebhookView donne les adresses a coller chez le fournisseur.
type InboundWebhookView struct {
	Secret  string            `json:"secret"`
	FromEnv bool              `json:"from_env"`
	URLs    map[string]string `json:"urls"`
}

// InboundStatusView dit ce que la releve et le rangement ont fait.
type InboundStatusView struct {
	LastPollAt    *time.Time `json:"last_poll_at"`
	LastPollError string     `json:"last_poll_error"`
	PollPending   bool       `json:"poll_pending"`
	Pending       int        `json:"pending"`
	Held          int        `json:"held"`
	ProcessedDay  int        `json:"processed_day"`
	FailedWeek    int        `json:"failed_week"`
}

// InboundSettingsView est l'ecran Paramètres > E-mails entrants.
type InboundSettingsView struct {
	Address        string `json:"address"`
	AddressFromEnv bool   `json:"address_from_env"`
	// Les e-mails des tickets invitent a repondre : l'adresse est posee et
	// la cle des etiquettes existe.
	ReplyEnabled bool               `json:"reply_enabled"`
	IMAP         InboundIMAPView    `json:"imap"`
	Webhook      InboundWebhookView `json:"webhook"`
	Status       InboundStatusView  `json:"status"`
}

// InboundSettingsInput modifie les reglages faits a l'ecran. Le mot de passe
// nul garde celui en place.
type InboundSettingsInput struct {
	Address      string
	IMAPEnabled  bool
	IMAPHost     string
	IMAPPort     int
	IMAPSecurity string
	IMAPUsername string
	IMAPPassword *string
	IMAPFolder   string
}

// Settings lit les reglages, en tirant le secret des webhooks au premier
// affichage.
func (s *InboundService) Settings(ctx context.Context) (InboundSettingsView, error) {
	row, err := s.q.GetInboundSettings(ctx)
	if err != nil {
		return InboundSettingsView{}, fmt.Errorf("lecture des reglages : %w", err)
	}
	secret, err := s.webhookSecret(ctx, row)
	if err != nil {
		return InboundSettingsView{}, err
	}
	stats, err := s.q.InboundStats(ctx)
	if err != nil {
		return InboundSettingsView{}, fmt.Errorf("statistiques : %w", err)
	}

	out := InboundSettingsView{
		Address:        row.Address,
		AddressFromEnv: s.env.Address != "",
		IMAP: InboundIMAPView{
			Enabled: row.ImapEnabled, Host: row.ImapHost, Port: int(row.ImapPort), Security: row.ImapSecurity,
			Username: row.ImapUsername, PasswordSet: len(row.ImapPasswordEnc) > 0, Folder: row.ImapFolder,
		},
		Webhook: InboundWebhookView{Secret: secret, FromEnv: s.env.WebhookSecret != "", URLs: map[string]string{}},
		Status: InboundStatusView{
			LastPollAt: row.LastPollAt, LastPollError: row.LastPollError, PollPending: row.PollRequestedAt != nil,
			Pending: int(stats.Pending), Held: int(stats.Held), ProcessedDay: int(stats.ProcessedDay), FailedWeek: int(stats.FailedWeek),
		},
	}
	if s.env.Address != "" {
		out.Address = s.env.Address
	}
	if s.env.IMAPHost != "" {
		out.IMAP = InboundIMAPView{
			Enabled: true, Host: s.env.IMAPHost, Port: s.env.IMAPPort, Security: s.env.IMAPSecurity,
			Username: s.env.IMAPUsername, PasswordSet: s.env.IMAPPassword != "", Folder: s.env.IMAPFolder, FromEnv: true,
		}
	}
	out.ReplyEnabled = strings.Contains(out.Address, "@") && len(s.thread.key) > 0
	for _, p := range mailin.Providers {
		out.Webhook.URLs[p] = fmt.Sprintf("%s/api/v1/hooks/mail/%s?token=%s", s.hookBase, p, secret)
	}
	return out, nil
}

// UpdateSettings enregistre les reglages faits a l'ecran. Ce que
// l'environnement fixe n'est pas modifiable ici.
func (s *InboundService) UpdateSettings(ctx context.Context, in InboundSettingsInput) (InboundSettingsView, error) {
	details := map[string]any{}
	in.Address = strings.ToLower(strings.TrimSpace(in.Address))
	if in.Address != "" {
		local, domainPart, ok := strings.Cut(in.Address, "@")
		if !ok || local == "" || !strings.Contains(domainPart, ".") || strings.Contains(local, "+") {
			details["address"] = "Une adresse complète, sans « + » : support@agence.fr"
		}
	}
	in.IMAPHost = strings.TrimSpace(in.IMAPHost)
	if in.IMAPSecurity == "" {
		in.IMAPSecurity = "tls"
	}
	if in.IMAPSecurity != "tls" && in.IMAPSecurity != "none" {
		details["imap_security"] = "tls ou none"
	}
	if in.IMAPPort == 0 {
		in.IMAPPort = 993
	}
	if in.IMAPPort < 1 || in.IMAPPort > 65535 {
		details["imap_port"] = "Port invalide"
	}
	if strings.TrimSpace(in.IMAPFolder) == "" {
		in.IMAPFolder = "INBOX"
	}
	if in.IMAPEnabled && (in.IMAPHost == "" || strings.TrimSpace(in.IMAPUsername) == "") {
		details["imap_host"] = "Serveur et identifiant requis pour relever la boîte"
	}
	if len(details) > 0 {
		return InboundSettingsView{}, domain.ErrValidation.WithDetails(details)
	}

	params := db.UpdateInboundSettingsParams{
		Address: in.Address, ImapEnabled: in.IMAPEnabled, ImapHost: in.IMAPHost, ImapPort: int32(in.IMAPPort),
		ImapSecurity: in.IMAPSecurity, ImapUsername: strings.TrimSpace(in.IMAPUsername), ImapFolder: strings.TrimSpace(in.IMAPFolder),
	}
	if in.IMAPPassword != nil {
		params.SetPassword = true
		if *in.IMAPPassword != "" {
			sealed, err := security.Seal(s.sealKey, *in.IMAPPassword)
			if err != nil {
				return InboundSettingsView{}, fmt.Errorf("chiffrement du mot de passe : %w", err)
			}
			params.ImapPasswordEnc = sealed
		}
	}
	if err := s.q.UpdateInboundSettings(ctx, params); err != nil {
		return InboundSettingsView{}, fmt.Errorf("enregistrement des reglages : %w", err)
	}
	return s.Settings(ctx)
}

// RotateWebhookSecret tire un nouveau secret : les webhooks configures avec
// l'ancien cessent d'etre acceptes.
func (s *InboundService) RotateWebhookSecret(ctx context.Context) (InboundSettingsView, error) {
	if s.env.WebhookSecret != "" {
		return InboundSettingsView{}, domain.ErrConflict.WithMessage("Le secret est fixé par INBOUND_WEBHOOK_SECRET")
	}
	raw, _, err := security.NewOneTimeToken()
	if err != nil {
		return InboundSettingsView{}, err
	}
	if err := s.q.SetInboundWebhookSecret(ctx, raw); err != nil {
		return InboundSettingsView{}, fmt.Errorf("enregistrement du secret : %w", err)
	}
	return s.Settings(ctx)
}

// RequestPoll demande une releve immediate, que la tache de fond fait dans
// les dix secondes.
func (s *InboundService) RequestPoll(ctx context.Context) (InboundSettingsView, error) {
	if err := s.q.RequestInboundPoll(ctx); err != nil {
		return InboundSettingsView{}, fmt.Errorf("demande de releve : %w", err)
	}
	return s.Settings(ctx)
}

func (s *InboundService) webhookSecret(ctx context.Context, row db.InboundMailSetting) (string, error) {
	if s.env.WebhookSecret != "" {
		return s.env.WebhookSecret, nil
	}
	if row.WebhookSecret != "" {
		return row.WebhookSecret, nil
	}
	raw, _, err := security.NewOneTimeToken()
	if err != nil {
		return "", err
	}
	if err := s.q.SetInboundWebhookSecret(ctx, raw); err != nil {
		return "", fmt.Errorf("enregistrement du secret : %w", err)
	}
	return raw, nil
}

// --- Entree ------------------------------------------------------------------

// Webhook depose les e-mails qu'un fournisseur pousse. Le jeton de l'adresse
// fait office de preuve.
func (s *InboundService) Webhook(ctx context.Context, provider, token string, body []byte, form *multipart.Form) (int, error) {
	row, err := s.q.GetInboundSettings(ctx)
	if err != nil {
		return 0, fmt.Errorf("lecture des reglages : %w", err)
	}
	secret := s.env.WebhookSecret
	if secret == "" {
		secret = row.WebhookSecret
	}
	if secret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(token)) != 1 {
		return 0, ErrInboundForbidden
	}

	raws, err := mailin.FromWebhook(provider, body, form)
	if errors.Is(err, mailin.ErrUnknownProvider) {
		return 0, domain.ErrNotFound
	}
	if err != nil {
		return 0, domain.ErrValidation.WithMessage(err.Error())
	}
	stored := 0
	for _, raw := range raws {
		ok, err := s.Ingest(ctx, provider, raw)
		if err != nil {
			return stored, err
		}
		if ok {
			stored++
		}
	}
	return stored, nil
}

// Ingest depose un e-mail dans la file. Faux s'il y etait deja.
func (s *InboundService) Ingest(ctx context.Context, source string, raw []byte) (bool, error) {
	if len(raw) > mailin.MaxRawBytes {
		return false, domain.ErrValidation.WithMessage("E-mail trop volumineux")
	}
	// Une lecture d'en-tete suffit ici ; un e-mail illisible entre quand meme,
	// et le rangement le marquera en echec, visible a l'ecran.
	params := db.InsertInboundEmailParams{Source: source, Raw: raw}
	if msg, err := mailin.Parse(raw); err == nil {
		params.MessageID = msg.MessageID
		params.FromAddress = msg.From.Address
		params.FromName = msg.From.Name
		params.Subject = truncate(msg.Subject, 300)
		params.Excerpt = truncate(msg.Body(), 280)
		params.AttachmentCount = int32(len(msg.Attachments))
	}
	ids, err := s.q.InsertInboundEmail(ctx, params)
	if err != nil {
		return false, fmt.Errorf("depot de l'e-mail : %w", err)
	}
	return len(ids) > 0, nil
}

// imapConfig rend la boite a relever, ou faux si la releve est coupee.
func (s *InboundService) imapConfig(ctx context.Context) (mailin.IMAPConfig, bool, error) {
	if s.env.IMAPHost != "" {
		return mailin.IMAPConfig{
			Host: s.env.IMAPHost, Port: s.env.IMAPPort, TLS: s.env.IMAPSecurity != "none",
			Username: s.env.IMAPUsername, Password: s.env.IMAPPassword, Folder: s.env.IMAPFolder,
		}, true, nil
	}
	row, err := s.q.GetInboundSettings(ctx)
	if err != nil {
		return mailin.IMAPConfig{}, false, err
	}
	if !row.ImapEnabled || row.ImapHost == "" {
		return mailin.IMAPConfig{}, false, nil
	}
	password := ""
	if len(row.ImapPasswordEnc) > 0 {
		password, err = security.Open(s.sealKey, row.ImapPasswordEnc)
		if err != nil {
			return mailin.IMAPConfig{}, true, errors.New("mot de passe IMAP illisible : JWT_SECRET a changé, ressaisissez-le")
		}
	}
	return mailin.IMAPConfig{
		Host: row.ImapHost, Port: int(row.ImapPort), TLS: row.ImapSecurity != "none",
		Username: row.ImapUsername, Password: password, Folder: row.ImapFolder,
	}, true, nil
}

// PollDue dit s'il est temps de relever : une minute depuis la derniere
// releve, ou une demande d'un admin.
func (s *InboundService) PollDue(ctx context.Context, every time.Duration) bool {
	row, err := s.q.GetInboundSettings(ctx)
	if err != nil {
		return false
	}
	return row.PollRequestedAt != nil || row.LastPollAt == nil || time.Since(*row.LastPollAt) >= every
}

// maxPerPoll borne une releve : une boite qui deborde se vide en quelques
// minutes plutot qu'en une releve qui n'en finit pas.
const maxPerPoll = 50

// PollIMAP releve la boite : chaque message non lu entre dans la file, puis
// est marque lu. Un message deja en file n'y entre pas deux fois.
func (s *InboundService) PollIMAP(ctx context.Context) error {
	cfg, enabled, err := s.imapConfig(ctx)
	if !enabled {
		if err != nil {
			return err
		}
		// Une releve demandee alors que la boite n'est pas reglee : la
		// demande ne doit pas rester en suspens a l'ecran.
		if row, rerr := s.q.GetInboundSettings(ctx); rerr == nil && row.PollRequestedAt != nil {
			return s.q.RecordInboundPoll(ctx, "La relève IMAP n'est pas activée")
		}
		return nil
	}
	if err == nil {
		err = s.poll(ctx, cfg)
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	if recErr := s.q.RecordInboundPoll(ctx, message); recErr != nil && err == nil {
		err = recErr
	}
	return err
}

func (s *InboundService) poll(ctx context.Context, cfg mailin.IMAPConfig) error {
	c, err := mailin.DialIMAP(cfg)
	if err != nil {
		return err
	}
	defer c.Close()

	uids, err := c.Unseen()
	if err != nil {
		return fmt.Errorf("liste des non lus : %w", err)
	}
	if len(uids) > maxPerPoll {
		uids = uids[:maxPerPoll]
	}
	for _, uid := range uids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		raw, err := c.Fetch(uid)
		if err != nil {
			return fmt.Errorf("lecture du message %d : %w", uid, err)
		}
		if _, err := s.Ingest(ctx, "imap", raw); err != nil {
			// Trop gros : on le laisse non lu, il sera signale a chaque releve.
			s.log.Warn("e-mail entrant refuse", "uid", uid, "error", err)
			continue
		}
		if err := c.MarkSeen(uid); err != nil {
			return fmt.Errorf("marquage du message %d : %w", uid, err)
		}
	}
	return nil
}

// --- Rangement ---------------------------------------------------------------

// ProcessPending range les e-mails en attente, vingt au plus par passage.
func (s *InboundService) ProcessPending(ctx context.Context) {
	for range 20 {
		if !s.processNext(ctx) {
			return
		}
	}
}

func (s *InboundService) processNext(ctx context.Context) bool {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.log.Warn("e-mails entrants : transaction", "error", err)
		return false
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.ClaimPendingInbound(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		s.log.Warn("e-mails entrants : lecture", "error", err)
		return false
	}

	// Le rangement ecrit le ticket dans ses propres transactions ; la ligne
	// reste verrouillee ici jusqu'a ce que son sort soit note.
	outcome, err := s.route(ctx, row)
	if err != nil {
		s.log.Warn("e-mail entrant non range", "id", row.ID, "error", err)
		if ferr := q.FailInboundAttempt(ctx, db.FailInboundAttemptParams{ID: row.ID, Error: truncate(err.Error(), 500)}); ferr != nil {
			return false
		}
	} else if ferr := q.FinishInboundEmail(ctx, db.FinishInboundEmailParams{
		ID: row.ID, Status: outcome.status, Reason: outcome.reason, ClientID: outcome.clientID, TicketID: outcome.ticketID,
	}); ferr != nil {
		s.log.Warn("e-mails entrants : enregistrement", "error", ferr)
		return false
	}
	if err := tx.Commit(ctx); err != nil {
		s.log.Warn("e-mails entrants : validation", "error", err)
		return false
	}

	if err == nil && outcome.status == "held" {
		s.notifyHeld(ctx, row, outcome.reason)
	}
	return true
}

type outcome struct {
	status   string
	reason   string
	clientID *uuid.UUID
	ticketID *uuid.UUID
}

// sender est l'expediteur d'un e-mail, reconnu.
type sender struct {
	// internal (equipe), client (compte du portail), contact (CRM, sans
	// compte) ou unknown.
	kind      string
	userID    *uuid.UUID
	clientID  *uuid.UUID
	name      string
	email     string
	ambiguous bool
}

func (s *InboundService) identify(ctx context.Context, address, headerName string) (sender, error) {
	out := sender{kind: "unknown", email: strings.ToLower(address), name: strings.TrimSpace(headerName)}
	if out.email == "" {
		return out, nil
	}

	u, err := s.q.FindActiveUserByEmail(ctx, out.email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return out, fmt.Errorf("recherche du compte : %w", err)
	default:
		id := u.ID
		out.userID = &id
		out.name = strings.TrimSpace(u.Firstname + " " + u.Lastname)
		if u.Role == "client" && u.ClientID != nil {
			out.kind, out.clientID = "client", u.ClientID
		} else if u.Role != "client" {
			out.kind = "internal"
		}
		return out, nil
	}

	contacts, err := s.q.FindContactsByEmail(ctx, &out.email)
	if err != nil {
		return out, fmt.Errorf("recherche du contact : %w", err)
	}
	clients := map[uuid.UUID]bool{}
	for _, c := range contacts {
		if c.ClientID != nil {
			clients[*c.ClientID] = true
		}
	}
	switch len(clients) {
	case 0:
	case 1:
		c := contacts[0]
		out.kind, out.clientID = "contact", c.ClientID
		if name := strings.TrimSpace(c.Firstname + " " + c.Lastname); name != "" {
			out.name = name
		}
	default:
		// La meme adresse chez deux clients : on ne devine pas.
		out.ambiguous = true
	}
	return out, nil
}

// route decide du sort d'un e-mail et l'applique.
func (s *InboundService) route(ctx context.Context, row db.InboundEmail) (outcome, error) {
	msg, err := mailin.Parse(row.Raw)
	if err != nil {
		return outcome{status: "failed", reason: "unreadable"}, nil
	}
	from := strings.ToLower(msg.From.Address)
	switch {
	case msg.Automatic:
		return outcome{status: "ignored", reason: "auto_reply"}, nil
	case from == "":
		return outcome{status: "ignored", reason: "no_sender"}, nil
	case s.isOwnAddress(ctx, from):
		return outcome{status: "ignored", reason: "loop"}, nil
	}

	who, err := s.identify(ctx, from, msg.From.Name)
	if err != nil {
		return outcome{}, err
	}

	// Une reponse a un ticket : l'etiquette est dans l'adresse ou le fil.
	if numero := s.ticketNumero(msg); numero != 0 {
		t, err := s.q.GetTicketByNumero(ctx, numero)
		if errors.Is(err, pgx.ErrNoRows) {
			return outcome{status: "held", reason: reasonTicketGone, clientID: who.clientID}, nil
		}
		if err != nil {
			return outcome{}, fmt.Errorf("lecture du ticket : %w", err)
		}
		ticketID, clientID := t.ID, t.ClientID
		switch {
		case who.kind == "internal":
		case (who.kind == "client" || who.kind == "contact") && who.clientID != nil && *who.clientID == clientID:
		default:
			// L'etiquette est juste mais l'expediteur n'est pas du client : un
			// transfert, une adresse personnelle. Un humain decide.
			return outcome{status: "held", reason: reasonSenderMismatch, clientID: &clientID, ticketID: &ticketID}, nil
		}
		if err := s.append(ctx, t.ID, t.Status, msg, who); err != nil {
			return outcome{}, err
		}
		return outcome{status: "processed", reason: "replied", clientID: &clientID, ticketID: &ticketID}, nil
	}

	// Une nouvelle demande.
	switch {
	case who.kind == "internal":
		// Un collegue qui transfere l'e-mail d'un client : a trier, il sait
		// pour qui.
		return outcome{status: "held", reason: reasonFromTeam}, nil
	case who.ambiguous:
		return outcome{status: "held", reason: reasonAmbiguousSender}, nil
	case who.kind == "unknown":
		return outcome{status: "held", reason: reasonUnknownSender}, nil
	}

	recent, err := s.q.CountTicketsOpenedByEmailFrom(ctx, from)
	if err != nil {
		return outcome{}, fmt.Errorf("comptage des demandes : %w", err)
	}
	if recent >= emailTicketsPerHour {
		return outcome{status: "held", reason: reasonRateLimited, clientID: who.clientID}, nil
	}

	projects, err := s.q.ListTicketProjectsOfClient(ctx, *who.clientID)
	if err != nil {
		return outcome{}, fmt.Errorf("projets du client : %w", err)
	}
	if len(projects) != 1 {
		return outcome{status: "held", reason: reasonProjectToChoose, clientID: who.clientID}, nil
	}

	ticketID, err := s.open(ctx, projects[0].ID, msg, who)
	if err != nil {
		return outcome{}, err
	}
	return outcome{status: "processed", reason: "created", clientID: who.clientID, ticketID: &ticketID}, nil
}

func (s *InboundService) ticketNumero(msg mailin.Message) int64 {
	texts := make([]string, 0, 8)
	for _, a := range append(append([]mailin.Address{}, msg.To...), msg.Cc...) {
		texts = append(texts, a.Address)
	}
	texts = append(texts, msg.Envelope...)
	texts = append(texts, msg.InReplyTo...)
	texts = append(texts, msg.References...)
	if n := s.thread.numeros(texts...); len(n) > 0 {
		return n[0]
	}
	return 0
}

// isOwnAddress : l'adresse de support ou l'expediteur de Piilot. Un e-mail
// qui en vient est un des notres qui revient.
func (s *InboundService) isOwnAddress(ctx context.Context, from string) bool {
	if from == s.ownFrom {
		return true
	}
	support := strings.ToLower(s.thread.address(ctx, s.q))
	if support == "" {
		return false
	}
	local, domainPart, _ := strings.Cut(support, "@")
	fl, fd, _ := strings.Cut(from, "@")
	if plus := strings.IndexByte(fl, '+'); plus >= 0 {
		fl = fl[:plus]
	}
	return fl == local && fd == domainPart
}

// open ouvre un ticket a partir d'un e-mail.
func (s *InboundService) open(ctx context.Context, projectID uuid.UUID, msg mailin.Message, who sender) (uuid.UUID, error) {
	subject := mailin.CleanSubject(msg.Subject)
	if subject == "" {
		subject = "Demande par e-mail"
	}
	body := msg.Body()
	if body == "" {
		body = "(e-mail sans texte)"
	}

	in := CreateTicketInput{
		ProjectID: projectID, Subject: truncate(subject, 200), Description: body,
		Tracker: "assistance", Priority: "normal", CreatedBy: who.userID,
		AuthorName: who.name, ViaEmail: true, OriginalMessageID: msg.MessageID,
	}
	switch who.kind {
	case "internal":
		// Transfere par un collegue : interne, il le rendra visible s'il faut.
		in.CreatedBy = who.userID
	case "client":
		in.FromPortal = true
	default:
		in.FromPortal, in.CreatedBy = true, nil
		in.RequesterEmail, in.RequesterName = who.email, who.name
	}

	item, err := s.tickets.Create(ctx, in)
	if err != nil {
		return uuid.Nil, err
	}
	s.attach(ctx, item.ID, msg, who)
	return item.ID, nil
}

// append ajoute un e-mail au fil d'un ticket. Un ticket clos rouvre : le
// client qui repond « ca ne marche toujours pas » ne doit pas parler dans le
// vide.
func (s *InboundService) append(ctx context.Context, ticketID uuid.UUID, status string, msg mailin.Message, who sender) error {
	body := msg.Body()
	if body == "" {
		body = "(e-mail sans texte)"
	}
	in := PostMessageInput{
		Body: body, ViaEmail: true, SenderName: who.name, SenderEmail: who.email, AuthorName: who.name,
	}
	if who.kind == "internal" {
		// Un collegue qui repond a l'e-mail : une note interne, jamais un
		// message envoye au client sans qu'il l'ait voulu depuis Piilot.
		in.IsInternal, in.AuthorID = true, who.userID
	} else {
		in.FromClient = true
		if who.kind == "client" {
			in.AuthorID = who.userID
		}
		if status == "done" || status == "annule" {
			reopen := "todo"
			in.NewStatus = &reopen
		}
	}

	if _, err := s.tickets.PostMessage(ctx, ticketID, in); err != nil {
		return err
	}
	s.attach(ctx, ticketID, msg, who)
	return nil
}

// attach joint les pieces jointes d'un e-mail au ticket. Une piece jointe
// trop lourde est laissee : le message est deja la, mieux vaut un fichier
// manquant qu'un e-mail perdu.
func (s *InboundService) attach(ctx context.Context, ticketID uuid.UUID, msg mailin.Message, who sender) {
	for _, a := range msg.Attachments {
		if int64(len(a.Content)) > s.maxFile {
			s.log.Info("piece jointe d'e-mail ignoree : trop lourde", "ticket", ticketID, "fichier", a.Filename)
			continue
		}
		key, size, err := s.files.Save(bytes.NewReader(a.Content), s.maxFile)
		if err != nil {
			s.log.Warn("piece jointe d'e-mail : ecriture", "error", err)
			continue
		}
		contentType := a.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		var uploadedBy *uuid.UUID
		if who.kind == "client" || who.kind == "internal" {
			uploadedBy = who.userID
		}
		if _, err := s.q.CreateTicketFile(ctx, db.CreateTicketFileParams{
			TicketID: &ticketID, Filename: truncate(a.Filename, 200), ContentType: contentType,
			SizeBytes: size, StorageKey: key, UploadedBy: uploadedBy,
		}); err != nil {
			_ = s.files.Remove(key)
			s.log.Warn("piece jointe d'e-mail : enregistrement", "error", err)
		}
	}
}

func (s *InboundService) notifyHeld(ctx context.Context, row db.InboundEmail, reason string) {
	recipients, err := s.q.ListTriageRecipients(ctx)
	if err != nil || len(recipients) == 0 {
		return
	}
	err = deliver(ctx, s.q, s.bus, notice{
		Kind:       NotifyInboundEmailHeld,
		Payload:    map[string]any{"from": row.FromAddress, "subject": row.Subject, "reason": reason},
		Recipients: recipients,
	})
	if err != nil {
		s.log.Warn("notification d'e-mail a trier", "error", err)
	}
}

// --- Tri ---------------------------------------------------------------------

// InboundProject est un projet propose au tri.
type InboundProject struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// InboundItem est un e-mail a trier.
type InboundItem struct {
	ID          uuid.UUID        `json:"id"`
	ReceivedAt  time.Time        `json:"received_at"`
	Source      string           `json:"source"`
	FromAddress string           `json:"from_address"`
	FromName    string           `json:"from_name"`
	Subject     string           `json:"subject"`
	Excerpt     string           `json:"excerpt"`
	Attachments int              `json:"attachments"`
	Reason      string           `json:"reason"`
	Client      *TicketProject   `json:"client"`
	Projects    []InboundProject `json:"projects"`
}

// InboundPage est la liste a trier.
type InboundPage struct {
	Items []InboundItem `json:"items"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
}

// InboundFile est une piece jointe d'un e-mail a trier.
type InboundFile struct {
	Filename  string `json:"filename"`
	SizeBytes int    `json:"size_bytes"`
}

// InboundDetail est un e-mail a trier, en entier.
type InboundDetail struct {
	InboundItem
	Status   string        `json:"status"`
	To       []string      `json:"to"`
	Body     string        `json:"body"`
	Files    []InboundFile `json:"files"`
	TicketID *uuid.UUID    `json:"ticket_id"`
}

const inboundPageSize = 25

// Held rend une page des e-mails a trier, avec les projets de leur client.
func (s *InboundService) Held(ctx context.Context, page int) (InboundPage, error) {
	if page < 1 {
		page = 1
	}
	rows, err := s.q.ListHeldInbound(ctx, db.ListHeldInboundParams{MaxRows: inboundPageSize, Skip: int32((page - 1) * inboundPageSize)})
	if err != nil {
		return InboundPage{}, fmt.Errorf("lecture des e-mails a trier : %w", err)
	}
	total, err := s.q.CountHeldInbound(ctx)
	if err != nil {
		return InboundPage{}, fmt.Errorf("comptage : %w", err)
	}

	var clientIDs []uuid.UUID
	for _, r := range rows {
		if r.ClientID != nil {
			clientIDs = append(clientIDs, *r.ClientID)
		}
	}
	projects := map[uuid.UUID][]InboundProject{}
	if len(clientIDs) > 0 {
		list, err := s.q.ListTicketProjectsOfClients(ctx, clientIDs)
		if err != nil {
			return InboundPage{}, fmt.Errorf("projets des clients : %w", err)
		}
		for _, p := range list {
			projects[p.ClientID] = append(projects[p.ClientID], InboundProject{ID: p.ID, Name: p.Name})
		}
	}

	out := InboundPage{Items: make([]InboundItem, 0, len(rows)), Total: int(total), Page: page}
	for _, r := range rows {
		item := InboundItem{
			ID: r.ID, ReceivedAt: r.ReceivedAt, Source: r.Source, FromAddress: r.FromAddress, FromName: r.FromName,
			Subject: r.Subject, Excerpt: r.Excerpt, Attachments: int(r.AttachmentCount), Reason: r.Reason,
			Projects: []InboundProject{},
		}
		if r.ClientID != nil && r.ClientName != nil {
			item.Client = &TicketProject{ID: *r.ClientID, Name: *r.ClientName}
			if p := projects[*r.ClientID]; p != nil {
				item.Projects = p
			}
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// Detail rend un e-mail entier, citations comprises.
func (s *InboundService) Detail(ctx context.Context, id uuid.UUID) (InboundDetail, error) {
	row, err := s.q.GetInboundEmail(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return InboundDetail{}, domain.ErrNotFound
	}
	if err != nil {
		return InboundDetail{}, fmt.Errorf("lecture de l'e-mail : %w", err)
	}

	out := InboundDetail{
		InboundItem: InboundItem{
			ID: row.ID, ReceivedAt: row.ReceivedAt, Source: row.Source, FromAddress: row.FromAddress, FromName: row.FromName,
			Subject: row.Subject, Excerpt: row.Excerpt, Attachments: int(row.AttachmentCount), Reason: row.Reason,
			Projects: []InboundProject{},
		},
		Status: row.Status, To: []string{}, Files: []InboundFile{}, TicketID: row.TicketID,
	}
	if row.ClientID != nil && row.ClientName != nil {
		out.Client = &TicketProject{ID: *row.ClientID, Name: *row.ClientName}
		list, err := s.q.ListTicketProjectsOfClient(ctx, *row.ClientID)
		if err != nil {
			return InboundDetail{}, fmt.Errorf("projets du client : %w", err)
		}
		for _, p := range list {
			out.Projects = append(out.Projects, InboundProject{ID: p.ID, Name: p.Name})
		}
	}
	if msg, err := mailin.Parse(row.Raw); err == nil {
		out.Body = msg.FullBody()
		for _, a := range append(append([]mailin.Address{}, msg.To...), msg.Cc...) {
			out.To = append(out.To, a.Address)
		}
		for _, a := range msg.Attachments {
			out.Files = append(out.Files, InboundFile{Filename: a.Filename, SizeBytes: len(a.Content)})
		}
	}
	return out, nil
}

// lockHeld verrouille un e-mail a trier, et le relit.
func (s *InboundService) lockHeld(ctx context.Context, q *db.Queries, id uuid.UUID) (db.InboundEmail, mailin.Message, error) {
	row, err := q.LockInboundEmail(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.InboundEmail{}, mailin.Message{}, domain.ErrNotFound
	}
	if err != nil {
		return db.InboundEmail{}, mailin.Message{}, fmt.Errorf("lecture de l'e-mail : %w", err)
	}
	if row.Status != "held" {
		return db.InboundEmail{}, mailin.Message{}, ErrInboundNotHeld
	}
	msg, err := mailin.Parse(row.Raw)
	if err != nil {
		return db.InboundEmail{}, mailin.Message{}, domain.ErrValidation.WithMessage("E-mail illisible")
	}
	return row, msg, nil
}

// OpenTicket fait d'un e-mail a trier un ticket, sur le projet choisi.
func (s *InboundService) OpenTicket(ctx context.Context, id, projectID uuid.UUID) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, msg, err := s.lockHeld(ctx, q, id)
	if err != nil {
		return uuid.Nil, err
	}
	projectClient, err := s.q.GetProjectClientForTriage(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrValidation.WithDetails(map[string]any{"project_id": "Projet introuvable"})
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("lecture du projet : %w", err)
	}

	who, err := s.identify(ctx, msg.From.Address, msg.From.Name)
	if err != nil {
		return uuid.Nil, err
	}
	// Un contact d'un autre client que celui du projet choisi : la personne
	// qui trie sait ce qu'elle fait, mais le ticket ne doit pas montrer au
	// compte du portail d'un client le projet d'un autre.
	if who.kind == "client" && who.clientID != nil && *who.clientID != projectClient {
		who.kind, who.userID = "contact", nil
	}
	if who.kind == "unknown" {
		who.kind = "contact"
	}

	ticketID, err := s.open(ctx, projectID, msg, who)
	if err != nil {
		return uuid.Nil, err
	}
	clientID := projectClient
	if err := q.FinishInboundEmail(ctx, db.FinishInboundEmailParams{
		ID: row.ID, Status: "processed", Reason: "created", ClientID: &clientID, TicketID: &ticketID,
	}); err != nil {
		return uuid.Nil, fmt.Errorf("enregistrement du tri : %w", err)
	}
	return ticketID, tx.Commit(ctx)
}

// AttachToTicket ajoute un e-mail a trier au fil d'un ticket, designe par son
// numero.
func (s *InboundService) AttachToTicket(ctx context.Context, id uuid.UUID, numero int64) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, msg, err := s.lockHeld(ctx, q, id)
	if err != nil {
		return uuid.Nil, err
	}
	t, err := s.q.GetTicketByNumero(ctx, numero)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrValidation.WithDetails(map[string]any{"numero": fmt.Sprintf("Aucun ticket #%d", numero)})
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("lecture du ticket : %w", err)
	}

	who, err := s.identify(ctx, msg.From.Address, msg.From.Name)
	if err != nil {
		return uuid.Nil, err
	}
	// Choisi a la main : un compte d'un autre client ecrit au nom de son
	// adresse, sans son compte.
	if who.kind == "client" && (who.clientID == nil || *who.clientID != t.ClientID) {
		who.kind, who.userID = "contact", nil
	}
	if who.kind == "unknown" {
		who.kind = "contact"
	}

	if err := s.append(ctx, t.ID, t.Status, msg, who); err != nil {
		return uuid.Nil, err
	}
	ticketID, clientID := t.ID, t.ClientID
	if err := q.FinishInboundEmail(ctx, db.FinishInboundEmailParams{
		ID: row.ID, Status: "processed", Reason: "replied", ClientID: &clientID, TicketID: &ticketID,
	}); err != nil {
		return uuid.Nil, fmt.Errorf("enregistrement du tri : %w", err)
	}
	return ticketID, tx.Commit(ctx)
}

// Dismiss ecarte un e-mail a trier : publicite, erreur d'adresse.
func (s *InboundService) Dismiss(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.LockInboundEmail(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if row.Status != "held" {
		return ErrInboundNotHeld
	}
	if err := q.FinishInboundEmail(ctx, db.FinishInboundEmailParams{
		ID: row.ID, Status: "ignored", Reason: "dismissed", ClientID: row.ClientID, TicketID: row.TicketID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
