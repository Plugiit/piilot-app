package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// projectLogoPrefix est le chemin sous lequel les logos de projet se relisent.
//
// Sous /auth et non /admin : un client du portail voit le logo de ses projets
// comme l'equipe voit celui des siens, et la route ne demande qu'une session —
// comme les photos de profil. Les cles sont tirees au sort et ne designent que
// des logos : il n'y a rien d'autre a y trouver.
const projectLogoPrefix = "/api/v1/auth/project-logos/"

// projectLogoTypes borne les formats acceptes. Le SVG y figure, comme pour les
// logos d'apps : un logo est dessine au trait, et il n'est servi qu'avec
// nosniff et une politique qui coupe tout script.
var projectLogoTypes = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/webp":    true,
	"image/gif":     true,
	"image/svg+xml": true,
}

// maxProjectLogo borne le depot d'un logo. Au-dela, ce n'est plus un logo.
const maxProjectLogo int64 = 512 << 10

func projectLogoURL(key *string) *string {
	if key == nil || *key == "" {
		return nil
	}

	address := projectLogoPrefix + *key

	return &address
}

// SetLogoFile range le logo envoye et le rattache au projet. L'ancien est
// efface apres coup : plus rien ne le designe une fois la nouvelle cle ecrite.
func (s *ProjectService) SetLogoFile(
	ctx context.Context,
	id, viewer uuid.UUID,
	contentType string,
	content io.Reader,
) (ProjectDetail, error) {
	// Le type annonce peut porter un parametre — « image/svg+xml; charset=utf-8 »
	// — que le navigateur ajoute de lui-meme.
	if index := strings.IndexByte(contentType, ';'); index >= 0 {
		contentType = contentType[:index]
	}
	if !projectLogoTypes[strings.TrimSpace(contentType)] {
		return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"file": "Format accepté : SVG, PNG, JPEG, WEBP ou GIF",
		})
	}

	previous, err := s.q.GetProject(ctx, db.GetProjectParams{ID: id, ViewerID: viewer})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectDetail{}, domain.ErrNotFound
		}
		return ProjectDetail{}, fmt.Errorf("lecture du projet : %w", err)
	}

	key, _, err := s.files.Save(content, maxProjectLogo)
	if err != nil {
		if errors.Is(err, storage.ErrTooLarge) {
			return ProjectDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"file": "Le logo dépasse 512 Ko",
			})
		}
		return ProjectDetail{}, fmt.Errorf("ecriture du logo : %w", err)
	}

	if _, err := s.q.SetProjectLogo(ctx, db.SetProjectLogoParams{ID: id, LogoKey: &key}); err != nil {
		// La ligne n'a pas ete ecrite : le fichier ne doit pas rester seul.
		_ = s.files.Remove(key)
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectDetail{}, domain.ErrNotFound
		}
		return ProjectDetail{}, fmt.Errorf("rattachement du logo : %w", err)
	}

	if previous.LogoKey != nil && *previous.LogoKey != "" {
		_ = s.files.Remove(*previous.LogoKey)
	}

	return s.Get(ctx, id, viewer)
}

// RemoveLogo retire le logo d'un projet et efface son fichier.
func (s *ProjectService) RemoveLogo(ctx context.Context, id, viewer uuid.UUID) (ProjectDetail, error) {
	previous, err := s.q.GetProject(ctx, db.GetProjectParams{ID: id, ViewerID: viewer})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectDetail{}, domain.ErrNotFound
		}
		return ProjectDetail{}, fmt.Errorf("lecture du projet : %w", err)
	}

	if _, err := s.q.SetProjectLogo(ctx, db.SetProjectLogoParams{ID: id, LogoKey: nil}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectDetail{}, domain.ErrNotFound
		}
		return ProjectDetail{}, fmt.Errorf("retrait du logo : %w", err)
	}

	if previous.LogoKey != nil && *previous.LogoKey != "" {
		_ = s.files.Remove(*previous.LogoKey)
	}

	return s.Get(ctx, id, viewer)
}

// OpenLogo ouvre le fichier d'un logo. Seules les cles qui sont le logo d'un
// projet passent : le magasin est commun aux pieces jointes.
func (s *ProjectService) OpenLogo(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, domain.ErrNotFound
	}

	used, err := s.q.ProjectLogoKeyInUse(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("verification du logo : %w", err)
	}
	if !used {
		return nil, domain.ErrNotFound
	}

	content, err := s.files.Open(key)
	if err != nil {
		// Cle connue de la base mais fichier absent : un disque restaure sans
		// son dossier. Pour l'ecran, le logo n'existe pas.
		return nil, domain.ErrNotFound
	}

	return content, nil
}
