package usecase

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Comment une pull request dit ce qu'elle fait avancer : par le numero du
// ticket ou de la tache, dans son titre, sa branche ou sa description. Pas de
// rattachement a la main : c'est ce que les developpeurs ecrivent deja.
//
//   - ticket : « #47 », « ticket-47 », « ticket/47 », « ticket_47 »
//   - tache  : « T-123 », « T123 », « task-123 », « tache-123 »
//
// Le numero seul ne suffit pas : il est cherche dans le projet du depot, et
// un « #47 » qui designe autre chose — une issue GitHub, un ticket d'un autre
// projet — ne rattache rien.
var (
	ticketRefPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:ticket[-_/ ]?#?|#)(\d{1,9})(?:[^0-9]|$)`)
	taskWordPattern  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:task|tache|tâche|t)[-_/](\d{1,9})(?:[^0-9]|$)`)
	taskShortPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9])T(\d{1,9})(?:[^0-9]|$)`)
)

// GitRefs sont les numeros trouves dans un texte.
type GitRefs struct {
	Tickets []int64
	Tasks   []int64
}

// ParseGitRefs lit les numeros de tickets et de taches d'un ou plusieurs
// textes — titre, branche, description. Chaque numero n'est rendu qu'une fois.
func ParseGitRefs(texts ...string) GitRefs {
	var refs GitRefs
	seenTickets := map[int64]bool{}
	seenTasks := map[int64]bool{}

	for _, text := range texts {
		for _, m := range ticketRefPattern.FindAllStringSubmatch(text, -1) {
			if n, err := strconv.ParseInt(m[1], 10, 64); err == nil && !seenTickets[n] {
				seenTickets[n] = true
				refs.Tickets = append(refs.Tickets, n)
			}
		}
		for _, pattern := range []*regexp.Regexp{taskWordPattern, taskShortPattern} {
			for _, m := range pattern.FindAllStringSubmatch(text, -1) {
				if n, err := strconv.ParseInt(m[1], 10, 64); err == nil && !seenTasks[n] {
					seenTasks[n] = true
					refs.Tasks = append(refs.Tasks, n)
				}
			}
		}
	}

	return refs
}

// NormalizeRepoURL ramene l'adresse d'un depot a une forme unique :
// « https://hote/groupe/depot », en minuscules, sans « .git » ni barre
// finale. C'est elle qui est stockee sur le projet et comparee a celle que
// GitHub ou GitLab envoient. Vide si l'adresse n'est pas lisible.
//
// « git@github.com:org/repo.git » est accepte : c'est ce qu'on copie le plus
// souvent.
func NormalizeRepoURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "git@") {
		rest := strings.TrimPrefix(raw, "git@")
		host, path, ok := strings.Cut(rest, ":")
		if !ok {
			return ""
		}
		raw = "https://" + host + "/" + path
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}

	path := strings.TrimSuffix(strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git"), "/")
	if path == "" || !strings.Contains(path, "/") {
		return ""
	}

	return "https://" + strings.ToLower(u.Host) + "/" + strings.ToLower(path)
}

// GitProviderOf devine le fournisseur a l'hote du depot : GitHub chez
// github.com, GitLab partout ailleurs — une instance GitLab auto-hebergee a
// n'importe quel nom.
func GitProviderOf(repoURL string) string {
	if strings.Contains(repoURL, "github.com/") {
		return "github"
	}
	return "gitlab"
}

// deployMilestonePattern reconnait le jalon qu'une mise en ligne atteint.
var deployMilestonePattern = regexp.MustCompile(`(?i)mise en ligne|mise en prod|d[ée]ploiement|release|go ?live|lancement|livraison`)
