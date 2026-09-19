// Commande seed : cree le premier compte de la plateforme.
//
// Sans elle, la table users est vide et personne ne peut se connecter, meme
// avec /auth/login parfaitement implemente. C'est l'amorcage du systeme, pas un
// outil d'administration : la gestion des comptes passera par l'API.
//
//	make seed EMAIL=moi@plugiit.com
//	SEED_PASSWORD='...' make seed EMAIL=moi@plugiit.com
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"github.com/plugiit/piilot-app/api/internal/repository"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erreur : %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	email := flag.String("email", "", "adresse e-mail du compte (obligatoire)")
	firstname := flag.String("firstname", "", "prenom")
	lastname := flag.String("lastname", "", "nom")
	role := flag.String("role", "admin", "code du role (admin, team, client)")
	flag.Parse()

	if strings.TrimSpace(*email) == "" {
		flag.Usage()
		return errors.New("-email est obligatoire")
	}

	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL est obligatoire")
	}

	// Le mot de passe ne passe jamais par un drapeau de ligne de commande : il
	// resterait dans l'historique du shell et serait lisible dans ps par tout
	// utilisateur de la machine.
	password, generated, err := resolvePassword()
	if err != nil {
		return err
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := repository.NewPool(ctx, databaseURL, repository.DefaultPoolConfig())
	if err != nil {
		return err
	}
	defer pool.Close()

	q := db.New(pool)

	// Le role doit exister : la contrainte de cle etrangere le refuserait de
	// toute facon, mais avec un message Postgres illisible.
	if _, err := q.GetRoleByCode(ctx, *role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("role inconnu : %q", *role)
		}
		return fmt.Errorf("lecture du role : %w", err)
	}

	// Refus plutot qu'ecrasement : reinitialiser un mot de passe est une
	// operation d'administration, pas d'amorcage.
	if _, err := q.GetUserByEmail(ctx, strings.TrimSpace(*email)); err == nil {
		return fmt.Errorf("un compte actif utilise deja %q", *email)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("verification de l'email : %w", err)
	}

	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        strings.TrimSpace(*email),
		PasswordHash: hash,
		Firstname:    *firstname,
		Lastname:     *lastname,
		Role:         *role,
	})
	if err != nil {
		return fmt.Errorf("creation du compte : %w", err)
	}

	fmt.Printf("compte cree\n  id    %s\n  email %s\n  role  %s\n", user.ID, user.Email, user.Role)

	if generated {
		fmt.Printf("  mot de passe %s\n\nNote : affiche une seule fois, il n'est stocke que hache.\n", password)
	}

	return nil
}

// resolvePassword prend le mot de passe dans l'environnement, ou en tire un au
// hasard. Le booleen indique s'il a ete genere, donc s'il faut l'afficher.
func resolvePassword() (string, bool, error) {
	if fromEnv := os.Getenv("SEED_PASSWORD"); fromEnv != "" {
		return fromEnv, false, nil
	}

	// 24 octets d'alea, soit 32 caracteres en base64 : bien au-dela de ce
	// qu'un humain choisirait, et sans cout puisque personne ne le retient.
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", false, fmt.Errorf("generation du mot de passe : %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), true, nil
}
