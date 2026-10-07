package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// GitService recoit ce que GitHub et GitLab poussent par webhook, et le fait
// entrer dans Piilot : une pull request rattache et fait avancer les tickets
// et les taches qu'elle nomme ; une mise en ligne clot les tickets fusionnes,
// atteint le jalon de mise en ligne et s'inscrit au journal du client.
//
// Rien ne sort vers GitHub ou GitLab : ils appellent, Piilot ecrit. Pas de
// jeton a garder, un seul secret de webhook pour toute l'installation.
type GitService struct {
	pool       *pgxpool.Pool
	q          *db.Queries
	tickets    *TicketService
	milestones *MilestoneService
	// Secret impose par l'environnement (GIT_WEBHOOK_SECRET) ; vide, celui de
	// la base sert, tire au sort a la premiere ouverture de l'ecran.
	envSecret string
	baseURL   string
	log       *slog.Logger
}

func NewGitService(pool *pgxpool.Pool, tickets *TicketService, milestones *MilestoneService, envSecret, baseURL string, log *slog.Logger) *GitService {
	return &GitService{
		pool: pool, q: db.New(pool), tickets: tickets, milestones: milestones,
		envSecret: strings.TrimSpace(envSecret), baseURL: strings.TrimRight(baseURL, "/"), log: log,
	}
}

// GitSettings est ce que l'ecran de reglage montre : ou pointer le webhook,
// avec quel secret.
type GitSettings struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
	// Defini par GIT_WEBHOOK_SECRET : il ne se change pas depuis l'ecran.
	FromEnv   bool       `json:"from_env"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// PullRequest est une pull request telle qu'un ticket ou une tache la montre.
type PullRequest struct {
	ID       uuid.UUID  `json:"id"`
	Provider string     `json:"provider"`
	Number   int64      `json:"number"`
	Title    string     `json:"title"`
	URL      string     `json:"url"`
	Branch   string     `json:"branch"`
	Author   string     `json:"author"`
	State    string     `json:"state"`
	OpenedAt *time.Time `json:"opened_at"`
	MergedAt *time.Time `json:"merged_at"`
}

// Deployment est une mise en ligne.
type Deployment struct {
	ID          uuid.UUID `json:"id"`
	Provider    string    `json:"provider"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Environment string    `json:"environment"`
	DeployedAt  time.Time `json:"deployed_at"`
}

func pullRequestOf(row db.PullRequest) PullRequest {
	return PullRequest{
		ID: row.ID, Provider: row.Provider, Number: row.Number, Title: row.Title, URL: row.Url,
		Branch: row.Branch, Author: row.Author, State: row.State, OpenedAt: row.OpenedAt, MergedAt: row.MergedAt,
	}
}

func deploymentOf(row db.Deployment) Deployment {
	return Deployment{
		ID: row.ID, Provider: row.Provider, Kind: row.Kind, Name: row.Name, URL: row.Url,
		Environment: row.Environment, DeployedAt: row.DeployedAt,
	}
}

// Secret rend le secret du webhook, en le creant s'il n'existe pas encore.
func (s *GitService) Secret(ctx context.Context) (string, error) {
	if s.envSecret != "" {
		return s.envSecret, nil
	}

	row, err := s.q.GetGitWebhook(ctx)
	if err == nil {
		return row.Secret, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("lecture du secret : %w", err)
	}

	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	if err := s.q.SetGitWebhookSecret(ctx, secret); err != nil {
		return "", fmt.Errorf("ecriture du secret : %w", err)
	}

	return secret, nil
}

// Settings rend de quoi configurer le webhook chez GitHub ou GitLab.
func (s *GitService) Settings(ctx context.Context) (GitSettings, error) {
	secret, err := s.Secret(ctx)
	if err != nil {
		return GitSettings{}, err
	}

	out := GitSettings{WebhookURL: s.baseURL + "/api/v1/hooks/git", Secret: secret, FromEnv: s.envSecret != ""}
	if row, err := s.q.GetGitWebhook(ctx); err == nil && !out.FromEnv {
		out.UpdatedAt = &row.UpdatedAt
	}

	return out, nil
}

// Rotate tire un nouveau secret. Les webhooks configures avec l'ancien
// cessent d'etre acceptes : c'est le but.
func (s *GitService) Rotate(ctx context.Context) (GitSettings, error) {
	if s.envSecret != "" {
		return GitSettings{}, domain.ErrConflict.WithMessage("Le secret est défini par GIT_WEBHOOK_SECRET : changez-le dans l'environnement")
	}

	secret, err := newSecret()
	if err != nil {
		return GitSettings{}, err
	}
	if err := s.q.SetGitWebhookSecret(ctx, secret); err != nil {
		return GitSettings{}, fmt.Errorf("ecriture du secret : %w", err)
	}

	return s.Settings(ctx)
}

func newSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("tirage du secret : %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// PullRequestsOfTicket et PullRequestsOfTask : ce que la fiche montre.
func (s *GitService) PullRequestsOfTicket(ctx context.Context, ticketID uuid.UUID) ([]PullRequest, error) {
	rows, err := s.q.ListPullRequestsOfTicket(ctx, &ticketID)
	if err != nil {
		return nil, fmt.Errorf("pull requests du ticket : %w", err)
	}
	out := make([]PullRequest, 0, len(rows))
	for _, r := range rows {
		out = append(out, pullRequestOf(r))
	}
	return out, nil
}

// Outcome dit ce qu'un evenement a produit, pour la reponse au webhook et
// les journaux.
type Outcome struct {
	Handled bool   `json:"handled"`
	Note    string `json:"note"`
}

// --- GitHub ---------------------------------------------------------------

type ghRepository struct {
	HTMLURL string `json:"html_url"`
}

type ghPullRequestEvent struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number    int64      `json:"number"`
		Title     string     `json:"title"`
		Body      *string    `json:"body"`
		HTMLURL   string     `json:"html_url"`
		State     string     `json:"state"`
		Merged    bool       `json:"merged"`
		CreatedAt *time.Time `json:"created_at"`
		MergedAt  *time.Time `json:"merged_at"`
		Head      struct {
			Ref string `json:"ref"`
		} `json:"head"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repository ghRepository `json:"repository"`
}

type ghReleaseEvent struct {
	Action  string `json:"action"`
	Release struct {
		TagName     string     `json:"tag_name"`
		Name        string     `json:"name"`
		HTMLURL     string     `json:"html_url"`
		PublishedAt *time.Time `json:"published_at"`
	} `json:"release"`
	Repository ghRepository `json:"repository"`
}

type ghPushEvent struct {
	Ref        string       `json:"ref"`
	Deleted    bool         `json:"deleted"`
	Compare    string       `json:"compare"`
	Repository ghRepository `json:"repository"`
}

type ghDeploymentStatusEvent struct {
	DeploymentStatus struct {
		State          string `json:"state"`
		Environment    string `json:"environment"`
		TargetURL      string `json:"target_url"`
		EnvironmentURL string `json:"environment_url"`
	} `json:"deployment_status"`
	Deployment struct {
		SHA         string `json:"sha"`
		Ref         string `json:"ref"`
		Environment string `json:"environment"`
	} `json:"deployment"`
	Repository ghRepository `json:"repository"`
}

// HandleGitHub traite un evenement GitHub deja authentifie.
func (s *GitService) HandleGitHub(ctx context.Context, event string, body []byte) (Outcome, error) {
	switch event {
	case "ping":
		return Outcome{Handled: true, Note: "pong"}, nil

	case "pull_request":
		var p ghPullRequestEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return Outcome{}, domain.ErrValidation.WithMessage("Charge GitHub illisible")
		}
		state := "closed"
		switch {
		case p.PullRequest.Merged:
			state = "merged"
		case p.PullRequest.State == "open":
			state = "open"
		}
		desc := ""
		if p.PullRequest.Body != nil {
			desc = *p.PullRequest.Body
		}
		return s.pullRequest(ctx, incomingPullRequest{
			RepoURL: p.Repository.HTMLURL, Provider: "github", Number: p.PullRequest.Number,
			Title: p.PullRequest.Title, URL: p.PullRequest.HTMLURL, Branch: p.PullRequest.Head.Ref,
			Author: p.PullRequest.User.Login, State: state, Description: desc,
			OpenedAt: p.PullRequest.CreatedAt, MergedAt: p.PullRequest.MergedAt,
		})

	case "release":
		var p ghReleaseEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return Outcome{}, domain.ErrValidation.WithMessage("Charge GitHub illisible")
		}
		if p.Action != "published" {
			return Outcome{Handled: false, Note: "release non publiée"}, nil
		}
		name := p.Release.TagName
		if name == "" {
			name = p.Release.Name
		}
		return s.deployment(ctx, incomingDeployment{
			RepoURL: p.Repository.HTMLURL, Provider: "github", Kind: "release", Name: name,
			URL: p.Release.HTMLURL, At: timeOr(p.Release.PublishedAt),
		})

	case "push":
		var p ghPushEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return Outcome{}, domain.ErrValidation.WithMessage("Charge GitHub illisible")
		}
		tag, ok := strings.CutPrefix(p.Ref, "refs/tags/")
		if !ok || p.Deleted {
			return Outcome{Handled: false, Note: "push sans tag"}, nil
		}
		return s.deployment(ctx, incomingDeployment{
			RepoURL: p.Repository.HTMLURL, Provider: "github", Kind: "tag", Name: tag,
			URL: strings.TrimRight(p.Repository.HTMLURL, "/") + "/releases/tag/" + tag, At: time.Now(),
		})

	case "deployment_status":
		var p ghDeploymentStatusEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return Outcome{}, domain.ErrValidation.WithMessage("Charge GitHub illisible")
		}
		if p.DeploymentStatus.State != "success" {
			return Outcome{Handled: false, Note: "déploiement " + p.DeploymentStatus.State}, nil
		}
		env := p.DeploymentStatus.Environment
		if env == "" {
			env = p.Deployment.Environment
		}
		url := p.DeploymentStatus.EnvironmentURL
		if url == "" {
			url = p.DeploymentStatus.TargetURL
		}
		return s.deployment(ctx, incomingDeployment{
			RepoURL: p.Repository.HTMLURL, Provider: "github", Kind: "deployment",
			Name: shortSHA(p.Deployment.SHA, p.Deployment.Ref), URL: url, Environment: env, At: time.Now(),
		})
	}

	return Outcome{Handled: false, Note: "événement ignoré : " + event}, nil
}

// --- GitLab ---------------------------------------------------------------

type glProject struct {
	WebURL string `json:"web_url"`
}

type glEnvelope struct {
	ObjectKind string `json:"object_kind"`
}

type glMergeRequestEvent struct {
	User struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"user"`
	Project          glProject `json:"project"`
	ObjectAttributes struct {
		IID          int64  `json:"iid"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		URL          string `json:"url"`
		State        string `json:"state"`
		SourceBranch string `json:"source_branch"`
		CreatedAt    string `json:"created_at"`
		MergedAt     string `json:"merged_at"`
	} `json:"object_attributes"`
}

type glTagPushEvent struct {
	Ref     string    `json:"ref"`
	After   string    `json:"after"`
	Project glProject `json:"project"`
}

type glReleaseEvent struct {
	Action  string    `json:"action"`
	Name    string    `json:"name"`
	Tag     string    `json:"tag"`
	URL     string    `json:"url"`
	Project glProject `json:"project"`
}

type glDeploymentEvent struct {
	Status        string    `json:"status"`
	Environment   string    `json:"environment"`
	DeployableURL string    `json:"deployable_url"`
	ShortSHA      string    `json:"short_sha"`
	Project       glProject `json:"project"`
}

// HandleGitLab traite un evenement GitLab deja authentifie.
func (s *GitService) HandleGitLab(ctx context.Context, body []byte) (Outcome, error) {
	var env glEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Outcome{}, domain.ErrValidation.WithMessage("Charge GitLab illisible")
	}

	switch env.ObjectKind {
	case "merge_request":
		var p glMergeRequestEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return Outcome{}, domain.ErrValidation.WithMessage("Charge GitLab illisible")
		}
		state := "closed"
		switch p.ObjectAttributes.State {
		case "opened", "locked":
			state = "open"
		case "merged":
			state = "merged"
		}
		author := p.User.Username
		if author == "" {
			author = p.User.Name
		}
		return s.pullRequest(ctx, incomingPullRequest{
			RepoURL: p.Project.WebURL, Provider: "gitlab", Number: p.ObjectAttributes.IID,
			Title: p.ObjectAttributes.Title, URL: p.ObjectAttributes.URL, Branch: p.ObjectAttributes.SourceBranch,
			Author: author, State: state, Description: p.ObjectAttributes.Description,
			OpenedAt: gitlabTime(p.ObjectAttributes.CreatedAt), MergedAt: gitlabTime(p.ObjectAttributes.MergedAt),
		})

	case "tag_push":
		var p glTagPushEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return Outcome{}, domain.ErrValidation.WithMessage("Charge GitLab illisible")
		}
		tag, ok := strings.CutPrefix(p.Ref, "refs/tags/")
		if !ok || strings.Trim(p.After, "0") == "" {
			return Outcome{Handled: false, Note: "tag supprimé"}, nil
		}
		return s.deployment(ctx, incomingDeployment{
			RepoURL: p.Project.WebURL, Provider: "gitlab", Kind: "tag", Name: tag,
			URL: strings.TrimRight(p.Project.WebURL, "/") + "/-/tags/" + tag, At: time.Now(),
		})

	case "release":
		var p glReleaseEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return Outcome{}, domain.ErrValidation.WithMessage("Charge GitLab illisible")
		}
		if p.Action != "create" {
			return Outcome{Handled: false, Note: "release " + p.Action}, nil
		}
		name := p.Tag
		if name == "" {
			name = p.Name
		}
		return s.deployment(ctx, incomingDeployment{
			RepoURL: p.Project.WebURL, Provider: "gitlab", Kind: "release", Name: name, URL: p.URL, At: time.Now(),
		})

	case "deployment":
		var p glDeploymentEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return Outcome{}, domain.ErrValidation.WithMessage("Charge GitLab illisible")
		}
		if p.Status != "success" {
			return Outcome{Handled: false, Note: "déploiement " + p.Status}, nil
		}
		return s.deployment(ctx, incomingDeployment{
			RepoURL: p.Project.WebURL, Provider: "gitlab", Kind: "deployment",
			Name: shortSHA(p.ShortSHA, p.Environment), URL: p.DeployableURL, Environment: p.Environment, At: time.Now(),
		})
	}

	return Outcome{Handled: false, Note: "événement ignoré : " + env.ObjectKind}, nil
}

// --- Traitement commun ----------------------------------------------------

type incomingPullRequest struct {
	RepoURL, Provider          string
	Number                     int64
	Title, URL, Branch, Author string
	State, Description         string
	OpenedAt, MergedAt         *time.Time
}

type incomingDeployment struct {
	RepoURL, Provider, Kind, Name, URL, Environment string
	At                                              time.Time
}

func (s *GitService) project(ctx context.Context, repoURL string) (db.Project, bool, error) {
	normalized := NormalizeRepoURL(repoURL)
	if normalized == "" {
		return db.Project{}, false, nil
	}
	project, err := s.q.GetProjectByRepoURL(ctx, normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Project{}, false, nil
		}
		return db.Project{}, false, fmt.Errorf("recherche du projet : %w", err)
	}
	return project, true, nil
}

// pullRequest enregistre la pull request et fait avancer ce qu'elle nomme.
//
//   - ouverte : les tickets qu'elle cite passent en revue, les taches aussi ;
//   - fusionnee : les tickets passent « prêt à déployer », les taches sont
//     terminees. Le ticket ne se clot qu'a la mise en ligne, quand le client
//     peut constater le changement.
func (s *GitService) pullRequest(ctx context.Context, in incomingPullRequest) (Outcome, error) {
	project, ok, err := s.project(ctx, in.RepoURL)
	if err != nil {
		return Outcome{}, err
	}
	if !ok {
		return Outcome{Handled: false, Note: "aucun projet ne porte le dépôt " + in.RepoURL}, nil
	}

	pr, err := s.q.UpsertPullRequest(ctx, db.UpsertPullRequestParams{
		ProjectID: project.ID, Provider: in.Provider, Number: in.Number, Title: strings.TrimSpace(in.Title),
		Url: in.URL, Branch: in.Branch, Author: in.Author, State: in.State, OpenedAt: in.OpenedAt, MergedAt: in.MergedAt,
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("enregistrement de la pull request : %w", err)
	}

	refs := ParseGitRefs(in.Title, in.Branch, in.Description)
	label := fmt.Sprintf("Pull request #%d", in.Number)
	if in.Provider == "gitlab" {
		label = fmt.Sprintf("Merge request !%d", in.Number)
	}
	linked := 0

	for _, numero := range refs.Tickets {
		ticket, err := s.q.GetTicketByNumeroInProject(ctx, db.GetTicketByNumeroInProjectParams{ProjectID: project.ID, Numero: numero})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return Outcome{}, fmt.Errorf("recherche du ticket #%d : %w", numero, err)
		}
		if err := s.q.LinkPullRequestToTicket(ctx, db.LinkPullRequestToTicketParams{PullRequestID: pr.ID, TicketID: &ticket.ID}); err != nil {
			return Outcome{}, fmt.Errorf("rattachement au ticket : %w", err)
		}
		linked++

		var next string
		switch {
		case in.State == "open" && (ticket.Status == "backlog" || ticket.Status == "todo" || ticket.Status == "in_progress"):
			next = "in_review"
		case in.State == "merged" && ticket.Status != "ready_to_deploy" && ticket.Status != "done" && ticket.Status != "annule":
			next = "ready_to_deploy"
		}
		if next == "" {
			continue
		}
		verb := "ouverte"
		if in.State == "merged" {
			verb = "fusionnée"
		}
		body := fmt.Sprintf("%s %s : %s\n%s", label, verb, strings.TrimSpace(in.Title), in.URL)
		if _, err := s.tickets.PostMessage(ctx, ticket.ID, PostMessageInput{Body: body, IsInternal: true, NewStatus: &next}); err != nil {
			return Outcome{}, fmt.Errorf("avancement du ticket #%d : %w", numero, err)
		}
	}

	for _, numero := range refs.Tasks {
		task, err := s.q.GetTaskByNumeroInProject(ctx, db.GetTaskByNumeroInProjectParams{ProjectID: project.ID, Numero: numero})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return Outcome{}, fmt.Errorf("recherche de la tâche T-%d : %w", numero, err)
		}
		if err := s.q.LinkPullRequestToTask(ctx, db.LinkPullRequestToTaskParams{PullRequestID: pr.ID, TaskID: &task.ID}); err != nil {
			return Outcome{}, fmt.Errorf("rattachement à la tâche : %w", err)
		}
		linked++

		var next string
		switch {
		case in.State == "open" && (task.Status == "todo" || task.Status == "progress"):
			next = "review"
		case in.State == "merged" && task.Status != "done":
			next = "done"
		}
		if next == "" {
			continue
		}
		if err := s.moveTask(ctx, task.ID, task.Status, next, label); err != nil {
			return Outcome{}, err
		}
	}

	s.log.Info("pull request reçue", "project", project.ID, "provider", in.Provider, "number", in.Number, "state", in.State, "links", linked)

	return Outcome{Handled: true, Note: fmt.Sprintf("%s %s, %d rattachement(s)", label, in.State, linked)}, nil
}

// moveTask change le statut d'une tache au nom de l'integration : pas
// d'acteur au journal, mais la pull request qui l'a decide.
func (s *GitService) moveTask(ctx context.Context, taskID uuid.UUID, from, to, cause string) error {
	if _, err := s.q.MoveTask(ctx, db.MoveTaskParams{ID: taskID, Status: to}); err != nil {
		return fmt.Errorf("avancement de la tâche : %w", err)
	}
	payload, _ := json.Marshal(map[string]any{"from": from, "to": to, "cause": cause})
	if err := s.q.LogTaskActivity(ctx, db.LogTaskActivityParams{TaskID: taskID, Kind: "status_changed", Payload: payload}); err != nil {
		return fmt.Errorf("journal de la tâche : %w", err)
	}
	return nil
}

// deployment enregistre une mise en ligne et en tire les consequences :
// journal du client, jalon de mise en ligne, tickets prets a deployer clos.
func (s *GitService) deployment(ctx context.Context, in incomingDeployment) (Outcome, error) {
	project, ok, err := s.project(ctx, in.RepoURL)
	if err != nil {
		return Outcome{}, err
	}
	if !ok {
		return Outcome{Handled: false, Note: "aucun projet ne porte le dépôt " + in.RepoURL}, nil
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Outcome{Handled: false, Note: "mise en ligne sans nom"}, nil
	}

	if _, err := s.q.InsertDeployment(ctx, db.InsertDeploymentParams{
		ProjectID: project.ID, Provider: in.Provider, Kind: in.Kind, Name: name, Url: in.URL, Environment: in.Environment, DeployedAt: in.At,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Outcome{Handled: true, Note: "mise en ligne déjà connue"}, nil
		}
		return Outcome{}, fmt.Errorf("enregistrement de la mise en ligne : %w", err)
	}

	label := "Mise en ligne : " + name
	if in.Environment != "" {
		label += " (" + in.Environment + ")"
	}

	// Journal du client : une mise en ligne est un evenement de la relation,
	// pas seulement de la production. Un projet interne n'a pas de client.
	if !project.IsInternal {
		if _, err := s.q.CreateInteraction(ctx, db.CreateInteractionParams{
			ClientID: project.ClientID, Kind: InteractionDeployment, Body: label, OccurredAt: in.At, ProjectID: &project.ID,
		}); err != nil {
			return Outcome{}, fmt.Errorf("journal du client : %w", err)
		}
	}

	// Le jalon de mise en ligne, s'il existe et n'est pas atteint.
	milestones, err := s.q.ListOpenMilestonesOfProject(ctx, project.ID)
	if err != nil {
		return Outcome{}, fmt.Errorf("jalons du projet : %w", err)
	}
	reached := 0
	for _, m := range milestones {
		if !deployMilestonePattern.MatchString(m.Title) {
			continue
		}
		done := true
		if _, err := s.milestones.Update(ctx, m.ID, MilestoneInput{SetDone: true, Done: done}); err != nil {
			return Outcome{}, fmt.Errorf("atteinte du jalon : %w", err)
		}
		reached++
		break
	}

	// Les tickets dont la pull request etait fusionnee : le client peut
	// maintenant constater le changement, on le lui dit.
	waiting, err := s.q.ListTicketsAwaitingDeploy(ctx, project.ID)
	if err != nil {
		return Outcome{}, fmt.Errorf("tickets prêts à déployer : %w", err)
	}
	done := "done"
	for _, t := range waiting {
		body := fmt.Sprintf("Mis en ligne avec %s.", name)
		if _, err := s.tickets.PostMessage(ctx, t.ID, PostMessageInput{Body: body, IsInternal: false, NewStatus: &done}); err != nil {
			return Outcome{}, fmt.Errorf("clôture du ticket #%d : %w", t.Numero, err)
		}
	}

	s.log.Info("mise en ligne reçue", "project", project.ID, "provider", in.Provider, "kind", in.Kind, "name", name, "tickets", len(waiting), "milestones", reached)

	return Outcome{Handled: true, Note: fmt.Sprintf("%s · %d ticket(s) clos, %d jalon(s) atteint(s)", label, len(waiting), reached)}, nil
}

func timeOr(t *time.Time) time.Time {
	if t == nil {
		return time.Now()
	}
	return *t
}

// gitlabTime lit les dates de GitLab, qui en a plusieurs ecritures.
func gitlabTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05 -0700"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	return nil
}

func shortSHA(sha, fallback string) string {
	if len(sha) >= 7 {
		return sha[:7]
	}
	if sha != "" {
		return sha
	}
	return fallback
}
