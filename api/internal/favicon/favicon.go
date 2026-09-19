// Package favicon va chercher l'icone d'un site pour en faire un logo.
//
// Tout ce qui est ici sort sur le reseau vers une adresse fournie par
// l'utilisateur. C'est exactement la forme d'une SSRF : sans garde-fou, une app
// dont l'adresse pointe vers 169.254.169.254 ferait interroger les metadonnees
// de la machine par l'API elle-meme. Le blocage se fait sur l'IP reellement
// contactee, pas sur le nom : un nom public peut resoudre vers une adresse
// privee, et resoudre deux fois laisserait passer entre les deux verifications.
package favicon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ErrBlocked dit qu'une adresse a ete refusee avant meme d'etre contactee.
var ErrBlocked = errors.New("adresse interdite")

// ErrNotFound dit qu'aucune icone exploitable n'a ete trouvee.
var ErrNotFound = errors.New("aucune icone trouvee")

const (
	// maxIcon borne ce qu'on accepte de telecharger. Une icone plus lourde
	// n'est pas une icone.
	maxIcon int64 = 256 << 10

	// maxHTML borne la lecture de la page ou l'on cherche la balise. L'entete
	// d'un document tient largement dedans, et le reste ne nous interesse pas.
	maxHTML int64 = 512 << 10

	// timeout borne l'ensemble d'une recuperation. Le job n'attend pas un site
	// lent : il reessaiera plus tard.
	timeout = 8 * time.Second
)

// Types acceptes. L'ico y figure : c'est encore le format le plus repandu pour
// un favicon, et les navigateurs l'affichent dans une balise img.
var accepted = map[string]bool{
	"image/png":                true,
	"image/jpeg":               true,
	"image/webp":               true,
	"image/gif":                true,
	"image/svg+xml":            true,
	"image/x-icon":             true,
	"image/vnd.microsoft.icon": true,
}

// Icon est ce qu'une recuperation rapporte.
type Icon struct {
	Content     []byte
	ContentType string
}

// Fetcher va chercher des icones. Son client refuse de sortir vers le reseau
// local, redirections comprises.
type Fetcher struct {
	client *http.Client
}

// New construit un recuperateur.
func New() *Fetcher {
	dialer := &net.Dialer{Timeout: 5 * time.Second}

	transport := &http.Transport{
		// Le controle vit dans le dialer et non avant l'appel : c'est le seul
		// endroit ou l'on connait l'adresse reellement jointe. Verifier le nom
		// en amont laisserait passer un DNS qui repond autre chose au moment
		// de la connexion.
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}

			for _, ip := range ips {
				if isPrivate(ip.IP) {
					return nil, fmt.Errorf("%w : %s", ErrBlocked, ip.IP)
				}
			}

			// On se connecte a l'adresse qui vient d'etre validee, et non au
			// nom : une seconde resolution pourrait rendre autre chose.
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		TLSHandshakeTimeout: 5 * time.Second,
		DisableKeepAlives:   true,
	}

	return &Fetcher{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Le dialer revalide chaque saut, mais une chaine sans fin
				// ferait tourner le job pour rien.
				if len(via) >= 5 {
					return errors.New("trop de redirections")
				}

				if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
					return fmt.Errorf("%w : schema %q", ErrBlocked, req.URL.Scheme)
				}

				return nil
			},
		},
	}
}

// isPrivate reconnait les adresses qu'on ne doit jamais joindre depuis l'API.
//
// La boucle locale et les reseaux prives sont evidents. Le lien-local l'est
// moins et c'est le plus dangereux : 169.254.169.254 sert les identifiants de
// la machine chez la plupart des hebergeurs.
func isPrivate(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// Fetch rapporte l'icone du site designe.
//
// La page est lue d'abord, pour sa balise `link` : c'est la que vit l'icone
// d'un site moderne, souvent en plusieurs tailles. Le repli sur /favicon.ico
// vient ensuite, parce qu'il existe toujours meme quand rien ne le declare.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Icon, error) {
	base, err := url.Parse(rawURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return Icon{}, fmt.Errorf("%w : adresse inexploitable", ErrNotFound)
	}

	candidates := f.declared(ctx, base)
	candidates = append(candidates, base.ResolveReference(&url.URL{Path: "/favicon.ico"}).String())

	var last error
	for _, candidate := range candidates {
		icon, err := f.download(ctx, candidate)
		if err == nil {
			return icon, nil
		}

		last = err
	}

	if last == nil {
		last = ErrNotFound
	}

	return Icon{}, last
}

// declared lit la page et rend les icones qu'elle annonce, la mieux definie en
// premier.
//
// Un echec ici n'est pas une erreur : une page illisible ou absente laisse
// simplement le repli faire son travail.
func (f *Fetcher) declared(ctx context.Context, base *url.URL) []string {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil
	}

	response, err := f.client.Do(request)
	if err != nil {
		return nil
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil
	}

	document, err := html.Parse(io.LimitReader(response.Body, maxHTML))
	if err != nil {
		return nil
	}

	type found struct {
		href string
		size int
	}

	var icons []found

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "link" {
			var rel, href, sizes string
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "rel":
					rel = strings.ToLower(attr.Val)
				case "href":
					href = attr.Val
				case "sizes":
					sizes = attr.Val
				}
			}

			// « apple-touch-icon » est souvent la plus grande et la plus
			// soignee : un site qui la fournit l'a dessinee pour etre vue.
			if href != "" && (strings.Contains(rel, "icon") || rel == "apple-touch-icon") {
				if resolved, err := base.Parse(href); err == nil {
					icons = append(icons, found{href: resolved.String(), size: parseSize(sizes)})
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)

	// La plus grande d'abord : dans un rail, une icone trop petite se voit.
	for i := 1; i < len(icons); i++ {
		for j := i; j > 0 && icons[j].size > icons[j-1].size; j-- {
			icons[j], icons[j-1] = icons[j-1], icons[j]
		}
	}

	out := make([]string, 0, len(icons))
	for _, icon := range icons {
		out = append(out, icon.href)
	}

	return out
}

// parseSize lit « 32x32 » et rend 32. Rend 0 quand l'attribut manque ou dit
// « any », ce qui ne permet pas de comparer.
func parseSize(sizes string) int {
	fields := strings.Fields(strings.ToLower(sizes))
	best := 0

	for _, field := range fields {
		parts := strings.Split(field, "x")
		if len(parts) != 2 {
			continue
		}

		value := 0
		for _, r := range parts[0] {
			if r < '0' || r > '9' {
				value = 0

				break
			}

			value = value*10 + int(r-'0')
		}

		if value > best {
			best = value
		}
	}

	return best
}

// download rapporte un fichier, s'il ressemble a une icone.
func (f *Fetcher) download(ctx context.Context, rawURL string) (Icon, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Icon{}, err
	}

	response, err := f.client.Do(request)
	if err != nil {
		return Icon{}, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return Icon{}, fmt.Errorf("%w : statut %d", ErrNotFound, response.StatusCode)
	}

	kind := response.Header.Get("Content-Type")
	if index := strings.IndexByte(kind, ';'); index >= 0 {
		kind = kind[:index]
	}
	kind = strings.ToLower(strings.TrimSpace(kind))

	if !accepted[kind] {
		return Icon{}, fmt.Errorf("%w : type %q", ErrNotFound, kind)
	}

	// Une lecture d'un octet de plus que la borne dit que le fichier la
	// depasse, sans avoir a se fier a Content-Length, qui peut mentir.
	content, err := io.ReadAll(io.LimitReader(response.Body, maxIcon+1))
	if err != nil {
		return Icon{}, err
	}

	if int64(len(content)) > maxIcon {
		return Icon{}, fmt.Errorf("%w : icone trop lourde", ErrNotFound)
	}

	if len(content) == 0 {
		return Icon{}, fmt.Errorf("%w : icone vide", ErrNotFound)
	}

	return Icon{Content: content, ContentType: kind}, nil
}
