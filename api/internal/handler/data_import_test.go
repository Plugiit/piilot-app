package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

type fakeImports struct {
	uploads int
	tooBig  bool
}

func (f *fakeImports) Upload(_ context.Context, _ *uuid.UUID, entity, _ string, _ io.Reader) (usecase.ImportDetail, error) {
	f.uploads++

	if f.tooBig {
		return usecase.ImportDetail{}, domain.ErrValidation.WithDetails(map[string]any{"file": "Le fichier dépasse 10 Mio"})
	}

	return usecase.ImportDetail{ImportItem: usecase.ImportItem{Entity: entity}}, nil
}

func (f *fakeImports) Get(context.Context, uuid.UUID) (usecase.ImportDetail, error) {
	return usecase.ImportDetail{}, nil
}

func (f *fakeImports) List(context.Context, int) (usecase.ImportPage, error) {
	return usecase.ImportPage{}, nil
}

func (f *fakeImports) Rows(context.Context, uuid.UUID, string, int) (usecase.ImportRowPage, error) {
	return usecase.ImportRowPage{}, nil
}

func (f *fakeImports) Run(context.Context, uuid.UUID, usecase.RunImportInput) (usecase.ImportItem, error) {
	return usecase.ImportItem{Status: "queued"}, nil
}

func (f *fakeImports) Delete(context.Context, uuid.UUID) error { return nil }

type fakeExports struct{}

func (fakeExports) Write(_ context.Context, _ string, out io.Writer) error {
	_, err := io.WriteString(out, "a;b\r\n")

	return err
}

func dataImportApp(imports *fakeImports) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(discardLogger())})
	h := NewDataImport(imports, fakeExports{})

	app.Post("/imports", h.Upload)
	app.Post("/imports/:id/run", h.Run)
	app.Get("/exports/:entity", h.Export)

	return app
}

func multipartUpload(t *testing.T, fields map[string]string, file string) *http.Request {
	t.Helper()

	var body bytes.Buffer

	w := multipart.NewWriter(&body)

	for k, v := range fields {
		_ = w.WriteField(k, v)
	}

	if file != "" {
		part, _ := w.CreateFormFile("file", "clients.csv")
		_, _ = part.Write([]byte(file))
	}

	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/imports", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())

	return req
}

func TestDataImportUploadValide(t *testing.T) {
	imports := &fakeImports{}
	app := dataImportApp(imports)

	cases := []struct {
		name   string
		req    *http.Request
		status int
	}{
		{"entite valide", multipartUpload(t, map[string]string{"entity": "clients"}, "Nom\na\n"), http.StatusCreated},
		{"entite inconnue", multipartUpload(t, map[string]string{"entity": "factures"}, "Nom\na\n"), http.StatusUnprocessableEntity},
		{"entite absente", multipartUpload(t, nil, "Nom\na\n"), http.StatusUnprocessableEntity},
		{"fichier absent", multipartUpload(t, map[string]string{"entity": "clients"}, ""), http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		res, err := app.Test(tc.req)
		if err != nil {
			t.Fatalf("%s : %v", tc.name, err)
		}

		_ = res.Body.Close()

		if res.StatusCode != tc.status {
			t.Errorf("%s : statut %d, attendu %d", tc.name, res.StatusCode, tc.status)
		}
	}

	// Les refus ne doivent pas atteindre le service.
	if imports.uploads != 1 {
		t.Errorf("%d appels au service, attendu 1", imports.uploads)
	}
}

func TestDataImportUploadTropGros(t *testing.T) {
	app := dataImportApp(&fakeImports{tooBig: true})

	res, err := app.Test(multipartUpload(t, map[string]string{"entity": "clients"}, "Nom\na\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(body), "file") {
		t.Fatalf("statut %d, corps %s : attendu 422 avec le champ file", res.StatusCode, body)
	}
}

func TestDataImportRunEtExport(t *testing.T) {
	app := dataImportApp(&fakeImports{})

	run := httptest.NewRequest(http.MethodPost, "/imports/"+uuid.NewString()+"/run", strings.NewReader(`{"mapping":{"name":"Nom"},"dry_run":true}`))
	run.Header.Set("Content-Type", "application/json")

	res, err := app.Test(run)
	if err != nil {
		t.Fatal(err)
	}

	_ = res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		t.Errorf("run : statut %d, attendu 202", res.StatusCode)
	}

	bad := httptest.NewRequest(http.MethodPost, "/imports/pas-un-uuid/run", strings.NewReader(`{}`))
	bad.Header.Set("Content-Type", "application/json")

	if res, err = app.Test(bad); err != nil {
		t.Fatal(err)
	}

	_ = res.Body.Close()

	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("run avec un identifiant invalide : statut %d, attendu 422", res.StatusCode)
	}

	if res, err = app.Test(httptest.NewRequest(http.MethodGet, "/exports/clients", nil)); err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK ||
		!strings.HasPrefix(res.Header.Get("Content-Type"), "text/csv") ||
		!strings.HasPrefix(res.Header.Get("Content-Disposition"), `attachment; filename="piilot-clients_`) {
		t.Errorf("export : statut %d, en-tetes %v", res.StatusCode, res.Header)
	}

	if res, err = app.Test(httptest.NewRequest(http.MethodGet, "/exports/..%2Fetc", nil)); err != nil {
		t.Fatal(err)
	}

	_ = res.Body.Close()

	if res.StatusCode == http.StatusOK {
		t.Error("une entite inventee ne doit pas etre exportee")
	}
}
