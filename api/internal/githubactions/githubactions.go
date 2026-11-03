// Package githubactions demande a GitHub de lancer un workflow d'un depot.
// Il ne fait rien d'autre : le workflow s'execute chez GitHub et rend compte
// lui-meme de son resultat.
package githubactions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBase = "https://api.github.com"
	timeout     = 15 * time.Second
	maxBody     = 1 << 16
)

// Dispatch est une demande de lancement.
type Dispatch struct {
	Token, Owner, Repo, Workflow, Ref string
	Inputs                            map[string]string
}

// Error est un refus de GitHub. Le message vient de GitHub, jamais du jeton.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("GitHub a repondu %d : %s", e.Status, e.Message)
}

// Retryable : GitHub limite le debit ou se dit indisponible, il n'a donc rien
// lance. Un 500, 502 ou 504 ne dit rien de tel : la demande a pu etre acceptee
// derriere, la retenter risquerait de lancer le workflow deux fois.
func (e *Error) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status == http.StatusServiceUnavailable
}

// Refused : GitHub a examine la demande et l'a refusee. Rien ne tourne.
func (e *Error) Refused() bool {
	return e.Status < http.StatusInternalServerError
}

// Client appelle l'API de GitHub.
type Client struct {
	base string
	http *http.Client
}

// New construit un client.
func New() *Client {
	return &Client{base: defaultBase, http: &http.Client{Timeout: timeout}}
}

// Dispatch lance le workflow. Une reponse 204 veut dire que GitHub l'a accepte,
// pas qu'il a reussi.
func (c *Client) Dispatch(ctx context.Context, d Dispatch) error {
	body, err := json.Marshal(map[string]any{"ref": d.Ref, "inputs": d.Inputs})
	if err != nil {
		return fmt.Errorf("demande de lancement : %w", err)
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/actions/workflows/%s/dispatches", c.base, url.PathEscape(d.Owner), url.PathEscape(d.Repo), url.PathEscape(d.Workflow))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("demande de lancement : %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+d.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("appel de GitHub : %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNoContent {
		return nil
	}

	raw, _ := io.ReadAll(io.LimitReader(res.Body, maxBody))
	var parsed struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if parsed.Message == "" {
		parsed.Message = http.StatusText(res.StatusCode)
	}
	return &Error{Status: res.StatusCode, Message: parsed.Message}
}
