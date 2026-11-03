package rdap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func clientRepondant(status int, body string, seen *string) *Client {
	c := New()
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if seen != nil {
			*seen = r.URL.String()
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
	})
	return c
}

func TestExpiryLitLaDateDuRegistre(t *testing.T) {
	var seen string
	c := clientRepondant(http.StatusOK, `{"events":[{"eventAction":"registration","eventDate":"2010-01-01T00:00:00Z"},{"eventAction":"expiration","eventDate":"2031-03-05T10:00:00Z"}]}`, &seen)
	at, err := c.Expiry(context.Background(), "exemple.fr")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2031, 3, 5, 10, 0, 0, 0, time.UTC); !at.Equal(want) {
		t.Errorf("date : %v, attendu %v", at, want)
	}
	if seen != "https://rdap.org/domain/exemple.fr" {
		t.Errorf("adresse appelee : %s", seen)
	}
}

func TestExpiryDitQuandElleNeSaitPas(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		noDate bool
	}{
		{"sans date", http.StatusOK, `{"events":[{"eventAction":"registration","eventDate":"2010-01-01T00:00:00Z"}]}`, true},
		{"domaine inconnu", http.StatusNotFound, `{}`, false},
		{"reponse illisible", http.StatusOK, `pas du json`, false},
		{"date illisible", http.StatusOK, `{"events":[{"eventAction":"expiration","eventDate":"demain"}]}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := clientRepondant(c.status, c.body, nil).Expiry(context.Background(), "exemple.fr")
			if err == nil {
				t.Fatal("une erreur etait attendue")
			}
			if errors.Is(err, ErrNoExpiry) != c.noDate {
				t.Errorf("erreur : %v", err)
			}
		})
	}
}

func TestLesRedirectionsNeQuittentPasHTTPS(t *testing.T) {
	check := New().http.CheckRedirect
	req, _ := http.NewRequest(http.MethodGet, "http://interne.test/", nil)
	if err := check(req, nil); err == nil {
		t.Error("une redirection HTTP a ete acceptee")
	}
	req, _ = http.NewRequest(http.MethodGet, "https://rdap.registre.test/", nil)
	if err := check(req, nil); err != nil {
		t.Errorf("redirection HTTPS refusee : %v", err)
	}
}
