// Commande bot-key : cree, liste et revoque les cles du bot Discord.
//
// A lancer depuis le terminal du conteneur (Coolify : ressource, onglet
// Terminal, conteneur « server »). Pas d'ecran : c'est un geste rare, fait par
// qui administre l'instance.
//
//	bot-key create <nom>   tire une cle, montree une seule fois
//	bot-key list           les cles, actives d'abord
//	bot-key revoke <id>    retire une cle, refusee des la requete suivante
//
// Plusieurs cles actives a la fois : pour tourner sans coupure, creer la
// nouvelle, la donner au bot, puis revoquer l'ancienne.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

const usage = `Usage :
  bot-key create <nom>   tire une clé, montrée une seule fois
  bot-key list           liste les clés
  bot-key revoke <id>    révoque une clé

Le nom : minuscules, chiffres, point, tiret ou souligné (ex. discord-prod).
`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "erreur : %s\n", describe(err))
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(out, usage)
		return errors.New("sous-commande manquante")
	}

	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL est obligatoire")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	pool, err := repository.NewPool(ctx, databaseURL, repository.DefaultPoolConfig())
	if err != nil {
		return err
	}
	defer pool.Close()

	keys := usecase.NewBotKeyService(pool)

	switch args[0] {
	case "create":
		if len(args) != 2 {
			fmt.Fprint(out, usage)
			return errors.New("create attend un nom")
		}
		raw, key, err := keys.Create(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Clé créée\n  id    %s\n  nom   %s\n  rôle  %s\n\n  %s\n\n", key.ID, key.Name, key.Role, raw)
		fmt.Fprintln(out, "Affichée une seule fois : seule son empreinte est stockée.")
		fmt.Fprintln(out, "Rangez-la dans les secrets du bot, jamais dans un dépôt.")
		return nil

	case "list":
		list, err := keys.List(ctx)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(out, "Aucune clé.")
			return nil
		}
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNOM\tCRÉÉE\tDERNIER USAGE\tÉTAT")
		for _, k := range list {
			state := "active"
			if k.RevokedAt != nil {
				state = "révoquée le " + k.RevokedAt.Local().Format("2006-01-02 15:04")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", k.ID, k.Name, k.CreatedAt.Local().Format("2006-01-02 15:04"), lastUsed(k.LastUsedAt), state)
		}
		return w.Flush()

	case "revoke":
		if len(args) != 2 {
			fmt.Fprint(out, usage)
			return errors.New("revoke attend l'identifiant d'une clé (bot-key list)")
		}
		id, err := uuid.Parse(args[1])
		if err != nil {
			return errors.New("identifiant invalide : prenez-le dans bot-key list")
		}
		name, err := keys.Revoke(ctx, id)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Clé « %s » révoquée : elle est refusée dès la requête suivante.\n", name)
		return nil

	default:
		fmt.Fprint(out, usage)
		return fmt.Errorf("sous-commande inconnue : %q", args[0])
	}
}

func lastUsed(t *time.Time) string {
	if t == nil {
		return "jamais"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// describe rend une erreur metier lisible au terminal : son message et, pour
// une validation, ce qui ne va pas.
func describe(err error) string {
	if e, ok := domain.AsError(err); ok {
		msg := e.Message
		for field, detail := range e.Details {
			msg += fmt.Sprintf("\n  %s : %v", field, detail)
		}
		return msg
	}
	return err.Error()
}
