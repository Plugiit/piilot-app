// Package mail compose et envoie les e-mails de Piilot.
//
// La composition (modeles, en texte brut et en HTML) et l'envoi (SMTP) sont
// separes : les usecases composent et deposent dans la file d'envoi, une tache
// de fond envoie. Aucun e-mail ne part pendant une requete HTTP.
//
// SMTP generique plutot que l'API d'un fournisseur : n'importe quel service
// (Brevo, Scaleway, Mailjet, un serveur de l'agence…) se branche par cinq
// variables, et Piilot ne depend d'aucun d'eux.
package mail

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	"time"
)

//go:embed templates/*.html
var templates embed.FS

// Message est un e-mail compose, pret a deposer dans la file.
type Message struct {
	Kind    string
	To      string
	Subject string
	Text    string
	HTML    string
}

// paris : les dates des e-mails sont celles de l'agence, quel que soit le
// fuseau du serveur.
var paris = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// dateTime ecrit « 9 octobre 2026 à 14 h 30 ».
func dateTime(t time.Time) string {
	months := []string{"janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	t = t.In(paris)

	return fmt.Sprintf("%d %s %d à %d h %02d", t.Day(), months[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

// view est ce que les modeles recoivent.
type view struct {
	Subject    string
	Title      string
	Action     string
	URL        string
	Expires    string
	Firstname  string
	Inviter    string
	ClientName string
}

// Invitation compose l'e-mail d'invitation.
func Invitation(to, firstname, inviter, clientName, url string, expires time.Time) (Message, error) {
	v := view{
		Subject:    "Votre invitation à rejoindre Piilot",
		Title:      "Votre compte Piilot vous attend",
		Action:     "Activer mon compte",
		URL:        url,
		Expires:    dateTime(expires),
		Firstname:  firstname,
		Inviter:    inviter,
		ClientName: clientName,
	}

	text := fmt.Sprintf("Bonjour%s,\n\n%s%s. Choisissez votre mot de passe pour activer votre compte :\n\n%s\n\nCe lien est valable jusqu'au %s. Il ne sert qu'une fois.\n",
		prefixed(firstname),
		ifElse(inviter != "", inviter+" vous invite à rejoindre Piilot", "Une invitation à rejoindre Piilot vous attend"),
		ifElse(clientName != "", ", pour suivre les projets de "+clientName, ""),
		url, v.Expires)

	return compose("invitation", to, v, "invitation.html", text)
}

// PasswordReset compose l'e-mail de reinitialisation du mot de passe.
func PasswordReset(to, firstname, url string, expires time.Time) (Message, error) {
	v := view{
		Subject:   "Réinitialisation de votre mot de passe Piilot",
		Title:     "Choisir un nouveau mot de passe",
		Action:    "Choisir un nouveau mot de passe",
		URL:       url,
		Expires:   dateTime(expires),
		Firstname: firstname,
	}

	text := fmt.Sprintf("Bonjour%s,\n\nUne réinitialisation du mot de passe de votre compte Piilot a été demandée. Choisissez-en un nouveau ici :\n\n%s\n\nCe lien est valable jusqu'au %s et ne sert qu'une fois.\n\nVous n'êtes pas à l'origine de cette demande ? Ignorez cet e-mail : votre mot de passe actuel reste valable.\n",
		prefixed(firstname), url, v.Expires)

	return compose("password_reset", to, v, "password_reset.html", text)
}

func compose(kind, to string, v view, content, text string) (Message, error) {
	tpl, err := htmltemplate.ParseFS(templates, "templates/layout.html", "templates/"+content)
	if err != nil {
		return Message{}, fmt.Errorf("modele %s : %w", content, err)
	}

	var html bytes.Buffer
	if err := tpl.ExecuteTemplate(&html, "layout", v); err != nil {
		return Message{}, fmt.Errorf("rendu de %s : %w", content, err)
	}

	return Message{Kind: kind, To: to, Subject: v.Subject, Text: text, HTML: html.String()}, nil
}

func prefixed(firstname string) string {
	if strings.TrimSpace(firstname) == "" {
		return ""
	}
	return " " + firstname
}

func ifElse(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}
