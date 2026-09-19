package handler

import (
	"context"
	"io"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// testSecret sert a signer les jetons des tests. Assez long pour satisfaire le
// plancher de 32 octets impose par la configuration.
const testSecret = "secret-de-test-suffisamment-long-32"

// discardLogger evite de polluer la sortie des tests.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// stubPermissions repond sans base. Les tests de ce paquet verifient le
// montage des routes et le comportement des gardes, pas la politique de droits
// elle-meme — celle-ci vit en base et se teste contre une vraie Postgres.
type stubPermissions struct {
	granted bool
}

func (s stubPermissions) HasPermission(context.Context, string, string) (bool, error) {
	return s.granted, nil
}

// stubAuth remplace le service d'authentification. Le champ meCalled permet de
// verifier qu'un handler protege n'a PAS ete atteint quand la garde refuse.
type stubAuth struct {
	profile   usecase.Profile
	session   usecase.Session
	meCalled  bool
	loginCall bool
}

func (s *stubAuth) Login(context.Context, usecase.LoginInput) (usecase.Session, error) {
	s.loginCall = true
	return s.session, nil
}

func (s *stubAuth) Refresh(context.Context, string, string, *netip.Addr) (usecase.Session, error) {
	return s.session, nil
}

func (s *stubAuth) Logout(context.Context, string) error {
	return nil
}

func (s *stubAuth) Me(context.Context, uuid.UUID) (usecase.Profile, error) {
	s.meCalled = true
	return s.profile, nil
}

func (s *stubAuth) UpdateProfile(
	context.Context, uuid.UUID, usecase.UpdateProfileInput,
) (usecase.Profile, error) {
	return s.profile, nil
}

func (s *stubAuth) ChangePassword(context.Context, uuid.UUID, string, string) error {
	return nil
}

func (s *stubAuth) SetAvatarFile(
	context.Context, uuid.UUID, string, io.Reader,
) (usecase.Profile, error) {
	return s.profile, nil
}

func (s *stubAuth) RemoveAvatar(context.Context, uuid.UUID) (usecase.Profile, error) {
	return s.profile, nil
}

func (s *stubAuth) OpenAvatar(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

// stubLimiter remplace le compteur Redis. blocked simule un quota depasse ;
// resets compte les remises a zero, pour verifier qu'une connexion reussie
// libere bien les compteurs.
type stubLimiter struct {
	blocked bool
	resets  int
}

func (s *stubLimiter) Allow(context.Context, string, int, time.Duration) (bool, time.Duration, error) {
	return !s.blocked, 42 * time.Second, nil
}

func (s *stubLimiter) Reset(context.Context, string) error {
	s.resets++
	return nil
}
