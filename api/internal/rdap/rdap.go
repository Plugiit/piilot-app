// Package rdap lit la date d'expiration d'un domaine par RDAP, le successeur
// structure de WHOIS. rdap.org redirige vers le registre de l'extension.
package rdap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBase = "https://rdap.org"
	timeout     = 15 * time.Second
	maxBody     = 1 << 20
	maxRedirect = 5
)

// ErrNoExpiry : le registre repond, mais ne publie pas de date d'expiration.
// Certaines extensions nationales ne le font pas.
var ErrNoExpiry = errors.New("le registre ne publie pas de date d'expiration")

// Client interroge RDAP.
type Client struct {
	base string
	http *http.Client
}

// New construit un client. Une redirection ne quitte jamais HTTPS.
func New() *Client {
	return &Client{
		base: defaultBase,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirect {
					return errors.New("trop de redirections")
				}
				if req.URL.Scheme != "https" {
					return errors.New("redirection hors HTTPS")
				}
				return nil
			},
		},
	}
}

type response struct {
	Events []struct {
		Action string `json:"eventAction"`
		Date   string `json:"eventDate"`
	} `json:"events"`
}

// Expiry rend la date d'expiration d'un domaine.
func (c *Client) Expiry(ctx context.Context, domain string) (time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/domain/"+url.PathEscape(domain), nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("requete RDAP : %w", err)
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("appel RDAP : %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("RDAP a repondu %d", res.StatusCode)
	}

	var body response
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(&body); err != nil {
		return time.Time{}, fmt.Errorf("reponse RDAP illisible : %w", err)
	}
	for _, e := range body.Events {
		if !strings.EqualFold(e.Action, "expiration") {
			continue
		}
		at, err := time.Parse(time.RFC3339, e.Date)
		if err != nil {
			return time.Time{}, fmt.Errorf("date RDAP illisible : %w", err)
		}
		return at, nil
	}
	return time.Time{}, ErrNoExpiry
}
