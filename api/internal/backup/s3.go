package backup

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// S3 est un client minimal pour un stockage compatible S3 : poser, lister,
// supprimer. Signature AWS v4 ecrite ici plutot qu'un SDK de plusieurs
// dizaines de paquets pour trois verbes HTTP.
//
// Fonctionne avec Scaleway, OVH, Hetzner, Garage, MinIO… : tout ce qui parle
// S3 en style « chemin » (https://endpoint/bucket/cle).
type S3 struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	// Prefixe des cles, pour partager un seau : « piilot/ ».
	Prefix string

	// Client HTTP, celui par defaut si nul.
	HTTP *http.Client
}

// Configured dit si de quoi joindre un seau a ete donne.
func (s *S3) Configured() bool {
	return s != nil && s.Endpoint != "" && s.Bucket != "" && s.AccessKey != "" && s.SecretKey != ""
}

// Put televerse un fichier sous la cle donnee.
func (s *S3) Put(ctx context.Context, key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}

	// Le hachage du corps fait partie de la signature : le fichier est lu
	// deux fois, c'est le prix d'un televersement sans « unsigned payload »
	// que tous les fournisseurs n'acceptent pas.
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.objectURL(key), f)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/x-tar")

	return s.do(req, hex.EncodeToString(h.Sum(nil)))
}

// Delete supprime une cle. Supprimer une cle absente n'est pas une erreur.
func (s *S3) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.objectURL(key), nil)
	if err != nil {
		return err
	}
	return s.do(req, emptyHash)
}

// List rend les cles sous le prefixe, avec leur date de depot.
func (s *S3) List(ctx context.Context) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	token := ""
	for {
		q := url.Values{}
		q.Set("list-type", "2")
		q.Set("prefix", s.Prefix)
		if token != "" {
			q.Set("continuation-token", token)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.bucketURL()+"?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		s.sign(req, emptyHash)
		res, err := s.client().Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("S3 %s : %s", res.Status, strings.TrimSpace(string(body)))
		}

		var page struct {
			Contents []struct {
				Key          string    `xml:"Key"`
				LastModified time.Time `xml:"LastModified"`
			} `xml:"Contents"`
			IsTruncated           bool   `xml:"IsTruncated"`
			NextContinuationToken string `xml:"NextContinuationToken"`
		}
		if err := xml.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("liste S3 illisible : %w", err)
		}
		for _, c := range page.Contents {
			out[c.Key] = c.LastModified
		}
		if !page.IsTruncated || page.NextContinuationToken == "" {
			return out, nil
		}
		token = page.NextContinuationToken
	}
}

const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func (s *S3) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 30 * time.Minute}
}

func (s *S3) bucketURL() string {
	return strings.TrimRight(s.Endpoint, "/") + "/" + s.Bucket
}

func (s *S3) objectURL(key string) string {
	return s.bucketURL() + "/" + escapePath(key)
}

func (s *S3) do(req *http.Request, bodyHash string) error {
	s.sign(req, bodyHash)
	res, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("S3 %s : %s", res.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// sign appose la signature AWS v4 sur la requete.
func (s *S3) sign(req *http.Request, bodyHash string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	scopeDate := now.Format("20060102")
	region := s.Region
	if region == "" {
		region = "us-east-1"
	}

	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", bodyHash)

	// En-tetes signes, tries, en minuscules.
	names := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	if req.Header.Get("Content-Type") != "" {
		names = append(names, "content-type")
	}
	sort.Strings(names)
	var canonHeaders strings.Builder
	for _, n := range names {
		v := req.Header.Get(n)
		if n == "host" {
			v = req.URL.Host
		}
		canonHeaders.WriteString(n + ":" + strings.TrimSpace(v) + "\n")
	}
	signedHeaders := strings.Join(names, ";")

	// Requete canonique : les parametres de requete tries et encodes.
	query := req.URL.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var canonQuery strings.Builder
	for i, k := range keys {
		if i > 0 {
			canonQuery.WriteByte('&')
		}
		canonQuery.WriteString(awsEscape(k) + "=" + awsEscape(query.Get(k)))
	}

	canonical := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		canonQuery.String(),
		canonHeaders.String(),
		signedHeaders,
		bodyHash,
	}, "\n")

	scope := scopeDate + "/" + region + "/s3/aws4_request"
	toSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(sha256Sum([]byte(canonical))),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+s.SecretKey), []byte(scopeDate))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(toSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.AccessKey, scope, signedHeaders, signature,
	))
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// awsEscape encode comme AWS l'attend : RFC 3986, espace en %20, tilde nu.
func awsEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(s), "+", "%20"), "%7E", "~")
}

// escapePath encode une cle d'objet segment par segment : les « / » restent.
func escapePath(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = awsEscape(p)
	}
	return strings.Join(parts, "/")
}
