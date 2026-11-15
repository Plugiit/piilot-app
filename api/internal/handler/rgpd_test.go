package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// erasureLister retient le filtre recu et ne fait que lister.
type erasureLister struct {
	RgpdErasureService
	calls  int
	filter usecase.RgpdErasureFilter
}

func (l *erasureLister) List(_ context.Context, f usecase.RgpdErasureFilter, _ int) (usecase.RgpdErasurePage, error) {
	l.calls++
	l.filter = f

	return usecase.RgpdErasurePage{Items: []usecase.RgpdErasureItem{}}, nil
}

func TestListErasuresValideLeFiltreDeSujet(t *testing.T) {
	id := uuid.NewString()

	cases := []struct {
		name   string
		query  string
		status int
		called bool
	}{
		{"filtre valide", "?status=scheduled&subject_type=user&subject_id=" + id, 200, true},
		{"sans filtre de sujet", "?status=scheduled", 200, true},
		{"type inconnu", "?subject_type=robot", 422, false},
		{"identifiant invalide", "?subject_type=user&subject_id=pas-un-uuid", 422, false},
		{"identifiant seul invalide", "?subject_id=123", 422, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &erasureLister{}
			app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(discardLogger())})
			app.Get("/erasures", NewRgpd(nil, svc).ListErasures)

			resp, err := app.Test(httptest.NewRequest("GET", "/erasures"+tc.query, nil))
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != tc.status {
				t.Errorf("statut %d, attendu %d", resp.StatusCode, tc.status)
			}

			if (svc.calls == 1) != tc.called {
				t.Errorf("service appele %d fois, attendu appele=%v", svc.calls, tc.called)
			}
		})
	}

	svc := &erasureLister{}
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(discardLogger())})
	app.Get("/erasures", NewRgpd(nil, svc).ListErasures)

	if _, err := app.Test(httptest.NewRequest("GET", "/erasures?subject_type=user&subject_id="+id, nil)); err != nil {
		t.Fatal(err)
	}

	if svc.filter.SubjectType != "user" || svc.filter.SubjectID == nil || svc.filter.SubjectID.String() != id {
		t.Errorf("filtre transmis : %+v", svc.filter)
	}
}
