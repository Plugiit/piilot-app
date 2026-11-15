package gitsource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBase = "https://api.github.com"
	timeout     = 20 * time.Second
	maxBody     = 8 << 20
	pageSize    = 100
	// maxPages borne la lecture : 2 000 pull requests fermees par depot sur la
	// periode, largement plus qu'une agence n'en fusionne en trois mois.
	maxPages = 20
)

// GitHub lit l'API REST de GitHub.
type GitHub struct {
	// Token est un jeton d'acces, avec la permission « Pull requests : lecture ».
	Token string
	// BaseURL vaut https://api.github.com ; les tests y mettent leur serveur.
	BaseURL string
	Client  *http.Client
}

var _ Provider = (*GitHub)(nil)

// RepoOf rend le proprietaire et le nom d'un depot GitHub, depuis son adresse
// normalisee.
func RepoOf(repoURL string) (owner, repo string, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(repoURL), "https://github.com/")
	if !found {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

type pullJSON struct {
	Number    int64   `json:"number"`
	Title     string  `json:"title"`
	HTMLURL   string  `json:"html_url"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	MergedAt  *string `json:"merged_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// MergedPullRequests lit GET /repos/{owner}/{repo}/pulls?state=closed, trie
// par mise a jour decroissante (c'est le tri que GitHub documente), page par
// page par l'en-tete Link, et ne garde que ce qui a ete fusionne dans la
// periode.
//
// Une pull request fusionnee a ete mise a jour au plus tot a sa fusion : la
// lecture s'arrete a la premiere page dont toutes les entrees sont plus
// anciennes que since. Elle ne depend pas de l'ordre a l'interieur d'une page.
// Au-dela de maxPages, elle echoue plutot que de rendre une liste incomplete.
func (g *GitHub) MergedPullRequests(ctx context.Context, repoURL string, since, until time.Time) ([]PullRequest, error) {
	owner, repo, ok := RepoOf(repoURL)
	if !ok {
		return nil, ErrUnsupportedRepo
	}
	base := strings.TrimRight(g.BaseURL, "/")
	if base == "" {
		base = defaultBase
	}
	baseURL, err := url.Parse(base)
	if err != nil || baseURL.Host == "" {
		return nil, fmt.Errorf("adresse de l’API GitHub : %q", g.BaseURL)
	}

	query := url.Values{"state": {"closed"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {fmt.Sprint(pageSize)}}
	next := fmt.Sprintf("%s/repos/%s/%s/pulls?%s", base, url.PathEscape(owner), url.PathEscape(repo), query.Encode())

	var out []PullRequest
	for page := 0; page < maxPages && next != ""; page++ {
		pulls, link, err := g.page(ctx, next)
		if err != nil {
			return nil, err
		}
		allOlder := len(pulls) > 0
		for _, p := range pulls {
			if updated, ok := parseTime(p.UpdatedAt); !ok || !updated.Before(since) {
				allOlder = false
			}
			if p.MergedAt == nil {
				continue
			}
			merged, ok := parseTime(*p.MergedAt)
			if !ok || merged.Before(since) || !merged.Before(until) {
				continue
			}
			pr := PullRequest{Number: p.Number, Title: p.Title, URL: p.HTMLURL, Branch: p.Head.Ref, Author: p.User.Login, MergedAt: merged}
			if opened, ok := parseTime(p.CreatedAt); ok {
				pr.OpenedAt = &opened
			}
			for _, l := range p.Labels {
				pr.Labels = append(pr.Labels, l.Name)
			}
			out = append(out, pr)
		}
		if allOlder || link == "" {
			return out, nil
		}
		// Le lien suivant ne doit mener qu'a GitHub : le jeton le suit.
		u, err := url.Parse(link)
		if err != nil || u.Scheme != baseURL.Scheme || u.Host != baseURL.Host {
			return nil, fmt.Errorf("lien de pagination inattendu : %q", link)
		}
		next = link
	}
	return nil, ErrTooMany
}

func (g *GitHub) page(ctx context.Context, target string) ([]pullJSON, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", fmt.Errorf("adresse de GitHub : %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := g.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("appel de GitHub : %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return nil, "", fmt.Errorf("lecture de la réponse de GitHub : %w", err)
	}

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, "", ErrUnauthorized
	case http.StatusForbidden, http.StatusTooManyRequests:
		if res.Header.Get("X-RateLimit-Remaining") == "0" || res.StatusCode == http.StatusTooManyRequests {
			return nil, "", fmt.Errorf("limite de débit de GitHub atteinte : réessayez plus tard")
		}
		return nil, "", fmt.Errorf("%w (%s)", ErrForbidden, messageOf(raw))
	case http.StatusNotFound:
		return nil, "", ErrRepoNotFound
	default:
		return nil, "", fmt.Errorf("GitHub répond %d", res.StatusCode)
	}

	var pulls []pullJSON
	if err := json.Unmarshal(raw, &pulls); err != nil {
		return nil, "", fmt.Errorf("réponse inattendue de GitHub : %w", err)
	}
	return pulls, nextLink(res.Header.Get("Link")), nil
}

// nextLink lit l'adresse rel="next" d'un en-tete Link.
func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		target, params, ok := strings.Cut(part, ";")
		if !ok || !strings.Contains(params, `rel="next"`) {
			continue
		}
		return strings.Trim(strings.TrimSpace(target), "<>")
	}
	return ""
}

func messageOf(raw []byte) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &e) != nil || e.Message == "" {
		return "sans détail"
	}
	if len(e.Message) > 200 {
		return e.Message[:200]
	}
	return e.Message
}

func parseTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}
