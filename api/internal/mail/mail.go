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

	ProjectName string
	Deliverable string
	Version     int
	IsFirst     bool

	Lead   string
	Quote  string
	Footer string
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

// DeliverableSubmitted previent un compte du portail qu'un livrable attend sa
// reponse : un nouveau livrable, ou une nouvelle version apres des retours.
func DeliverableSubmitted(to, firstname, projectName, deliverable string, version int, url string) (Message, error) {
	first := version <= 1
	v := view{
		Subject:     ifElse(first, "Nouveau livrable à valider : ", "Nouvelle version à valider : ") + deliverable,
		Title:       ifElse(first, "Un livrable vous attend", "Une nouvelle version vous attend"),
		Action:      "Voir le livrable",
		URL:         url,
		Firstname:   firstname,
		ProjectName: projectName,
		Deliverable: deliverable,
		Version:     version,
		IsFirst:     first,
	}

	text := fmt.Sprintf("Bonjour%s,\n\n%s pour le projet %s : « %s »%s.\n\nConsultez-le dans votre espace client, puis validez-le ou dites-nous ce qui doit changer :\n\n%s\n",
		prefixed(firstname),
		ifElse(first, "Un nouveau livrable vous attend", "Une nouvelle version vous attend"),
		projectName, deliverable,
		ifElse(first, "", fmt.Sprintf(", version %d", version)),
		url)

	return compose("deliverable_submitted", to, v, "deliverable_submitted.html", text)
}

// DeliverableReminder relance un client dont un livrable attend la reponse
// depuis `days` jours. Envoye une fois par version, par la tache de fond.
func DeliverableReminder(to, firstname, projectName, deliverable string, version int, days int, url string) (Message, error) {
	first := version <= 1
	since := fmt.Sprintf("%d jour%s", days, ifElse(days > 1, "s", ""))
	v := view{
		Subject:     "Un livrable attend votre réponse : " + deliverable,
		Title:       "Un livrable attend votre réponse",
		Action:      "Voir le livrable",
		URL:         url,
		Firstname:   firstname,
		ProjectName: projectName,
		Deliverable: deliverable,
		Version:     version,
		IsFirst:     first,
		Lead:        since,
	}

	text := fmt.Sprintf("Bonjour%s,\n\nLe livrable « %s »%s du projet %s attend votre réponse depuis %s.\n\nUn coup d'œil suffit : validez-le, ou dites-nous ce qui doit changer :\n\n%s\n",
		prefixed(firstname), deliverable,
		ifElse(first, "", fmt.Sprintf(", version %d,", version)),
		projectName, since, url)

	return compose("deliverable_reminder", to, v, "deliverable_reminder.html", text)
}

// TicketToClient previent la personne qui a ouvert une demande : l'agence a
// repondu, ou la demande a change de statut. `status` est le libelle du
// nouveau statut, vide quand il n'a pas bouge ; `quote`, la reponse, vide
// quand il n'y en a pas de publique.
func TicketToClient(to, firstname string, numero int64, subject, quote, status, url string) (Message, error) {
	ref := fmt.Sprintf("#%d « %s »", numero, subject)

	var title, lead string
	switch {
	case quote != "" && status != "":
		title = fmt.Sprintf("Réponse sur votre demande #%d", numero)
		lead = fmt.Sprintf("L'agence a répondu à votre demande %s, qui passe au statut « %s ».", ref, status)
	case quote != "":
		title = fmt.Sprintf("Réponse sur votre demande #%d", numero)
		lead = fmt.Sprintf("L'agence a répondu à votre demande %s.", ref)
	default:
		title = fmt.Sprintf("Votre demande #%d : %s", numero, strings.ToLower(status))
		lead = fmt.Sprintf("Votre demande %s passe au statut « %s ».", ref, status)
	}

	v := view{
		Subject:   title + " — " + subject,
		Title:     title,
		Action:    "Voir la demande",
		URL:       url,
		Firstname: firstname,
		Lead:      lead,
		Quote:     quote,
		Footer:    "Vous pouvez répondre directement depuis votre espace client.",
	}

	text := fmt.Sprintf("Bonjour%s,\n\n%s\n%s\nVoir la demande et répondre :\n\n%s\n",
		prefixed(firstname), lead, ifElse(quote != "", "\n« "+quote+" »\n", ""), url)

	return compose("ticket_client", to, v, "ticket.html", text)
}

// TicketToTeam previent l'agence d'une demande deposee dans le portail, ou de
// la reponse d'un client.
func TicketToTeam(to, firstname string, created bool, numero int64, subject, project, client, author, quote, url string) (Message, error) {
	ref := fmt.Sprintf("#%d « %s »", numero, subject)
	who := ifElse(author != "", author+" ("+client+")", client)

	var title, lead string
	if created {
		title = fmt.Sprintf("Nouvelle demande #%d de %s", numero, client)
		lead = fmt.Sprintf("%s a ouvert la demande %s sur le projet %s.", who, ref, project)
	} else {
		title = fmt.Sprintf("Réponse du client sur #%d", numero)
		lead = fmt.Sprintf("%s a répondu sur la demande %s.", who, ref)
	}

	v := view{
		Subject:   title + " — " + subject,
		Title:     title,
		Action:    "Ouvrir le ticket",
		URL:       url,
		Firstname: firstname,
		Lead:      lead,
		Quote:     quote,
		Footer:    "Répondez depuis Piilot : le client voit votre réponse dans son espace et la reçoit par e-mail.",
	}

	text := fmt.Sprintf("Bonjour%s,\n\n%s\n%s\nOuvrir le ticket :\n\n%s\n",
		prefixed(firstname), lead, ifElse(quote != "", "\n« "+quote+" »\n", ""), url)

	return compose("ticket_team", to, v, "ticket.html", text)
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
