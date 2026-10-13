package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Umami lit une instance Umami, auto-hebergee ou Umami Cloud.
//
// Le format des reponses a change entre versions majeures : /stats rend des
// objets {value, prev} en 2.x, des chiffres a plat et un objet comparison en
// 3.x ; le classement des pages s'appelle « url » en 2.x et « path » en 3.x.
// La version se deduit de la forme de /stats. Une reponse qui ne correspond
// plus a la version retenue — une instance mise a jour — relance la
// detection au lieu d'echouer, et OnVersion le signale.
type Umami struct {
	// BaseURL : la racine de l'instance (https://stats.agence.fr), ou l'API
	// d'Umami Cloud (https://api.umami.is/v1).
	BaseURL string
	// Username et Password : instance auto-hebergee, jeton par
	// /api/auth/login.
	Username string
	Password string
	// APIKey : Umami Cloud, en-tete x-umami-api-key.
	APIKey string
	// Version : 2 ou 3 ; 0 pour la deduire.
	Version int
	// Pinned : la version a ete choisie a la main, la detection ne la
	// change pas.
	Pinned bool
	// OnVersion est appele quand la detection change la version.
	OnVersion func(previous, detected int)
	// OnToken est appele a chaque nouveau jeton, pour le garder entre deux
	// demarrages.
	OnToken func(token string)
	Client  *http.Client

	mu    sync.Mutex
	token string
}

var _ Provider = (*Umami)(nil)

// maxBody borne une reponse : un classement de cent lignes tient dans
// quelques Ko.
const maxBody = 8 << 20

// SetToken reprend un jeton garde d'un demarrage precedent.
func (u *Umami) SetToken(token string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.token = token
}

// CurrentVersion rend la version retenue, 0 si elle n'est pas encore connue.
func (u *Umami) CurrentVersion() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.Version
}

func (u *Umami) setVersion(v int) {
	u.mu.Lock()
	if u.Pinned || u.Version == v {
		u.mu.Unlock()
		return
	}
	previous := u.Version
	u.Version = v
	u.mu.Unlock()
	if u.OnVersion != nil {
		u.OnVersion(previous, v)
	}
}

// endpoint : /api/... pour une instance, /... sous l'API de Cloud (/v1).
func (u *Umami) endpoint(path string, query url.Values) string {
	base := strings.TrimRight(strings.TrimSpace(u.BaseURL), "/")
	if !strings.HasSuffix(base, "/v1") {
		base += "/api"
	}
	if len(query) > 0 {
		return base + path + "?" + query.Encode()
	}
	return base + path
}

func (u *Umami) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return &http.Client{Timeout: 20 * time.Second}
}

// login echange identifiant et mot de passe contre un jeton.
func (u *Umami) login(ctx context.Context) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": u.Username, "password": u.Password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.endpoint("/auth/login", nil), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("adresse d'Umami : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := u.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("appel d'Umami : %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return "", ErrUnauthorized
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Umami répond %d à la connexion", res.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(&out); err != nil || out.Token == "" {
		return "", errors.New("réponse de connexion illisible : l’adresse ne pointe pas vers Umami")
	}
	u.SetToken(out.Token)
	if u.OnToken != nil {
		u.OnToken(out.Token)
	}
	return out.Token, nil
}

// statusError porte un code HTTP inattendu, pour qui veut le distinguer.
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("Umami répond %d", e.code) }

// get appelle l'API et decode la reponse. Avec un jeton, un 401 relance une
// connexion et rejoue la requete, une fois.
func (u *Umami) get(ctx context.Context, path string, query url.Values, out any) error {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.endpoint(path, query), nil)
		if err != nil {
			return fmt.Errorf("adresse d'Umami : %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if u.APIKey != "" {
			req.Header.Set("x-umami-api-key", u.APIKey)
		} else {
			u.mu.Lock()
			token := u.token
			u.mu.Unlock()
			if token == "" {
				if token, err = u.login(ctx); err != nil {
					return err
				}
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}

		res, err := u.client().Do(req)
		if err != nil {
			return fmt.Errorf("appel d'Umami : %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(res.Body, maxBody))
		_ = res.Body.Close()

		switch {
		case res.StatusCode == http.StatusUnauthorized && u.APIKey == "" && attempt == 0:
			// Jeton expire ou revoque : une nouvelle connexion, puis on rejoue.
			u.SetToken("")
			continue
		case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
			return ErrUnauthorized
		case res.StatusCode == http.StatusNotFound:
			return ErrNotFound
		case res.StatusCode != http.StatusOK:
			return statusError{res.StatusCode}
		}
		if readErr != nil {
			return fmt.Errorf("lecture de la réponse : %w", readErr)
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("réponse illisible de %s : %w", path, err)
		}
		return nil
	}
}

func rangeQuery(r Range) url.Values {
	q := url.Values{}
	q.Set("startAt", strconv.FormatInt(r.Start.UnixMilli(), 10))
	q.Set("endAt", strconv.FormatInt(r.End.UnixMilli(), 10))
	return q
}

// ListWebsites rend tous les sites, page par page.
func (u *Umami) ListWebsites(ctx context.Context) ([]Website, error) {
	var out []Website
	for page := 1; page <= 50; page++ {
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("pageSize", "100")
		var raw json.RawMessage
		if err := u.get(ctx, "/websites", q, &raw); err != nil {
			return nil, err
		}
		items, more, err := parseWebsites(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if !more {
			break
		}
	}
	return out, nil
}

// parseWebsites lit {data, count, page, pageSize} (2.x et 3.x) ou un tableau
// nu (anciennes versions).
func parseWebsites(raw json.RawMessage) ([]Website, bool, error) {
	type site struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Domain string `json:"domain"`
	}
	var list []site
	more := false
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, false, fmt.Errorf("liste des sites illisible : %w", err)
		}
	} else {
		var page struct {
			Data     []site `json:"data"`
			Count    int    `json:"count"`
			Page     int    `json:"page"`
			PageSize int    `json:"pageSize"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, false, fmt.Errorf("liste des sites illisible : %w", err)
		}
		list = page.Data
		more = page.PageSize > 0 && page.Page*page.PageSize < page.Count && len(page.Data) > 0
	}
	out := make([]Website, 0, len(list))
	for _, s := range list {
		if s.ID != "" {
			out = append(out, Website{ID: s.ID, Name: s.Name, Domain: s.Domain})
		}
	}
	return out, more, nil
}

// Stats rend les chiffres de la periode et de la precedente.
func (u *Umami) Stats(ctx context.Context, websiteID string, r Range) (Comparison, error) {
	var raw map[string]json.RawMessage
	if err := u.get(ctx, "/websites/"+url.PathEscape(websiteID)+"/stats", rangeQuery(r), &raw); err != nil {
		return Comparison{}, err
	}
	out, version, err := ParseStats(raw)
	if err != nil {
		return Comparison{}, err
	}
	// La forme de la reponse dit la version : une instance mise a jour se
	// reconnait ici, sans rien casser.
	u.setVersion(version)
	return out, nil
}

// ParseStats lit /stats dans les deux formats et dit lequel c'etait.
func ParseStats(raw map[string]json.RawMessage) (Comparison, int, error) {
	fields := []string{"visitors", "visits", "pageviews", "bounces", "totaltime"}
	var out Comparison
	cur := []*int64{&out.Current.Visitors, &out.Current.Visits, &out.Current.Pageviews, &out.Current.Bounces, &out.Current.TotalTime}
	prev := []*int64{&out.Previous.Visitors, &out.Previous.Visits, &out.Previous.Pageviews, &out.Previous.Bounces, &out.Previous.TotalTime}

	pv, ok := raw["pageviews"]
	if !ok {
		return out, 0, errors.New("réponse de /stats sans pages vues : format inconnu")
	}

	// 2.x : {"pageviews": {"value": 10, "prev": 8}, ...}
	if bytes.HasPrefix(bytes.TrimSpace(pv), []byte("{")) {
		for i, f := range fields {
			var v struct {
				Value float64 `json:"value"`
				Prev  float64 `json:"prev"`
			}
			if data, ok := raw[f]; ok {
				if err := json.Unmarshal(data, &v); err != nil {
					return out, 0, fmt.Errorf("champ %s illisible : %w", f, err)
				}
			}
			*cur[i], *prev[i] = int64(v.Value), int64(v.Prev)
		}
		return out, 2, nil
	}

	// 3.x : {"pageviews": 10, ..., "comparison": {"pageviews": 8, ...}}
	var comparison map[string]json.RawMessage
	if data, ok := raw["comparison"]; ok {
		_ = json.Unmarshal(data, &comparison)
	}
	for i, f := range fields {
		*cur[i] = number(raw[f])
		*prev[i] = number(comparison[f])
	}
	return out, 3, nil
}

// number lit un nombre JSON, ou une chaine qui en contient un (certaines
// bases rendent les sommes en texte). Zero sinon.
func number(data json.RawMessage) int64 {
	if len(data) == 0 {
		return 0
	}
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		return int64(f)
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
	}
	return 0
}

type xy struct {
	X string          `json:"x"`
	Y json.RawMessage `json:"y"`
}

// Timeseries rend la courbe des pages vues et des visiteurs, pas a pas.
func (u *Umami) Timeseries(ctx context.Context, websiteID string, r Range, unit Unit, timezone string) ([]Point, error) {
	q := rangeQuery(r)
	q.Set("unit", string(unit))
	q.Set("timezone", timezone)
	var raw struct {
		Pageviews []xy `json:"pageviews"`
		Sessions  []xy `json:"sessions"`
	}
	if err := u.get(ctx, "/websites/"+url.PathEscape(websiteID)+"/pageviews", q, &raw); err != nil {
		return nil, err
	}
	return mergeSeries(raw.Pageviews, raw.Sessions), nil
}

// mergeSeries aligne les deux series sur leur pas.
func mergeSeries(pageviews, sessions []xy) []Point {
	byAt := map[string]*Point{}
	var order []string
	add := func(rows []xy, set func(*Point, int64)) {
		for _, row := range rows {
			at := normalizeAt(row.X)
			p, ok := byAt[at]
			if !ok {
				p = &Point{At: at}
				byAt[at] = p
				order = append(order, at)
			}
			set(p, number(row.Y))
		}
	}
	add(pageviews, func(p *Point, n int64) { p.Pageviews = n })
	add(sessions, func(p *Point, n int64) { p.Visitors = n })

	// Les pas arrivent dans l'ordre ; deux series fusionnees peuvent se
	// croiser, le tri textuel suffit sur ce format.
	sort.Strings(order)
	out := make([]Point, 0, len(order))
	for _, at := range order {
		out = append(out, *byAt[at])
	}
	return out
}

// normalizeAt ramene « 2026-10-12 00:00:00 » et « 2026-10-12T00:00:00Z » a
// « 2026-10-12T00:00:00 ».
func normalizeAt(x string) string {
	x = strings.TrimSpace(x)
	x = strings.Replace(x, " ", "T", 1)
	if i := strings.IndexAny(x, "Z+"); i > 10 {
		x = x[:i]
	}
	if i := strings.IndexByte(x, '.'); i > 10 {
		x = x[:i]
	}
	if len(x) == 10 {
		x += "T00:00:00"
	}
	return x
}

// metricType traduit une dimension dans le nom de la version.
func metricType(dim Dimension, version int) string {
	if dim == Page {
		if version == 2 {
			return "url"
		}
		return "path"
	}
	return string(dim)
}

// Metrics rend un classement. Le nom du classement des pages depend de la
// version : s'il est refuse, l'autre est essaye, et la version corrigee.
func (u *Umami) Metrics(ctx context.Context, websiteID string, r Range, dim Dimension, limit int) ([]MetricRow, error) {
	version := u.CurrentVersion()
	if version == 0 {
		version = 3
	}
	rows, err := u.metrics(ctx, websiteID, r, metricType(dim, version), limit)
	var se statusError
	if dim == Page && errors.As(err, &se) && se.code == http.StatusBadRequest {
		other := 5 - version // 2 <-> 3
		rows, err = u.metrics(ctx, websiteID, r, metricType(dim, other), limit)
		if err == nil {
			u.setVersion(other)
		}
	}
	return rows, err
}

func (u *Umami) metrics(ctx context.Context, websiteID string, r Range, typ string, limit int) ([]MetricRow, error) {
	q := rangeQuery(r)
	q.Set("type", typ)
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw []xy
	if err := u.get(ctx, "/websites/"+url.PathEscape(websiteID)+"/metrics", q, &raw); err != nil {
		return nil, err
	}
	out := make([]MetricRow, 0, len(raw))
	for _, row := range raw {
		out = append(out, MetricRow{Value: row.X, Count: number(row.Y)})
	}
	return out, nil
}

// Active rend les visiteurs des cinq dernieres minutes : {visitors} en 3.x
// et dans les 2.x recentes, {x} dans les plus anciennes.
func (u *Umami) Active(ctx context.Context, websiteID string) (int64, error) {
	var raw json.RawMessage
	if err := u.get(ctx, "/websites/"+url.PathEscape(websiteID)+"/active", nil, &raw); err != nil {
		return 0, err
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.HasPrefix(trimmed, []byte("[")) {
		var list []map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &list); err != nil || len(list) == 0 {
			return 0, nil
		}
		return firstNumber(list[0]), nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return 0, fmt.Errorf("visiteurs actifs illisibles : %w", err)
	}
	return firstNumber(obj), nil
}

func firstNumber(obj map[string]json.RawMessage) int64 {
	for _, key := range []string{"visitors", "x"} {
		if v, ok := obj[key]; ok {
			return number(v)
		}
	}
	return 0
}
