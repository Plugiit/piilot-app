// Package spaces repartit Piilot sur plusieurs domaines.
//
// L'application compte quatre espaces : la connexion (auth), le travail
// quotidien de l'agence (team), son administration (admin) et le portail des
// clients (client). Par defaut, ils partagent un seul domaine et se
// distinguent par leurs chemins. Avec AUTH_URL, TEAM_URL, ADMIN_URL et
// CLIENT_URL, chacun recoit le sien.
//
// C'est toujours la meme application, servie par la meme image : chaque
// domaine sert le front et l'API, et une adresse ouverte sur le mauvais domaine
// est renvoyee vers le bon. Le back-office se sert sur deux domaines : celui
// de l'equipe, et celui de l'administration, ou les administrateurs font tout
// leur travail. Les domaines partagent un parent (auth.agence.fr,
// team.agence.fr…) pour qu'une seule connexion vaille partout : le cookie de
// session est pose sur ce parent.
//
// La correspondance chemin → espace existe aussi cote front, dans
// web/src/lib/spaces.ts : les deux doivent bouger ensemble.
package spaces

import (
	"fmt"
	"net/url"
	"strings"
)

// Les quatre espaces.
const (
	Auth   = "auth"
	Team   = "team"
	Admin  = "admin"
	Client = "client"
)

// Spaces porte l'origine de chaque espace (« https://team.agence.fr »), ou
// rien en mode mono-domaine.
type Spaces struct {
	AuthURL   string
	TeamURL   string
	AdminURL  string
	ClientURL string
	// CookieDomain est le parent commun, avec son point : « .agence.fr ».
	CookieDomain string
}

// Parse lit et verifie les quatre origines. Toutes ou aucune : un espace sans
// domaine en mode multi-domaines n'aurait nulle part ou etre servi.
//
// cookieDomain peut etre vide : il se deduit alors du parent commun des
// quatre domaines.
func Parse(auth, team, admin, client, cookieDomain string) (Spaces, error) {
	raw := map[string]string{"AUTH_URL": auth, "TEAM_URL": team, "ADMIN_URL": admin, "CLIENT_URL": client}

	set := 0
	for _, v := range raw {
		if strings.TrimSpace(v) != "" {
			set++
		}
	}
	if set == 0 {
		return Spaces{}, nil
	}
	if set != len(raw) {
		return Spaces{}, fmt.Errorf("AUTH_URL, TEAM_URL, ADMIN_URL et CLIENT_URL se renseignent ensemble, ou pas du tout")
	}

	origins := map[string]string{}
	hosts := map[string]string{}
	for name, v := range raw {
		u, err := url.Parse(strings.TrimSpace(v))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || (u.Path != "" && u.Path != "/") {
			return Spaces{}, fmt.Errorf("%s doit etre une origine, comme https://team.agence.fr (recu : %q)", name, v)
		}
		origins[name] = u.Scheme + "://" + u.Host
		hosts[name] = u.Hostname()
	}

	seen := map[string]string{}
	for name, h := range hosts {
		if other, ok := seen[h]; ok {
			return Spaces{}, fmt.Errorf("%s et %s designent le meme domaine (%s)", other, name, h)
		}
		seen[h] = name
	}

	if cookieDomain == "" {
		cookieDomain = commonParent(hosts["AUTH_URL"], hosts["TEAM_URL"], hosts["ADMIN_URL"], hosts["CLIENT_URL"])
		if cookieDomain == "" {
			return Spaces{}, fmt.Errorf("les quatre domaines doivent partager un domaine parent (comme .agence.fr) pour qu'une connexion vaille partout")
		}
	}
	if !strings.HasPrefix(cookieDomain, ".") {
		cookieDomain = "." + cookieDomain
	}
	for name, h := range hosts {
		if !strings.HasSuffix("."+h, cookieDomain) {
			return Spaces{}, fmt.Errorf("%s (%s) n'est pas sous COOKIE_DOMAIN (%s)", name, h, cookieDomain)
		}
	}

	return Spaces{
		AuthURL:      origins["AUTH_URL"],
		TeamURL:      origins["TEAM_URL"],
		AdminURL:     origins["ADMIN_URL"],
		ClientURL:    origins["CLIENT_URL"],
		CookieDomain: cookieDomain,
	}, nil
}

// commonParent rend le plus long suffixe de domaine commun, avec son point,
// s'il compte au moins deux etiquettes : « .agence.fr », jamais « .fr ».
func commonParent(hosts ...string) string {
	split := make([][]string, len(hosts))
	for i, h := range hosts {
		split[i] = strings.Split(h, ".")
	}

	var common []string
	for depth := 1; ; depth++ {
		var label string
		for i, parts := range split {
			// Le parent est strictement au-dessus de chaque domaine.
			if len(parts) <= depth {
				return join(common)
			}
			l := parts[len(parts)-depth]
			if i == 0 {
				label = l
			} else if l != label {
				return join(common)
			}
		}
		common = append([]string{label}, common...)
	}
}

func join(labels []string) string {
	if len(labels) < 2 {
		return ""
	}
	return "." + strings.Join(labels, ".")
}

// Enabled dit si les espaces ont chacun leur domaine.
func (s Spaces) Enabled() bool { return s.TeamURL != "" }

// URL rend l'origine d'un espace.
func (s Spaces) URL(space string) string {
	switch space {
	case Auth:
		return s.AuthURL
	case Admin:
		return s.AdminURL
	case Client:
		return s.ClientURL
	default:
		return s.TeamURL
	}
}

// OfHost rend l'espace d'un hote, ou une chaine vide pour un hote inconnu —
// une sonde, une adresse IP : rien ne lui est impose.
func (s Spaces) OfHost(host string) string {
	for _, c := range []struct{ space, origin string }{
		{Auth, s.AuthURL}, {Team, s.TeamURL}, {Admin, s.AdminURL}, {Client, s.ClientURL},
	} {
		if u, err := url.Parse(c.origin); err == nil && strings.EqualFold(u.Host, host) {
			return c.space
		}
	}

	return ""
}

// OfPath rend l'espace auquel appartient une adresse de l'application.
//
// Team designe le back-office en general : il est servi sur le domaine de
// l'equipe comme sur celui de l'administration, ou les administrateurs font
// tout leur travail. Admin designe les seuls ecrans d'administration — le
// tableau de bord de l'agence et les parametres —, qui n'existent que sur le
// domaine d'administration. Vide pour la racine, que chaque domaine redirige
// vers son accueil.
func OfPath(path string) string {
	under := func(prefix string) bool { return path == prefix || strings.HasPrefix(path, prefix+"/") }

	switch {
	case path == "/" || path == "":
		return ""
	case under("/login"), under("/invitation"), under("/mot-de-passe-oublie"), under("/reinitialiser"):
		return Auth
	case under("/client"):
		return Client
	// Le tableau de bord de l'agence est la racine du module PM : seule
	// l'adresse exacte releve de l'administration, ses voisines sont du
	// back-office ordinaire.
	case path == "/pm" || path == "/pm/", under("/parametres"):
		return Admin
	default:
		return Team
	}
}

// Redirect rend l'adresse vers laquelle renvoyer une requete servie sur le
// mauvais domaine, ou une chaine vide quand elle est au bon endroit.
//
// Le serveur ne connait pas le role de l'appelant a cet endroit : une page du
// back-office ouverte sur la connexion ou le portail part vers le domaine de
// l'equipe, et le front y envoie ensuite un administrateur vers le sien.
func (s Spaces) Redirect(host, path, rawQuery string) string {
	if !s.Enabled() {
		return ""
	}

	current := s.OfHost(host)
	if current == "" {
		return ""
	}

	var target string
	switch want := OfPath(path); want {
	case "":
		return ""
	case Team:
		// Le back-office se sert sur l'un ou l'autre des deux domaines.
		if current == Team || current == Admin {
			return ""
		}
		target = Team
	default:
		if want == current {
			return ""
		}
		target = want
	}

	out := s.URL(target) + path
	if rawQuery != "" {
		out += "?" + rawQuery
	}

	return out
}
