package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// PortalDashboard est l'accueil du portail : ou en sont les projets, ce qui
// attend le client, ce que l'agence a livre, ce qui s'est passe recemment.
// Un appel, tout l'ecran.
type PortalDashboard struct {
	// Moyenne de l'avancement des projets qui ont une echeance a tenir (ni
	// livres, ni heberges). Nul sans projet en cours.
	ProgressAverage *int `json:"progress_average"`
	ActiveProjects  int  `json:"active_projects"`
	// Livrables qui attendent la reponse du client, et ceux valides ce mois.
	DeliverablesPending  int `json:"deliverables_pending"`
	ValidatedThisMonth   int `json:"validated_this_month"`
	TicketsResolvedMonth int `json:"tickets_resolved_this_month"`
	// Demandes resolues par jour, du plus ancien a aujourd'hui ; un jour sans
	// rien vaut zero. Trente jours affiches, plus six pour amorcer le lissage
	// du bord gauche.
	TicketDays []PortalTicketDay `json:"ticket_days"`
	// Le jalon non atteint le plus proche, tous projets confondus.
	NextMilestone *PortalDashboardMilestone `json:"next_milestone"`
	// Tous les projets du client : le selecteur et « Autres projets ».
	Projects []PortalProject   `json:"projects"`
	Tickets  PortalTicketStats `json:"tickets"`
	// Le projet suivi : celui demande, ou le plus recemment actif. Nul sans
	// projet.
	Focus *PortalProjectDetail `json:"focus"`
	// Ses demandes, les plus recemment actives d'abord.
	FocusTickets []PortalTicket `json:"focus_tickets"`
	// Ce qui attend le client, tous projets confondus.
	Actions []PortalAction `json:"actions"`
	// L'historique du projet suivi.
	Activity []PortalActivityItem `json:"activity"`
}

// PortalAction est une action requise du client.
type PortalAction struct {
	// deliverable : un livrable a valider ; project_waiting : un projet en
	// attente de son retour.
	Kind    string         `json:"kind"`
	Title   string         `json:"title"`
	Project DeliverableRef `json:"project"`
	// Le livrable, ou le projet.
	RefID uuid.UUID `json:"ref_id"`
	// Depuis quand elle attend.
	Since *time.Time `json:"since"`
	// Elle attend depuis plus de trois jours.
	Overdue bool `json:"overdue"`
}

// actionOverdue : au-dela, une action requise passe en rouge — le delai de
// la relance automatique des livrables.
const actionOverdue = 72 * time.Hour

// PortalDashboardMilestone est la prochaine etape, et son projet.
type PortalDashboardMilestone struct {
	Title   string         `json:"title"`
	DueOn   *string        `json:"due_on"`
	Project DeliverableRef `json:"project"`
}

// PortalTicketDay est un point de la courbe des demandes resolues.
type PortalTicketDay struct {
	// AAAA-MM-JJ.
	Day      string `json:"day"`
	Resolved int    `json:"resolved"`
}

// ticketDays : la fenetre de la courbe des demandes, 30 jours affiches et 6
// d'amorce pour le lissage. Au mois, un client dont toutes les
// demandes tombaient dans le meme mois n'y voyait qu'un point.
const ticketDays = 36

// PortalTicketStats compte les demandes du client par etat, dans ses mots.
type PortalTicketStats struct {
	Received   int `json:"received"`
	InProgress int `json:"in_progress"`
	Ready      int `json:"ready"`
	Resolved   int `json:"resolved"`
}

// PortalActivityItem est une entree du fil d'activite.
type PortalActivityItem struct {
	// deliverable_submitted, deliverable_validated, milestone_reached,
	// deployment ou ticket_replied.
	Kind    string         `json:"kind"`
	At      time.Time      `json:"at"`
	Title   string         `json:"title"`
	Project DeliverableRef `json:"project"`
	// Le livrable, le jalon, la mise en ligne ou la demande concernee.
	RefID uuid.UUID `json:"ref_id"`
	// Numero de version d'un livrable, ou de la demande.
	Number int `json:"number"`
}

// Dashboard rend l'accueil du portail, centre sur un projet : celui demande,
// ou le plus recemment actif. Les compteurs viennent des declencheurs
// (projets, demandes, activite mensuelle) : rien n'y est recompte.
func (s *PortalService) Dashboard(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) (PortalDashboard, error) {
	clientID, err := s.q.PortalClientOf(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && clientID == nil) {
		return PortalDashboard{}, domain.ErrNotFound
	}
	if err != nil {
		return PortalDashboard{}, fmt.Errorf("lecture du compte : %w", err)
	}

	projects, err := s.Projects(ctx, userID)
	if err != nil {
		return PortalDashboard{}, err
	}

	out := PortalDashboard{
		Projects:     projects.Items,
		FocusTickets: []PortalTicket{},
		Actions:      []PortalAction{},
		Activity:     []PortalActivityItem{},
	}

	sum, tracked := 0, 0
	var nextDue *time.Time
	for _, p := range projects.Items {
		out.DeliverablesPending += p.DeliverablesPending
		if p.Status == "cadrage" || p.Status == "production" || p.Status == "attente" {
			out.ActiveProjects++
			sum += p.Progress
			tracked++
		}
		if p.NextMilestone == nil {
			continue
		}
		// Le jalon date le plus proche ; a defaut de date, le premier venu.
		var due *time.Time
		if p.NextMilestone.DueOn != nil {
			if t, err := time.Parse(dateLayout, *p.NextMilestone.DueOn); err == nil {
				due = &t
			}
		}
		if out.NextMilestone == nil || (due != nil && (nextDue == nil || due.Before(*nextDue))) {
			out.NextMilestone = &PortalDashboardMilestone{
				Title: p.NextMilestone.Title, DueOn: p.NextMilestone.DueOn,
				Project: DeliverableRef{ID: p.ID, Name: p.Name},
			}
			nextDue = due
		}
	}
	if tracked > 0 {
		avg := (sum + tracked/2) / tracked
		out.ProgressAverage = &avg
	}

	stats, err := s.q.GetClientTicketStats(ctx, *clientID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return PortalDashboard{}, fmt.Errorf("demandes du client : %w", err)
	default:
		out.Tickets = PortalTicketStats{
			Received: int(stats.Received), InProgress: int(stats.InProgress), Ready: int(stats.Ready), Resolved: int(stats.Resolved),
		}
	}

	// Le mois en cours : ce que l'agence a valide et resolu.
	today := agencyToday(time.Now())
	month := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	months, err := s.q.ListClientActivityMonths(ctx, db.ListClientActivityMonthsParams{ClientID: *clientID, Since: month})
	if err != nil {
		return PortalDashboard{}, fmt.Errorf("activite du mois : %w", err)
	}
	for _, m := range months {
		if m.Month.Equal(month) {
			out.ValidatedThisMonth, out.TicketsResolvedMonth = int(m.DeliverablesValidated), int(m.TicketsResolved)
		}
	}

	// La courbe : trente-six jours, completes a zero.
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	first := day.AddDate(0, 0, -(ticketDays - 1))
	days, err := s.q.ListClientTicketDays(ctx, db.ListClientTicketDaysParams{ClientID: *clientID, Since: first})
	if err != nil {
		return PortalDashboard{}, fmt.Errorf("demandes resolues par jour : %w", err)
	}
	// Cle texte : deux time.Time egaux peuvent differer de fuseau, et une map
	// les compare a l'identique.
	resolved := make(map[string]int, len(days))
	for _, d := range days {
		resolved[d.Day.Format(dateLayout)] = int(d.Resolved)
	}
	out.TicketDays = make([]PortalTicketDay, 0, ticketDays)
	for d := first; !d.After(day); d = d.AddDate(0, 0, 1) {
		key := d.Format(dateLayout)
		out.TicketDays = append(out.TicketDays, PortalTicketDay{Day: key, Resolved: resolved[key]})
	}

	// Le projet suivi. Un identifiant qui n'est pas a ce client rend
	// « introuvable », comme partout dans le portail.
	var focusID *uuid.UUID
	switch {
	case projectID != nil:
		focusID = projectID
	case len(projects.Items) > 0:
		first := projects.Items[0].ID
		focusID = &first
	}
	if focusID != nil {
		focus, err := s.Project(ctx, userID, *focusID)
		if err != nil {
			return PortalDashboard{}, err
		}
		out.Focus = &focus

		tickets, err := s.q.PortalProjectTickets(ctx, db.PortalProjectTicketsParams{ClientID: *clientID, ProjectID: *focusID})
		if err != nil {
			return PortalDashboard{}, fmt.Errorf("demandes du projet : %w", err)
		}
		for _, t := range tickets {
			out.FocusTickets = append(out.FocusTickets, PortalTicket{
				ID: t.ID, Numero: t.Numero, Subject: t.Subject, Status: t.Status, Open: ticketOpen(t.Status),
				Project: DeliverableRef{ID: focus.ID, Name: focus.Name}, Created: t.CreatedAt, Updated: t.UpdatedAt,
			})
		}
	}

	// Les actions requises : les livrables a valider, puis les projets qui
	// attendent un retour.
	pending, err := s.q.PortalPendingDeliverables(ctx, *clientID)
	if err != nil {
		return PortalDashboard{}, fmt.Errorf("livrables a valider : %w", err)
	}
	now := time.Now()
	for _, d := range pending {
		since := d.SubmittedAt
		out.Actions = append(out.Actions, PortalAction{
			Kind: "deliverable", Title: d.Title, Project: DeliverableRef{ID: d.ProjectID, Name: d.ProjectName},
			RefID: d.ID, Since: &since, Overdue: now.Sub(since) > actionOverdue,
		})
	}
	for _, p := range projects.Items {
		if p.Status == "attente" {
			at := p.LastActivityAt
			out.Actions = append(out.Actions, PortalAction{
				Kind: "project_waiting", Title: p.Name, Project: DeliverableRef{ID: p.ID, Name: p.Name},
				RefID: p.ID, Since: &at, Overdue: now.Sub(at) > actionOverdue,
			})
		}
	}

	feed, err := s.q.PortalRecentActivity(ctx, db.PortalRecentActivityParams{ClientID: *clientID, ProjectID: focusID})
	if err != nil {
		return PortalDashboard{}, fmt.Errorf("activite recente : %w", err)
	}
	for _, f := range feed {
		out.Activity = append(out.Activity, PortalActivityItem{
			Kind: f.Kind, At: f.At, Title: f.Title, Project: DeliverableRef{ID: f.ProjectID, Name: f.ProjectName},
			RefID: f.RefID, Number: int(f.Number),
		})
	}

	return out, nil
}
