// Package gateway est la passerelle de Piilot : le seul point d'entree du
// trafic, place devant l'application.
//
// Elle ne fait qu'une chose, mais c'est elle qui rend la mise a jour
// invisible : pendant que l'updater remplace l'application, deux instances
// tournent un moment cote a cote, et la passerelle choisit a chaque requete
// celle qui doit la recevoir. Elle ne depend d'aucun outil de deploiement —
// Coolify, Traefik, nginx ou rien du tout devant elle, c'est pareil.
//
//   - Elle retrouve les instances par le nom DNS du service (`server`), que
//     Docker resout vers tous les conteneurs qui le portent.
//   - Elle les sonde chaque seconde sur /health/live et n'envoie le trafic
//     qu'a une instance saine — la plus recente quand il y en a deux. Une
//     instance qui s'arrete se declare en arret (503) avant de fermer : elle
//     est ecartee sans qu'une requete echoue.
//   - Une requete qui n'a pas pu joindre l'instance — connexion refusee, rien
//     n'a ete envoye — repart vers une autre. Sans instance saine, elle
//     attend qu'il y en ait une, quelques secondes au plus.
//   - Au-dela, elle sert une page de maintenance qui se recharge seule, et
//     un 503 aux appels d'API.
//
// Elle reste volontairement simple et ne change presque jamais : c'est la
// seule piece qui ne peut pas etre remplacee sans coupure.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config regle la passerelle.
type Config struct {
	// Upstream : nom DNS et port des instances, « server:8080 ».
	Upstream string
	// ProbeInterval : rythme des sondes. Une instance qui s'arrete continue de
	// servir quelques secondes (DRAIN_DELAY) : la sonde doit passer plusieurs
	// fois dans ce delai.
	ProbeInterval time.Duration
	// Hold : temps qu'une requete peut attendre une instance saine avant la
	// page de maintenance.
	Hold time.Duration
}

// Path de la sonde propre a la passerelle : elle repond toujours, et dit si
// une instance peut servir. La page de maintenance l'interroge pour savoir
// quand recharger.
const statusPath = "/health/gateway"

// instance est une instance de l'application, vue par la sonde.
type instance struct {
	addr    string // ip:port
	healthy bool
	started time.Time
	// successes compte les sondes reussies d'affilee : une instance toute
	// neuve ne recoit du trafic qu'apres deux sondes, pas sur un premier
	// signe de vie.
	successes int
}

// Gateway est le proxy.
type Gateway struct {
	cfg   Config
	log   *slog.Logger
	proxy *httputil.ReverseProxy
	probe *http.Client

	mu        sync.Mutex
	instances map[string]*instance
	// active est l'adresse qui recoit le trafic, vide s'il n'y en a pas.
	active atomic.Value // string
	// ready est ferme puis remplace chaque fois qu'une instance devient
	// active : les requetes en attente s'y reveillent.
	ready chan struct{}

	resolve func(ctx context.Context, host string) ([]string, error)
}

// New construit la passerelle.
func New(cfg Config, log *slog.Logger) *Gateway {
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = time.Second
	}
	if cfg.Hold <= 0 {
		cfg.Hold = 15 * time.Second
	}

	g := &Gateway{
		cfg:       cfg,
		log:       log,
		probe:     &http.Client{Timeout: cfg.ProbeInterval},
		instances: map[string]*instance{},
		ready:     make(chan struct{}),
		resolve:   net.DefaultResolver.LookupHost,
	}
	g.active.Store("")

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.MaxIdleConnsPerHost = 64

	g.proxy = &httputil.ReverseProxy{
		Rewrite:      g.rewrite,
		Transport:    &retrying{g: g, base: transport},
		ErrorHandler: g.fail,
	}

	return g
}

// Run sonde les instances jusqu'a l'arret du contexte.
func (g *Gateway) Run(ctx context.Context) {
	g.refresh(ctx)

	tick := time.NewTicker(g.cfg.ProbeInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			g.refresh(ctx)
		}
	}
}

// refresh resout le nom du service, sonde chaque instance et choisit celle
// qui recoit le trafic.
func (g *Gateway) refresh(ctx context.Context) {
	host, port, err := net.SplitHostPort(g.cfg.Upstream)
	if err != nil {
		g.log.Error("UPSTREAM invalide", "upstream", g.cfg.Upstream, "error", err)
		return
	}

	lookupCtx, cancel := context.WithTimeout(ctx, g.cfg.ProbeInterval)
	ips, err := g.resolve(lookupCtx, host)
	cancel()
	if err != nil {
		// Aucun conteneur ne porte le nom : l'application est arretee, ou en
		// train d'etre recreee. Ce n'est pas une erreur de la passerelle.
		ips = nil
	}

	seen := map[string]bool{}
	var wg sync.WaitGroup
	for _, ip := range ips {
		addr := net.JoinHostPort(ip, port)
		// Une adresse qui porte deja son port vient des tests, ou des
		// instances ecoutent sur une meme IP.
		if _, _, err := net.SplitHostPort(ip); err == nil {
			addr = ip
		}
		seen[addr] = true

		g.mu.Lock()
		inst, ok := g.instances[addr]
		if !ok {
			inst = &instance{addr: addr}
			g.instances[addr] = inst
		}
		g.mu.Unlock()

		wg.Add(1)
		go func() {
			defer wg.Done()
			g.check(ctx, inst)
		}()
	}
	wg.Wait()

	g.mu.Lock()
	for addr := range g.instances {
		if !seen[addr] {
			delete(g.instances, addr)
		}
	}
	g.elect()
	g.mu.Unlock()
}

// check sonde une instance.
func (g *Gateway) check(ctx context.Context, inst *instance) {
	healthy, started := false, time.Time{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+inst.addr+"/health/live", nil)
	if err == nil {
		if res, err := g.probe.Do(req); err == nil {
			var body struct {
				StartedAt time.Time `json:"started_at"`
			}
			_ = json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&body)
			_ = res.Body.Close()
			healthy, started = res.StatusCode == http.StatusOK, body.StartedAt
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	inst.healthy = healthy
	if healthy {
		inst.successes++
		inst.started = started
	} else {
		inst.successes = 0
	}
}

// elect choisit l'instance active. Appelee sous verrou.
//
// L'instance active le reste tant qu'elle est saine, sauf si une plus recente
// est prete : c'est la bascule d'une mise a jour. Une seule instance recoit le
// trafic a la fois — reparties sur deux versions, les requetes d'un meme
// ecran demanderaient a l'une des fichiers que seule l'autre connait.
func (g *Gateway) elect() {
	current := g.active.Load().(string)

	var best *instance
	for _, inst := range g.instances {
		eligible := inst.healthy && (inst.successes >= 2 || inst.addr == current)
		if !eligible {
			continue
		}
		if best == nil || inst.started.After(best.started) {
			best = inst
		}
	}

	next := ""
	if best != nil {
		next = best.addr
	}
	if next == current {
		return
	}

	g.active.Store(next)
	switch {
	case next == "":
		g.log.Warn("aucune instance saine : les requetes attendent")
	case current == "":
		g.log.Info("instance active", "addr", next)
		close(g.ready)
		g.ready = make(chan struct{})
	default:
		g.log.Info("bascule vers une autre instance", "from", current, "to", next)
	}
}

// wait rend l'instance active, en attendant qu'il y en ait une, au plus Hold.
func (g *Gateway) wait(ctx context.Context) (string, bool) {
	deadline := time.NewTimer(g.cfg.Hold)
	defer deadline.Stop()

	for {
		if addr := g.active.Load().(string); addr != "" {
			return addr, true
		}

		g.mu.Lock()
		ready := g.ready
		g.mu.Unlock()

		select {
		case <-ready:
		case <-deadline.C:
			return "", false
		case <-ctx.Done():
			return "", false
		}
	}
}

// discard ecarte une instance injoignable sans attendre la prochaine sonde.
func (g *Gateway) discard(addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if inst, ok := g.instances[addr]; ok {
		inst.healthy = false
		inst.successes = 0
	}
	g.elect()
}

// ServeHTTP aiguille une requete.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == statusPath {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ready": g.active.Load().(string) != ""})
		return
	}

	g.proxy.ServeHTTP(w, r)
}

// rewrite prepare la requete transmise. L'hote reste celui demande par le
// navigateur : l'application en deduit l'espace a servir.
func (g *Gateway) rewrite(pr *httputil.ProxyRequest) {
	pr.SetURL(&url.URL{Scheme: "http", Host: "upstream"})
	pr.Out.Host = pr.In.Host

	trusted := fromPrivate(pr.In.RemoteAddr)
	header := func(name, fallback string) string {
		if v := pr.In.Header.Get(name); trusted && v != "" {
			return v
		}
		return fallback
	}

	pr.Out.Header.Set("X-Forwarded-For", ClientIP(pr.In.RemoteAddr, pr.In.Header.Values("X-Forwarded-For")))
	pr.Out.Header.Set("X-Forwarded-Host", header("X-Forwarded-Host", pr.In.Host))
	proto := "http"
	if pr.In.TLS != nil {
		proto = "https"
	}
	pr.Out.Header.Set("X-Forwarded-Proto", header("X-Forwarded-Proto", proto))
}

// ClientIP rend l'adresse de l'appelant.
//
// On remonte la chaine depuis la connexion : tant que l'adresse est privee,
// c'est un proxy de l'installation (Traefik, nginx…), et l'on passe a celle
// qu'il a recue. La premiere adresse publique est l'appelant. Ce qui precede
// dans X-Forwarded-For a pu etre ecrit par lui : on ne le lit pas.
func ClientIP(remoteAddr string, forwarded []string) string {
	chain := []string{}
	for _, value := range forwarded {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				chain = append(chain, part)
			}
		}
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	current := host
	for i := len(chain) - 1; i >= 0; i-- {
		if !isPrivate(current) {
			break
		}
		if _, err := netip.ParseAddr(chain[i]); err != nil {
			break
		}
		current = chain[i]
	}

	return current
}

func fromPrivate(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return isPrivate(host)
}

func isPrivate(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
}

// fail repond quand aucune instance n'a pu servir la requete.
func (g *Gateway) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		// Le navigateur a abandonne : il n'y a plus personne a qui repondre.
		return
	}

	var unavailable *unavailableError
	if !errors.As(err, &unavailable) {
		// L'instance a ete jointe mais n'a pas repondu jusqu'au bout. La
		// requete a pu etre traitee : on ne la rejoue pas.
		g.log.Warn("requete interrompue", "path", r.URL.Path, "error", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	w.Header().Set("Retry-After", "5")
	w.Header().Set("Cache-Control", "no-store")

	if strings.HasPrefix(r.URL.Path, "/api/") || !acceptsHTML(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    "MAINTENANCE",
			"message": "Piilot est en cours de mise à jour. Réessayez dans un instant.",
			"details": map[string]any{},
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, maintenancePage)
}

func acceptsHTML(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html")
}

// unavailableError : aucune instance n'a pu recevoir la requete.
type unavailableError struct{}

func (*unavailableError) Error() string { return "aucune instance disponible" }

// retrying envoie la requete a l'instance active, et la renvoie a une autre
// quand la connexion n'a pas pu s'etablir.
//
// Seul ce cas se rejoue : rien n'est parti, l'application n'a rien vu. Une
// requete coupee apres l'envoi a pu etre traitee — creer un client, envoyer
// un e-mail —, la rejouer risquerait de le faire deux fois.
type retrying struct {
	g    *Gateway
	base http.RoundTripper
}

func (t *retrying) RoundTrip(req *http.Request) (*http.Response, error) {
	// Le corps est garde ouvert entre deux essais. Le transport ne le lit
	// qu'une fois la connexion etablie : apres un echec de connexion, il est
	// intact.
	var body *keptBody
	if req.Body != nil && req.Body != http.NoBody {
		body = &keptBody{ReadCloser: req.Body}
		req.Body = body
	}

	for {
		addr, ok := t.g.wait(req.Context())
		if !ok {
			if err := req.Context().Err(); err != nil {
				return nil, err
			}
			return nil, &unavailableError{}
		}

		out := req.Clone(req.Context())
		out.URL.Host = addr
		if body != nil {
			out.Body = body
		}

		res, err := t.base.RoundTrip(out)
		if err == nil {
			return res, nil
		}

		var op *net.OpError
		if !errors.As(err, &op) || op.Op != "dial" || (body != nil && body.read) {
			return nil, err
		}

		t.g.log.Warn("instance injoignable, requete renvoyee", "addr", addr, "error", err)
		t.g.discard(addr)
	}
}

// keptBody est un corps de requete que le transport ne peut pas fermer, et
// qui dit s'il a commence a etre lu.
type keptBody struct {
	io.ReadCloser
	read bool
}

func (b *keptBody) Read(p []byte) (int, error) {
	b.read = true
	return b.ReadCloser.Read(p)
}

func (b *keptBody) Close() error { return nil }
