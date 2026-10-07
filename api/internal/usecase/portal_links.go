package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// Repondre a un livrable depuis l'e-mail, sans se connecter.
//
// L'e-mail de depot porte deux liens signes — « Valider », « Faire un retour »
// — qui ouvrent une page publique du portail. Le jeton nomme la version et le
// compte destinataire ; la signature prouve qu'il vient de nous, la date de
// peremption le borne dans le temps. Rien n'est decide a l'ouverture du lien :
// un filtre anti-spam qui suit les liens d'un e-mail ne valide rien. C'est le
// geste sur la page, un POST avec le meme jeton, qui enregistre la reponse,
// au nom du compte du jeton et par le meme chemin que depuis le portail.

// ErrReviewLinkExpired : lien perime, altere ou qui ne designe plus la version
// courante.
var ErrReviewLinkExpired = &domain.Error{
	Status: http.StatusGone, Code: "LINK_EXPIRED",
	Message: "Ce lien n'est plus valable : connectez-vous à votre espace pour répondre",
}

// ReviewLinkTTL : un client a un mois pour repondre depuis l'e-mail. Au-dela,
// le portail reste la.
const ReviewLinkTTL = 30 * 24 * time.Hour

// reviewLinkPurpose separe cette cle de toute autre derivee du meme secret.
const reviewLinkPurpose = "deliverable-review"

// PortalReview est la page publique de reponse a un livrable.
type PortalReview struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ProjectName string    `json:"project_name"`
	Milestone   *string   `json:"milestone"`
	// Prenom du compte qui repond, pour que la page dise a qui elle parle.
	Firstname string `json:"firstname"`
	// La version que le lien designe.
	Version int        `json:"version"`
	URL     string     `json:"url"`
	FileID  *uuid.UUID `json:"file_id"`
	// Etat de cette version : en_attente tant que personne n'a repondu.
	Status    string     `json:"status"`
	DecidedAt *time.Time `json:"decided_at"`
}

// SetLinkSecret donne la cle des liens signes. Sans cle, les liens ne sont ni
// produits ni acceptes.
func (s *PortalService) SetLinkSecret(secret []byte) {
	s.linkKey = security.DeriveKey(secret, reviewLinkPurpose)
}

// ReviewLinks produit les deux liens d'un e-mail de depot pour un
// destinataire : valider, faire un retour. Vides sans cle.
func ReviewLinks(secret []byte, clientURL string, deliverableID, versionID, userID uuid.UUID, now time.Time) (validate, feedback string) {
	if len(secret) == 0 {
		return "", ""
	}
	key := security.DeriveKey(secret, reviewLinkPurpose)
	token := security.SignLink(key, versionID.String()+"|"+userID.String(), now.Add(ReviewLinkTTL))
	base := fmt.Sprintf("%s/client/livrables/%s/repondre?token=%s", strings.TrimRight(clientURL, "/"), deliverableID, token)

	return base + "&decision=valide", base + "&decision=retours"
}

// readReviewLink relit un jeton et rend la version et le compte qu'il nomme.
func (s *PortalService) readReviewLink(token string, now time.Time) (versionID, userID uuid.UUID, err error) {
	if len(s.linkKey) == 0 {
		return uuid.Nil, uuid.Nil, ErrReviewLinkExpired
	}
	payload, err := security.VerifyLink(s.linkKey, token, now)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrReviewLinkExpired
	}
	v, u, ok := strings.Cut(payload, "|")
	if !ok {
		return uuid.Nil, uuid.Nil, ErrReviewLinkExpired
	}
	versionID, err = uuid.Parse(v)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrReviewLinkExpired
	}
	userID, err = uuid.Parse(u)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrReviewLinkExpired
	}

	return versionID, userID, nil
}

// Review rend la page de reponse que le lien designe.
//
// Le compte du jeton doit toujours avoir acces au livrable — un compte ferme
// ou retire du client ne repond plus — et le lien doit designer la version
// courante : une nouvelle version deposee depuis perime les liens de la
// precedente, ses retours n'auraient plus de sens.
func (s *PortalService) Review(ctx context.Context, deliverableID uuid.UUID, token string) (PortalReview, error) {
	versionID, userID, err := s.readReviewLink(token, time.Now())
	if err != nil {
		return PortalReview{}, err
	}

	row, err := s.q.PortalGetDeliverable(ctx, db.PortalGetDeliverableParams{UserID: userID, DeliverableID: deliverableID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PortalReview{}, ErrReviewLinkExpired
		}

		return PortalReview{}, fmt.Errorf("lecture du livrable : %w", err)
	}
	if row.CurrentVersionID == nil || *row.CurrentVersionID != versionID {
		return PortalReview{}, ErrReviewLinkExpired
	}

	versions, err := s.q.ListDeliverableVersions(ctx, deliverableID)
	if err != nil {
		return PortalReview{}, fmt.Errorf("lecture des versions : %w", err)
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return PortalReview{}, fmt.Errorf("lecture du compte : %w", err)
	}

	out := PortalReview{
		ID:          row.ID,
		Title:       row.Title,
		Description: row.Description,
		ProjectName: row.ProjectName,
		Milestone:   row.MilestoneTitle,
		Firstname:   user.Firstname,
		Status:      "en_attente",
	}
	for _, v := range versions {
		if v.ID != versionID {
			continue
		}
		out.Version = int(v.Numero)
		out.URL = v.Url
		out.FileID = v.AttachmentID
		out.Status = v.Decision
		out.DecidedAt = v.DecidedAt
	}

	return out, nil
}

// DecideByLink enregistre la reponse portee par le lien, au nom du compte du
// jeton, par le meme chemin que depuis le portail : memes notifications, meme
// journal, meme refus si la version a deja recu sa reponse.
func (s *PortalService) DecideByLink(ctx context.Context, deliverableID uuid.UUID, token, decision, feedback string) (PortalReview, error) {
	// Review verifie tout ce qui doit l'etre : jeton, acces, version courante.
	review, err := s.Review(ctx, deliverableID, token)
	if err != nil {
		return PortalReview{}, err
	}
	_, userID, err := s.readReviewLink(token, time.Now())
	if err != nil {
		return PortalReview{}, err
	}

	if _, err := s.Decide(ctx, userID, deliverableID, decision, feedback); err != nil {
		return PortalReview{}, err
	}

	review.Status = decision
	now := time.Now()
	review.DecidedAt = &now

	return review, nil
}
