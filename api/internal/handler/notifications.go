package handler

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/middleware"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

// NotificationService est le contrat dont les endpoints ont besoin.
type NotificationService interface {
	Feed(ctx context.Context, userID uuid.UUID, before *time.Time) (usecase.NotificationFeed, error)
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
}

// NotificationStream ouvre un flux vers les notifications d'un compte.
type NotificationStream interface {
	Subscribe(ctx context.Context, userID uuid.UUID) (<-chan []byte, func(), error)
}

// Notifications porte le panneau de notifications et son flux.
type Notifications struct {
	svc    NotificationService
	stream NotificationStream
}

func NewNotifications(svc NotificationService, stream NotificationStream) *Notifications {
	return &Notifications{svc: svc, stream: stream}
}

// List rend le panneau : les dernieres notifications et le compteur de non-lues.
func (h *Notifications) List(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	var before *time.Time

	if raw := c.Query("before"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{
				"before": "Date invalide, format attendu RFC 3339",
			})
		}

		before = &parsed
	}

	feed, err := h.svc.Feed(c.Context(), userID, before)
	if err != nil {
		return err
	}

	return c.JSON(feed)
}

// MarkRead marque une notification comme lue.
func (h *Notifications) MarkRead(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.MarkRead(c.Context(), userID, id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// MarkAllRead vide le compteur.
func (h *Notifications) MarkAllRead(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	if err := h.svc.MarkAllRead(c.Context(), userID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// streamKeepAlive borne l'attente entre deux signes de vie.
//
// Sans trafic, un intermediaire finit par couper une connexion qu'il croit
// morte. Un commentaire SSE — une ligne commencant par « : » — passe pour du
// bruit cote client et suffit a tenir le tuyau ouvert.
const streamKeepAlive = 25 * time.Second

// Stream pousse les notifications au fil de l'eau.
//
// SSE et non WebSocket : le trafic ne va que du serveur vers le navigateur, et
// SSE s'en tient a une reponse HTTP qui ne se termine pas — il traverse les
// proxys sans reglage, et le navigateur se reconnecte tout seul. Un WebSocket
// demanderait une montee en charge bidirectionnelle dont personne n'a l'usage
// ici.
func (h *Notifications) Stream(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	// Coupe la mise en tampon des proxys : sans cela nginx garde les messages
	// jusqu'a remplir son bloc, et le « temps reel » arrive par paquets.
	c.Set("X-Accel-Buffering", "no")

	// Le contexte de la requete est clos des que le handler rend la main, alors
	// que le flux vit apres. On suit donc la fermeture du client, signalee par
	// l'ecriture qui echoue.
	ctx, cancel := context.WithCancel(context.Background())

	messages, release, err := h.stream.Subscribe(ctx, userID)
	if err != nil {
		cancel()

		return fmt.Errorf("ouverture du flux : %w", err)
	}

	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer cancel()
		defer release()

		// Un premier message des l'ouverture : le navigateur sait que le flux
		// est etabli, et EventSource passe en « open » sans attendre un geste
		// de quelqu'un d'autre.
		if _, err := fmt.Fprint(w, ": ouvert\n\n"); err != nil {
			return
		}
		if err := w.Flush(); err != nil {
			return
		}

		ticker := time.NewTicker(streamKeepAlive)
		defer ticker.Stop()

		for {
			select {
			case message, open := <-messages:
				if !open {
					return
				}

				if _, err := fmt.Fprintf(w, "event: notification\ndata: %s\n\n", message); err != nil {
					return
				}

			case <-ticker.C:
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
					return
				}
			}

			// L'echec d'ecriture est le seul signal fiable qu'un client SSE est
			// parti : il n'envoie rien, et la connexion reste ouverte cote
			// serveur jusqu'a ce qu'on tente de lui parler.
			if err := w.Flush(); err != nil {
				return
			}
		}
	})

	return nil
}
