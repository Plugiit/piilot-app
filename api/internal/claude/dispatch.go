package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"

	"github.com/plugiit/piilot-app/api/internal/middleware"
)

// route : une route REST que les outils ont le droit de rappeler. Le motif
// suit l'OpenAPI : /api/v1/admin/tasks/{id}.
type route struct {
	method  string
	pattern string
}

// sessionTTL : le jeton de session signe pour un rappel ne sert que le temps
// de ce rappel, et ne quitte jamais le processus.
const sessionTTL = time.Minute

var errRouteNotAllowed = errors.New("route non ouverte aux outils")

// apiError : le refus d'une route, tel que l'interface l'aurait recu.
type apiError struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func (e *apiError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = fmt.Sprintf("Erreur HTTP %d", e.Status)
	}
	if e.Code != "" {
		msg += " (" + e.Code + ")"
	}
	if len(e.Details) > 0 {
		if d, err := json.Marshal(e.Details); err == nil {
			msg += " " + string(d)
		}
	}
	return msg
}

// request : un rappel a faire.
type request struct {
	route route
	// ids remplissent les {…} du motif, dans l'ordre.
	ids   []uuid.UUID
	query url.Values
	// body : encode en JSON. Ignore si raw est renseigne.
	body any
	// raw et contentType : un corps deja forme (multipart).
	raw         []byte
	contentType string
}

// call rappelle une route REST dans le processus, au nom du compte de la
// session, et rend sa reponse decodee.
//
// La requete traverse toute la chaine de l'application : garde de session
// (compte actif, role), permission de la route, validation, audit. Le compte
// agit comme depuis l'interface, rien de plus.
func (s *Server) call(_ context.Context, sess *session, r request) (any, error) {
	if !s.routes[r.route] {
		return nil, errRouteNotAllowed
	}
	handler := s.router.Load()
	if handler == nil {
		return nil, errors.New("serveur en cours de demarrage, reessayez")
	}
	path, err := r.route.build(r.ids)
	if err != nil {
		return nil, err
	}
	token, err := s.cfg.Signer.Sign(sess.caller.UserID, sess.caller.Role, sessionTTL)
	if err != nil {
		return nil, fmt.Errorf("signature de la session : %w", err)
	}

	var req fasthttp.Request
	req.Header.SetMethod(r.route.method)
	if len(r.query) > 0 {
		path += "?" + r.query.Encode()
	}
	req.SetRequestURI(path)
	req.Header.SetHost(s.host())
	req.Header.Set(fiber.HeaderUserAgent, "Piilot MCP ("+sess.caller.Client+")")
	req.Header.Set(fiber.HeaderAccept, fiber.MIMEApplicationJSON)
	req.Header.SetCookie(middleware.AccessCookieName, token)
	switch {
	case r.raw != nil:
		req.Header.SetContentType(r.contentType)
		req.SetBody(r.raw)
	case r.body != nil:
		payload, err := json.Marshal(r.body)
		if err != nil {
			return nil, fmt.Errorf("encodage de la requete : %w", err)
		}
		req.Header.SetContentType(fiber.MIMEApplicationJSON)
		req.SetBody(payload)
	}

	var rc fasthttp.RequestCtx
	rc.Init(&req, remoteAddr(sess.ip), nil)
	(*handler)(&rc)

	status := rc.Response.StatusCode()
	body := bytes.Clone(rc.Response.Body())
	if status >= 400 {
		e := &apiError{Status: status}
		_ = json.Unmarshal(body, e)
		return nil, e
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]any{"ok": true}, nil
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("reponse illisible : %w", err)
	}
	// Le resultat structure d'un outil est un objet JSON.
	if _, isObject := out.(map[string]any); !isObject {
		out = map[string]any{"items": out}
	}
	return out, nil
}

// open declare les routes qu'un outil rappelle.
func (s *Server) open(routes ...route) {
	for _, r := range routes {
		s.routes[r] = true
	}
}

// build remplit le motif. Les identifiants sont des UUID reformates : rien
// d'autre n'entre dans le chemin.
func (r route) build(ids []uuid.UUID) (string, error) {
	var b strings.Builder
	rest := r.pattern
	for _, id := range ids {
		start := strings.IndexByte(rest, '{')
		end := strings.IndexByte(rest, '}')
		if start < 0 || end < start {
			return "", fmt.Errorf("motif %s : trop d'identifiants", r.pattern)
		}
		b.WriteString(rest[:start])
		b.WriteString(id.String())
		rest = rest[end+1:]
	}
	if strings.ContainsRune(rest, '{') {
		return "", fmt.Errorf("motif %s : identifiant manquant", r.pattern)
	}
	b.WriteString(rest)
	return b.String(), nil
}

// host : l'hote de la ressource, pour que l'application voie le rappel
// arriver sur son propre domaine.
func (s *Server) host() string {
	u, err := url.Parse(s.cfg.Resource)
	if err != nil {
		return "localhost"
	}
	return u.Host
}

// remoteAddr : l'adresse de Claude, pour que le journal et les bornes voient
// le meme appelant que la barriere.
func remoteAddr(ip string) net.Addr {
	addr := &net.TCPAddr{IP: net.ParseIP(ip)}
	if addr.IP == nil {
		addr.IP = net.IPv4zero
	}
	return addr
}

// parseID lit un identifiant fourni par Claude.
func parseID(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s : identifiant invalide (UUID attendu)", field)
	}
	return id, nil
}
