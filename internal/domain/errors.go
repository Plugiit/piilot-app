// Package domain contient les entites et les regles metier, sans dependance
// au framework HTTP ni a la base de donnees.
package domain

import (
	"errors"
	"fmt"
	"net/http"
)

// Error est l'erreur metier transportee jusqu'au client. Le Code est stable et
// destine au front (switch sur le code, jamais sur le message) ; le Message est
// affichable a l'utilisateur.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
	cause   error
}

// Error implemente l'interface error.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.cause)
	}
	return e.Code
}

// Unwrap expose la cause pour errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.cause }

// WithCause attache l'erreur technique d'origine : elle part dans les logs,
// jamais dans la reponse HTTP.
func (e *Error) WithCause(err error) *Error {
	clone := *e
	clone.cause = err
	return &clone
}

// WithDetails attache des details structures (erreurs de validation par champ).
func (e *Error) WithDetails(details map[string]any) *Error {
	clone := *e
	clone.Details = details
	return &clone
}

// Erreurs metier reutilisables. Les handlers les retournent telles quelles :
// le middleware ErrorHandler les traduit en JSON.
var (
	ErrUnauthorized = &Error{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "Authentification requise"}
	ErrForbidden    = &Error{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: "Accès refusé"}
	ErrNotFound     = &Error{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: "Ressource introuvable"}
	ErrConflict     = &Error{Status: http.StatusConflict, Code: "CONFLICT", Message: "La ressource existe déjà"}
	ErrValidation   = &Error{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_FAILED", Message: "Données invalides"}
	ErrRateLimited  = &Error{Status: http.StatusTooManyRequests, Code: "RATE_LIMITED", Message: "Trop de requêtes"}
	ErrInternal     = &Error{Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR", Message: "Une erreur interne est survenue"}
)

// AsError extrait une *Error d'une chaine d'erreurs, ou nil.
func AsError(err error) (*Error, bool) {
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr, true
	}
	return nil, false
}
