// Commande seed : cree un compte depuis le terminal.
//
// Sans elle, la table users d'un deploiement neuf est vide et personne ne peut
// se connecter. C'est l'amorcage du systeme, pas un outil d'administration :
// la gestion courante des comptes passera par l'API.
//
// Deux usages :
//
//	create-admin
//	    Interactif. Pose les questions, masque le mot de passe a la saisie et
//	    cree un compte admin. C'est le meme binaire, appele sous ce nom (lien
//	    symbolique installe dans l'image).
//
//	SEED_PASSWORD='...' seed -email=moi@plugiit.com [-firstname=...] [-lastname=...] [-role=team]
//	    Scriptable. Sans SEED_PASSWORD, un mot de passe est genere et affiche
//	    une seule fois.
//
// Lance sans -email depuis un terminal, seed passe lui aussi en interactif.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/mail"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

// account regroupe ce qu'il faut pour creer un compte.
type account struct {
	email, firstname, lastname, role string
	hash                             string
	// generated n'est renseigne que si le mot de passe a ete tire au
	// hasard : il faut alors l'afficher, une seule fois.
	generated string
}

func run() error {
	email := flag.String("email", "", "adresse e-mail du compte")
	firstname := flag.String("firstname", "", "prenom")
	lastname := flag.String("lastname", "", "nom")
	role := flag.String("role", "admin", "code du role (admin, team, client)")
	flag.Parse()

	asCreateAdmin := filepath.Base(os.Args[0]) == "create-admin"
	interactive := asCreateAdmin || (strings.TrimSpace(*email) == "" && isTerminal(os.Stdin))

	if !interactive && strings.TrimSpace(*email) == "" {
		flag.Usage()
		return errors.New("-email est obligatoire hors d'un terminal interactif")
	}

	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL est obligatoire")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Connexion avant les questions : une base injoignable se signale tout de
	// suite, pas apres avoir tout saisi.
	pool, err := repository.NewPool(ctx, databaseURL, repository.DefaultPoolConfig())
	if err != nil {
		return err
	}
	defer pool.Close()

	q := db.New(pool)

	acc := account{
		email:     strings.TrimSpace(*email),
		firstname: strings.TrimSpace(*firstname),
		lastname:  strings.TrimSpace(*lastname),
		role:      *role,
	}
	if asCreateAdmin {
		acc.role = "admin"
	}

	// Le role doit exister : la contrainte de cle etrangere le refuserait de
	// toute facon, mais avec un message Postgres illisible.
	if _, err := q.GetRoleByCode(ctx, acc.role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("role inconnu : %q", acc.role)
		}
		return fmt.Errorf("lecture du role : %w", err)
	}

	emailFree := func(address string) error {
		// Refus plutot qu'ecrasement : reinitialiser un mot de passe est une
		// operation d'administration, pas d'amorcage.
		if _, err := q.GetUserByEmail(ctx, address); err == nil {
			return fmt.Errorf("un compte actif utilise déjà %s", address)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("vérification de l'adresse : %w", err)
		}
		return nil
	}

	if interactive {
		if err := ask(&acc, emailFree); err != nil {
			return err
		}
	} else {
		if err := emailFree(acc.email); err != nil {
			return err
		}
		if err := passwordFromEnv(&acc); err != nil {
			return err
		}
	}

	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        acc.email,
		PasswordHash: acc.hash,
		Firstname:    acc.firstname,
		Lastname:     acc.lastname,
		Role:         acc.role,
	})
	if err != nil {
		return fmt.Errorf("création du compte : %w", err)
	}

	fmt.Printf("\nCompte créé\n  id    %s\n  email %s\n  rôle  %s\n", user.ID, user.Email, user.Role)
	if acc.generated != "" {
		fmt.Printf("  mot de passe %s\n\nAffiché une seule fois : il n'est stocké que haché.\n", acc.generated)
	}

	return nil
}

// ask complete le compte par des questions. Chaque reponse invalide est
// redemandee plutot que d'abandonner toute la saisie.
func ask(acc *account, emailFree func(string) error) error {
	in := bufio.NewReader(os.Stdin)

	fmt.Printf("Création d'un compte %s\n\n", acc.role)

	// Adresse passee en drapeau : verifiee tout de suite, pas redemandee.
	if acc.email != "" {
		if err := emailFree(acc.email); err != nil {
			return err
		}
	}
	for acc.email == "" {
		answer, err := prompt(in, "Adresse e-mail : ")
		if err != nil {
			return err
		}
		if _, err := mail.ParseAddress(answer); err != nil || !strings.Contains(answer, "@") {
			fmt.Println("  Adresse invalide.")
			continue
		}
		if err := emailFree(answer); err != nil {
			fmt.Printf("  %v\n", err)
			continue
		}
		acc.email = answer
	}
	if acc.firstname == "" {
		answer, err := prompt(in, "Prénom : ")
		if err != nil {
			return err
		}
		acc.firstname = answer
	}
	if acc.lastname == "" {
		answer, err := prompt(in, "Nom : ")
		if err != nil {
			return err
		}
		acc.lastname = answer
	}

	// SEED_PASSWORD reste prioritaire, meme en interactif : utile pour un
	// script qui simule un terminal.
	if os.Getenv("SEED_PASSWORD") != "" {
		return passwordFromEnv(acc)
	}

	for {
		password, err := secret(in, fmt.Sprintf("Mot de passe (%d caractères minimum) : ", security.MinPasswordLength))
		if err != nil {
			return err
		}
		hash, err := security.HashPassword(password)
		if err != nil {
			fmt.Printf("  %v\n", err)
			continue
		}
		confirm, err := secret(in, "Confirmation : ")
		if err != nil {
			return err
		}
		if confirm != password {
			fmt.Println("  Les deux saisies diffèrent, on recommence.")
			continue
		}
		acc.hash = hash
		return nil
	}
}

// passwordFromEnv prend le mot de passe dans SEED_PASSWORD, ou en tire un au
// hasard. Il ne passe jamais par un drapeau : il resterait dans l'historique
// du shell et serait lisible dans ps par tout utilisateur de la machine.
func passwordFromEnv(acc *account) error {
	password := os.Getenv("SEED_PASSWORD")
	if password == "" {
		// 24 octets d'alea, soit 32 caracteres en base64 : bien au-dela de ce
		// qu'un humain choisirait, et sans cout puisque personne ne le retient.
		buf := make([]byte, 24)
		if _, err := rand.Read(buf); err != nil {
			return fmt.Errorf("génération du mot de passe : %w", err)
		}
		password = base64.RawURLEncoding.EncodeToString(buf)
		acc.generated = password
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	acc.hash = hash
	return nil
}

func prompt(in *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := in.ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", errors.New("saisie interrompue")
	}
	return strings.TrimSpace(line), nil
}

// secret lit une ligne sans l'afficher. L'echo du terminal est coupe par
// stty, present dans l'image Alpine comme sur macOS et Linux : pas de
// dependance pour une seule saisie. Si stty echoue, la saisie reste visible
// et on le dit.
func secret(in *bufio.Reader, label string) (string, error) {
	// L'echo est coupe avant d'afficher la question : une frappe rapide, ou
	// un collage, arriverait sinon avant que stty ait agi, et s'afficherait.
	restore := func() {}
	if isTerminal(os.Stdin) && stty("-echo") == nil {
		// Un Ctrl-C pendant la saisie laisserait le terminal muet : l'echo
		// est retabli avant de sortir.
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			select {
			case <-signals:
				_ = stty("echo")
				fmt.Println()
				os.Exit(130)
			case <-done:
			}
		}()
		restore = func() {
			close(done)
			signal.Stop(signals)
			_ = stty("echo")
		}
	} else if isTerminal(os.Stdin) {
		label += "(saisie visible) "
	}

	fmt.Print(label)
	line, err := in.ReadString('\n')
	restore()
	fmt.Println()
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", errors.New("saisie interrompue")
	}
	// Seul le retour a la ligne est retire : un espace fait partie du mot de
	// passe choisi.
	return strings.TrimRight(line, "\r\n"), nil
}

func stty(arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
