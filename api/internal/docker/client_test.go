package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLeClientUtiliseLaVersionDApiDuMoteur(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version":
			_, _ = w.Write([]byte(`{"ApiVersion":"1.47"}`))
		default:
			asked = r.URL.Path
			_, _ = w.Write([]byte(`{"Id":"abc","Name":"/app"}`))
		}
	}))
	defer srv.Close()

	c, err := NewHTTP(srv.URL).InspectContainer(context.Background(), "abc")
	if err != nil || c.ID != "abc" {
		t.Fatalf("InspectContainer = %+v, %v", c, err)
	}
	if asked != "/v1.47/containers/abc/json" {
		t.Errorf("chemin = %s, attendu la version du moteur", asked)
	}
}
