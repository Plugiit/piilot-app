// Package updates sait quelle est la derniere version publiee de Piilot, en
// interrogeant les releases GitHub, et compare deux numeros de version.
//
// Il n'est appele que depuis une tache de fond : jamais pendant une requete
// HTTP, ou un ecran attendrait un serveur qu'on ne maitrise pas.
package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Release est une version publiee sur GitHub.
type Release struct {
	Version     string
	Name        string
	URL         string
	PublishedAt *time.Time
}

// Client porte les appels sortants, avec un delai de garde court : une tache
// de fond qui attend un service tiers ne doit pas s'accumuler.
type Client struct {
	http *http.Client
	// Adresses de base, remplacables en test.
	githubAPI string
}

// New construit un client.
func New() *Client {
	return &Client{
		http:      &http.Client{Timeout: 20 * time.Second},
		githubAPI: "https://api.github.com",
	}
}

// LatestRelease rend la derniere version stable du depot `owner/name`.
//
// L'endpoint « latest » de GitHub ignore les pre-versions (1.0.0-rc.1) et les
// brouillons : une instance de production ne doit pas se voir proposer une
// version que l'equipe n'a pas declaree stable.
func (c *Client) LatestRelease(ctx context.Context, repository string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.githubAPI+"/repos/"+repository+"/releases/latest", nil)
	if err != nil {
		return Release{}, fmt.Errorf("requete GitHub : %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "piilot-update-check")

	res, err := c.http.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("appel GitHub : %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub a repondu %d", res.StatusCode)
	}

	var body struct {
		TagName     string     `json:"tag_name"`
		Name        string     `json:"name"`
		HTMLURL     string     `json:"html_url"`
		PublishedAt *time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("reponse GitHub illisible : %w", err)
	}

	version := strings.TrimPrefix(body.TagName, "v")
	if _, ok := parse(version); !ok {
		return Release{}, fmt.Errorf("tag GitHub inattendu : %q", body.TagName)
	}

	return Release{Version: version, Name: body.Name, URL: body.HTMLURL, PublishedAt: body.PublishedAt}, nil
}

// Newer dit si `candidate` est une version strictement posterieure a `current`.
//
// Une version illisible n'est jamais plus recente : une instance construite
// hors release (« dev ») ou un tag inattendu ne doivent pas declencher de
// proposition de mise a jour.
func Newer(candidate, current string) bool {
	c, ok := parse(candidate)
	if !ok {
		return false
	}
	v, ok := parse(current)
	if !ok {
		return false
	}

	for i := range 3 {
		if c.core[i] != v.core[i] {
			return c.core[i] > v.core[i]
		}
	}

	// A numeros egaux, la version stable passe devant sa pre-version :
	// 1.0.0 est plus recente que 1.0.0-rc.2.
	switch {
	case c.pre == v.pre:
		return false
	case c.pre == "":
		return true
	case v.pre == "":
		return false
	default:
		return c.pre > v.pre
	}
}

type semver struct {
	core [3]int
	pre  string
}

// parse lit « X.Y.Z » ou « X.Y.Z-pre ».
func parse(version string) (semver, bool) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	core, pre, _ := strings.Cut(version, "-")

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}

	var out semver
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return semver{}, false
		}
		out.core[i] = n
	}
	out.pre = pre

	return out, true
}
