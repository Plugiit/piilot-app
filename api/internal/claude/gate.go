package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// Bornes d'une connexion : largement au-dessus d'une conversation, en dessous
// d'une boucle emballee.
const (
	callsPerWindow = 120
	callsWindow    = time.Minute
)

// scopesChallenge : ce que Claude doit demander, soit tout ce que Piilot
// propose aujourd'hui.
var scopesChallenge = strings.Join(usecase.OfferedScopes, " ")

// Gate authentifie chaque appel de /mcp avant le SDK.
//
// Les refus d'authentification doivent etre des 401 HTTP portant
// WWW-Authenticate : c'est ce qui declenche la connexion chez Claude. Un refus
// rendu par l'outil lui-meme arriverait en 200, et Claude afficherait juste le
// message.
func (s *Server) Gate(c fiber.Ctx) error {
	// Une session de navigateur n'a rien a faire ici, comme sous /bot.
	if c.Cookies(middleware.AccessCookieName) != "" {
		return s.unauthorized(c, "invalid_request", "Cookie de session refusé sur /mcp.")
	}
	if origin := c.Get(fiber.HeaderOrigin); origin != "" && !slices.Contains(s.cfg.Origins, origin) {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden", "error_description": "Origine non admise."})
	}

	raw, ok := bearer(c.Get(fiber.HeaderAuthorization))
	if !ok {
		return s.unauthorized(c, "", "Connexion requise.")
	}
	caller, ok, err := s.cfg.Auth.Authenticate(c.Context(), raw)
	if errors.Is(err, usecase.ErrClaudeNotAllowed) {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden", "error_description": usecase.ErrClaudeNotAllowed.Message})
	}
	if err != nil {
		s.cfg.Log.Error("authentification MCP", "error", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "server_error"})
	}
	if !ok {
		return s.unauthorized(c, "invalid_token", "Jeton invalide ou expiré.")
	}

	allowed, retryAfter, err := s.cfg.Limiter.Allow(c.Context(), "mcp:grant:"+caller.GrantID.String(), callsPerWindow, callsWindow)
	if err != nil {
		s.cfg.Log.Warn("compteur indisponible, appel MCP autorise", "error", err)
	} else if !allowed {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		return c.Status(http.StatusTooManyRequests).JSON(fiber.Map{"error": "rate_limited"})
	}

	// Le corps est lu comme le SDK le lira : un seul message, en clair, cles
	// exactes. Ce que la barriere ne sait pas lire ne passe pas. Les lots sont
	// refuses : Claude n'en envoie pas, et un lot ferait passer mille appels
	// pour un seul dans la borne ci-dessus.
	if enc := strings.TrimSpace(c.Get(fiber.HeaderContentEncoding)); enc != "" && !strings.EqualFold(enc, "identity") {
		return c.Status(http.StatusUnsupportedMediaType).JSON(fiber.Map{"error": "invalid_request", "error_description": "Corps compressé refusé."})
	}
	method, tool, ok := readCall(c.BodyRaw())
	if !ok {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid_request", "error_description": "Un message JSON-RPC, seul, attendu."})
	}

	// L'ecriture se refuse ici, en 403 insufficient_scope : Claude propose
	// alors de reconnecter avec la portee manquante. L'outil le reverifie :
	// c'est lui qui fait foi.
	if method == "tools/call" && s.kinds[tool] && !caller.CanWrite() {
		c.Set(fiber.HeaderWWWAuthenticate, fmt.Sprintf(`Bearer error="insufficient_scope", scope=%q, resource_metadata=%q`,
			usecase.ScopeRead+" "+usecase.ScopeWrite, s.cfg.ResourceMetadata))
		return c.Status(http.StatusForbidden).JSON(fiber.Map{
			"error":             "insufficient_scope",
			"error_description": "L'outil " + tool + " modifie Piilot : autorisez les modifications.",
		})
	}

	c.Locals(sessionKey{}, &session{caller: caller, ip: strings.Clone(c.IP())})
	return c.Next()
}

// unauthorized repond 401 avec le defi qui lance la connexion chez Claude. Le
// detail lisible va dans le corps : l'en-tete reste en ASCII.
func (s *Server) unauthorized(c fiber.Ctx, code, description string) error {
	challenge := fmt.Sprintf(`Bearer resource_metadata=%q, scope=%q`, s.cfg.ResourceMetadata, scopesChallenge)
	if code != "" {
		challenge = fmt.Sprintf(`Bearer error=%q, resource_metadata=%q, scope=%q`, code, s.cfg.ResourceMetadata, scopesChallenge)
	}
	c.Set(fiber.HeaderWWWAuthenticate, challenge)
	body := fiber.Map{"error": "unauthorized", "error_description": description}
	if code != "" {
		body["error"] = code
	}
	return c.Status(http.StatusUnauthorized).JSON(body)
}

// readCall lit un message JSON-RPC : un objet seul, rien apres, cles prises a
// la lettre comme le fait le SDK (une map, et non une structure que
// encoding/json remplirait sans tenir compte de la casse). Rend la methode et,
// pour tools/call, le nom de l'outil.
func readCall(body []byte) (method, tool string, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var msg map[string]json.RawMessage
	if err := dec.Decode(&msg); err != nil || msg == nil {
		return "", "", false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return "", "", false
	}
	if raw, present := msg["method"]; present && json.Unmarshal(raw, &method) != nil {
		return "", "", false
	}
	if method != "tools/call" {
		return method, "", true
	}
	var params map[string]json.RawMessage
	if json.Unmarshal(msg["params"], &params) != nil || json.Unmarshal(params["name"], &tool) != nil {
		return "", "", false
	}
	return method, tool, true
}

func bearer(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
