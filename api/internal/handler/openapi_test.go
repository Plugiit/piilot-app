package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/plugiit/piilot-app/api/openapi"
)

// C'est ce test qui empeche la spec de deriver.
//
// Le contrat HTTP est ecrit a la main : rien ne garantit spontanement qu'il
// decrive les routes reellement montees. Sans ce garde-fou, une route ajoutee
// sans sa description laisserait le front generer des types incomplets — et le
// front compilerait quand meme, ce qui rend la panne invisible jusqu'au moment
// ou un appel echoue en production.
//
// La verification porte sur les chemins et les methodes, pas sur les schemas :
// elle attrape l'oubli, pas la description approximative.
func TestSpecOpenAPICouvreExactementLesRoutesMontees(t *testing.T) {
	app := testApp(t)

	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(openapi.Spec, &spec); err != nil {
		t.Fatalf("openapi/openapi.json illisible : %v", err)
	}

	decrites := map[string]bool{}
	for path, operations := range spec.Paths {
		for method := range operations {
			// Un chemin OpenAPI ne porte pas que des operations : « parameters »
			// y declare ce que toutes partagent, et « summary » les decrit. Les
			// prendre pour des verbes ferait echouer le test sur des cles
			// parfaitement valides.
			if !estMethodeHTTP(method) {
				continue
			}
			decrites[strings.ToUpper(method)+" "+path] = true
		}
	}

	montees := map[string]bool{}
	for _, route := range app.GetRoutes(true) {
		// HEAD et OPTIONS sont derives automatiquement par Fiber : les decrire
		// n'apporterait rien et alourdirait la spec.
		if route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		montees[route.Method+" "+enNotationOpenAPI(route.Path)] = true
	}

	for _, route := range manquantes(montees, decrites) {
		t.Errorf("route montee mais absente de openapi/openapi.json : %s", route)
	}

	for _, route := range manquantes(decrites, montees) {
		t.Errorf("openapi/openapi.json decrit une route qui n'est pas montee : %s", route)
	}
}

// L'endpoint doit servir la spec telle qu'embarquee : c'est elle que le front
// consomme pour generer ses types.
func TestSpecOpenAPIEstServieEnJSON(t *testing.T) {
	app := testApp(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("statut = %d, attendu %d", res.StatusCode, http.StatusOK)
	}

	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, attendu application/json", ct)
	}

	var document map[string]any
	if err := json.NewDecoder(res.Body).Decode(&document); err != nil {
		t.Fatalf("la spec servie n'est pas du JSON valide : %v", err)
	}

	if _, ok := document["openapi"]; !ok {
		t.Error("la spec servie ne porte pas de champ openapi")
	}
}

// estMethodeHTTP dit si la cle d'un chemin OpenAPI est une operation.
func estMethodeHTTP(key string) bool {
	switch strings.ToUpper(key) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions, "TRACE":
		return true
	default:
		return false
	}
}

// enNotationOpenAPI traduit les parametres de chemin de Fiber vers OpenAPI.
//
// Fiber ecrit « /projects/:id », OpenAPI « /projects/{id} ». Comparer les deux
// notations telles quelles ferait echouer le test sur chaque route parametree,
// et la seule facon de le faire passer serait d'ecrire une spec qui n'en est
// plus une — c'est donc au test de rapprocher les deux ecritures.
func enNotationOpenAPI(path string) string {
	segments := strings.Split(path, "/")

	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[i] = "{" + strings.TrimPrefix(segment, ":") + "}"
		}
	}

	return strings.Join(segments, "/")
}

// manquantes retourne les cles de a qui n'existent pas dans b, triees pour que
// l'echec du test soit lisible et stable d'une execution a l'autre.
func manquantes(a, b map[string]bool) []string {
	var out []string
	for key := range a {
		if !b[key] {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}
