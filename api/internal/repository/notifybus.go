package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// NotifyBus porte les notifications jusqu'aux navigateurs connectes.
//
// Redis et non un canal Go : le flux d'un compte est ouvert sur l'instance qui
// a repondu a sa requete, et le geste qui le concerne peut tomber sur une
// autre. Un canal en memoire marcherait tant qu'il n'y a qu'une instance, et
// perdrait des notifications au jour ou il y en aurait deux — panne silencieuse
// et penible a diagnostiquer.
type NotifyBus struct{ rdb *redis.Client }

func NewNotifyBus(rdb *redis.Client) *NotifyBus { return &NotifyBus{rdb: rdb} }

// channel isole les messages d'un destinataire. Un canal par personne plutot
// qu'un canal commun filtre a la lecture : Redis n'envoie alors a chaque
// instance que ce qui la concerne.
func channel(userID uuid.UUID) string { return "notify:" + userID.String() }

// Publish envoie un message aux flux ouverts d'un destinataire.
//
// L'absence d'abonne n'est pas une erreur : personne n'a l'application ouverte,
// et la notification attend en base qu'on vienne la lire.
func (b *NotifyBus) Publish(ctx context.Context, userID uuid.UUID, payload []byte) error {
	if err := b.rdb.Publish(ctx, channel(userID), payload).Err(); err != nil {
		return fmt.Errorf("diffusion de la notification : %w", err)
	}

	return nil
}

// Subscribe ouvre un flux pour un destinataire.
//
// Le canal rendu est ferme quand le contexte l'est. La fonction de liberation
// doit etre appelee par le lecteur : sans elle, l'abonnement Redis survivrait a
// la requete HTTP qui l'a ouvert.
func (b *NotifyBus) Subscribe(ctx context.Context, userID uuid.UUID) (<-chan []byte, func(), error) {
	sub := b.rdb.Subscribe(ctx, channel(userID))

	// Confirme l'abonnement avant de rendre la main : sans cela, un message
	// publie dans la foulee partirait avant que Redis nous ait inscrits.
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()

		return nil, nil, fmt.Errorf("abonnement aux notifications : %w", err)
	}

	out := make(chan []byte)

	go func() {
		defer close(out)

		for message := range sub.Channel() {
			select {
			case out <- []byte(message.Payload):
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, func() { _ = sub.Close() }, nil
}
