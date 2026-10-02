// Package docker parle a l'API du moteur Docker, par son socket Unix.
//
// Le strict necessaire a l'updater : lire un conteneur et son image, tirer une
// image, et recreer un conteneur a l'identique. Ecrit en HTTP brut plutot
// qu'avec le SDK officiel, qui tirerait des dizaines de dependances pour une
// dizaine d'appels.
package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrNotFound est rendu quand le conteneur ou l'image n'existe pas.
var ErrNotFound = errors.New("introuvable")

// Client appelle le moteur Docker.
//
// La version d'API n'est pas figee : elle est demandee au moteur au premier
// appel. Une version ecrite en dur casse dans les deux sens — trop recente
// pour un vieux serveur, trop ancienne pour un moteur recent, qui refuse les
// versions sous un plancher.
type Client struct {
	http    *http.Client
	base    string
	version string
	mu      sync.Mutex
}

// NewUnix construit un client sur le socket Unix `path`.
func NewUnix(path string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		},
	}

	// Pas de delai global : tirer une image peut prendre plusieurs minutes.
	// Chaque appel porte le contexte de l'appelant, qui borne la duree.
	return &Client{http: &http.Client{Transport: transport}, base: "http://docker"}
}

// NewHTTP construit un client sur une adresse HTTP. Sert aux tests.
func NewHTTP(base string) *Client {
	return &Client{http: http.DefaultClient, base: strings.TrimRight(base, "/")}
}

// Container est ce que l'inspection d'un conteneur rend. Config et HostConfig
// restent bruts : l'updater les recopie tels quels dans le conteneur recree,
// sans avoir a connaitre chacun de leurs champs.
type Container struct {
	ID              string          `json:"Id"`
	Name            string          `json:"Name"`
	Image           string          `json:"Image"`
	Config          json.RawMessage `json:"Config"`
	HostConfig      json.RawMessage `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]Endpoint `json:"Networks"`
	} `json:"NetworkSettings"`
	State struct {
		Status  string `json:"Status"`
		Running bool   `json:"Running"`
		Health  *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
}

// ConfigImage rend la reference d'image declaree dans la configuration du
// conteneur (« ghcr.io/plugiit/piilot-app:latest »), et non l'identifiant de
// l'image qu'il fait tourner.
func (c Container) ConfigImage() string {
	var cfg struct {
		Image string `json:"Image"`
	}
	_ = json.Unmarshal(c.Config, &cfg)

	return cfg.Image
}

// Labels rend les etiquettes du conteneur.
func (c Container) Labels() map[string]string {
	var cfg struct {
		Labels map[string]string `json:"Labels"`
	}
	_ = json.Unmarshal(c.Config, &cfg)

	return cfg.Labels
}

// Endpoint est le branchement d'un conteneur sur un reseau.
type Endpoint struct {
	Aliases    []string        `json:"Aliases,omitempty"`
	IPAMConfig json.RawMessage `json:"IPAMConfig,omitempty"`
	DriverOpts json.RawMessage `json:"DriverOpts,omitempty"`
	Links      []string        `json:"Links,omitempty"`
}

// Image est ce que l'inspection d'une image rend.
type Image struct {
	ID     string          `json:"Id"`
	Config json.RawMessage `json:"Config"`
}

// InspectContainer lit un conteneur, par identifiant ou par nom.
func (c *Client) InspectContainer(ctx context.Context, id string) (Container, error) {
	var out Container
	err := c.do(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil, nil, &out)

	return out, err
}

// FindContainers rend les identifiants des conteneurs, arretes compris, qui
// portent toutes les etiquettes `labels` (« cle=valeur »).
func (c *Client) FindContainers(ctx context.Context, labels ...string) ([]string, error) {
	filters, _ := json.Marshal(map[string][]string{"label": labels})
	query := url.Values{"all": {"true"}, "filters": {string(filters)}}

	var out []struct {
		ID string `json:"Id"`
	}
	if err := c.do(ctx, http.MethodGet, "/containers/json", query, nil, &out); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(out))
	for _, item := range out {
		ids = append(ids, item.ID)
	}

	return ids, nil
}

// InspectImage lit une image, par identifiant ou par reference.
func (c *Client) InspectImage(ctx context.Context, ref string) (Image, error) {
	var out Image
	err := c.do(ctx, http.MethodGet, "/images/"+ref+"/json", nil, nil, &out)

	return out, err
}

// Pull tire `repository:tag` depuis son registre.
//
// Le moteur repond en flux, une ligne JSON par etape, et signale une erreur
// dans le flux plutot que par le statut HTTP : il faut lire jusqu'au bout pour
// savoir si le tirage a abouti.
func (c *Client) Pull(ctx context.Context, repository, tag string) error {
	query := url.Values{"fromImage": {repository}, "tag": {tag}}

	res, err := c.request(ctx, http.MethodPost, "/images/create", query, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		var line struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) == nil && line.Error != "" {
			return fmt.Errorf("tirage de %s:%s : %s", repository, tag, line.Error)
		}
	}

	return scanner.Err()
}

// StopContainer arrete un conteneur, en lui laissant `grace` pour finir.
func (c *Client) StopContainer(ctx context.Context, id string, grace time.Duration) error {
	query := url.Values{"t": {fmt.Sprint(int(grace.Seconds()))}}
	err := c.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/stop", query, nil, nil)

	// Deja arrete : le moteur repond 304, ce n'est pas une erreur.
	if errors.Is(err, errNotModified) {
		return nil
	}

	return err
}

// StartContainer demarre un conteneur.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil, nil, nil)
	if errors.Is(err, errNotModified) {
		return nil
	}

	return err
}

// RenameContainer renomme un conteneur.
func (c *Client) RenameContainer(ctx context.Context, id, name string) error {
	return c.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/rename",
		url.Values{"name": {name}}, nil, nil)
}

// RemoveContainer supprime un conteneur, arrete ou non, sans ses volumes
// nommes.
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id),
		url.Values{"force": {"true"}}, nil, nil)
}

// CreateContainer cree un conteneur nomme `name` et rend son identifiant.
// `body` est la configuration complete, au format de l'API.
func (c *Client) CreateContainer(ctx context.Context, name string, body any) (string, error) {
	var out struct {
		ID string `json:"Id"`
	}
	err := c.do(ctx, http.MethodPost, "/containers/create", url.Values{"name": {name}}, body, &out)

	return out.ID, err
}

// ConnectNetwork branche un conteneur sur un reseau supplementaire.
func (c *Client) ConnectNetwork(ctx context.Context, network, container string, endpoint Endpoint) error {
	body := map[string]any{"Container": container, "EndpointConfig": endpoint}

	return c.do(ctx, http.MethodPost, "/networks/"+url.PathEscape(network)+"/connect", nil, body, nil)
}

var errNotModified = errors.New("non modifie")

// apiVersion rend la version d'API du moteur, demandee une fois. /version est
// le seul endpoint qui s'appelle sans prefixe de version.
func (c *Client) apiVersion(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.version != "" {
		return c.version, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/version", nil)
	if err != nil {
		return "", fmt.Errorf("requete Docker : %w", err)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("socket Docker injoignable : %w", err)
	}
	defer res.Body.Close()

	var out struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || out.APIVersion == "" {
		return "", fmt.Errorf("version d'API Docker illisible (statut %d)", res.StatusCode)
	}

	c.version = out.APIVersion

	return c.version, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	res, err := c.request(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if out == nil {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("reponse de Docker illisible (%s %s) : %w", method, path, err)
	}

	return nil
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("corps de requete Docker : %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	version, err := c.apiVersion(ctx)
	if err != nil {
		return nil, err
	}

	target := c.base + "/v" + version + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("requete Docker : %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("socket Docker injoignable : %w", err)
	}

	switch {
	case res.StatusCode == http.StatusNotModified:
		res.Body.Close()
		return nil, errNotModified
	case res.StatusCode == http.StatusNotFound:
		res.Body.Close()
		return nil, fmt.Errorf("%s %s : %w", method, path, ErrNotFound)
	case res.StatusCode >= 300:
		defer res.Body.Close()
		var msg struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 64*1024)).Decode(&msg)
		return nil, fmt.Errorf("Docker a repondu %d a %s %s : %s", res.StatusCode, method, path, msg.Message)
	}

	return res, nil
}
