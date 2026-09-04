// Package openapi porte le contrat HTTP publie par le serveur.
//
// La specification est ecrite a la main et embarquee dans le binaire : elle est
// la source de verite du contrat, et le front en derive ses types
// (npm run api:types). Un test verifie qu'elle decrit exactement les routes
// montees, pour qu'elle ne puisse pas deriver en silence.
package openapi

import _ "embed"

// Spec est le document OpenAPI servi sur /openapi.json.
//
//go:embed openapi.json
var Spec []byte
