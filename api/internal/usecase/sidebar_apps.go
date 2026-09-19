package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/repository/db"
	"github.com/plugiit/plugiit-api-go/internal/storage"
)

// SidebarApp est une application jointe depuis le rail.
type SidebarApp struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	URL  string    `json:"url"`
	// Adresse du logo depose, nulle tant qu'il n'y en a pas : le rail affiche
	// alors une pastille a l'initiale.
	LogoURL *string `json:"logo_url"`
	// Vrai quand le logo a ete recupere depuis l'adresse du site plutot que
	// depose : l'ecran le dit, et une nouvelle tentative peut l'ecraser.
	LogoIsFavicon bool      `json:"logo_is_favicon"`
	Color         string    `json:"color"`
	Position      int32     `json:"position"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// sidebarLogoPrefix est le chemin sous lequel les logos se relisent.
const sidebarLogoPrefix = "/api/v1/admin/sidebar-apps/logos/"

// sidebarLogoTypes borne les formats acceptes pour un logo.
//
// Le SVG y figure, contrairement aux photos de profil : un logo d'outil est
// dessine au trait, et l'imposer en bitmap le rendrait flou dans un rail. Il
// n'est servi qu'avec `nosniff` et une politique qui coupe tout script — dans
// une balise `img`, un navigateur n'execute de toute facon rien.
var sidebarLogoTypes = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/webp":    true,
	"image/gif":     true,
	"image/svg+xml": true,
}

// maxSidebarLogo borne le depot d'un logo. Large pour une icone, etroit pour
// tout le reste : au-dela, ce n'est plus un logo.
const maxSidebarLogo int64 = 512 << 10

// SidebarAppService sert le rail et son ecran de reglages.
type SidebarAppService struct {
	q     *db.Queries
	files storage.Store
}

func NewSidebarAppService(pool *pgxpool.Pool, files storage.Store) *SidebarAppService {
	return &SidebarAppService{q: db.New(pool), files: files}
}

// SidebarAppInput est ce que le formulaire envoie.
type SidebarAppInput struct {
	Name     string
	URL      string
	Color    string
	Position *int32
}

// clean valide et normalise ce que le formulaire envoie.
//
// L'adresse est verifiee ici et pas seulement a l'ecran : une entree du rail
// qui ne mene nulle part est un lien mort qu'on clique une fois par jour.
func (in SidebarAppInput) clean() (SidebarAppInput, error) {
	out := SidebarAppInput{
		Name:     strings.TrimSpace(in.Name),
		URL:      strings.TrimSpace(in.URL),
		Color:    strings.TrimSpace(in.Color),
		Position: in.Position,
	}

	if out.Name == "" {
		return out, domain.ErrValidation.WithDetails(map[string]any{
			"name": "Le nom est requis",
		})
	}

	parsed, err := url.Parse(out.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return out, domain.ErrValidation.WithDetails(map[string]any{
			"url": "Une adresse commençant par http:// ou https:// est attendue",
		})
	}

	if out.Color == "" {
		out.Color = defaultServiceColor
	}

	return out, nil
}

// logoURLOf compose l'adresse de relecture d'un logo.
func logoURLOf(key *string) *string {
	if key == nil || *key == "" {
		return nil
	}

	address := sidebarLogoPrefix + *key

	return &address
}

// List renvoie les apps du rail, dans leur ordre.
func (s *SidebarAppService) List(ctx context.Context) ([]SidebarApp, error) {
	rows, err := s.q.ListSidebarApps(ctx)
	if err != nil {
		return nil, fmt.Errorf("liste des apps : %w", err)
	}

	items := make([]SidebarApp, 0, len(rows))
	for _, row := range rows {
		items = append(items, SidebarApp{
			ID:            row.ID,
			Name:          row.Name,
			URL:           row.Url,
			LogoURL:       logoURLOf(row.LogoKey),
			LogoIsFavicon: row.LogoIsFavicon,
			Color:         row.Color,
			Position:      row.Position,
			UpdatedAt:     row.UpdatedAt,
		})
	}

	return items, nil
}

// Create ajoute une app au rail.
func (s *SidebarAppService) Create(ctx context.Context, in SidebarAppInput) (SidebarApp, error) {
	clean, err := in.clean()
	if err != nil {
		return SidebarApp{}, err
	}

	row, err := s.q.CreateSidebarApp(ctx, db.CreateSidebarAppParams{
		Name:  clean.Name,
		Url:   clean.URL,
		Color: clean.Color,
	})
	if err != nil {
		return SidebarApp{}, fmt.Errorf("creation de l'app : %w", err)
	}

	return SidebarApp{
		ID:            row.ID,
		Name:          row.Name,
		URL:           row.Url,
		LogoURL:       logoURLOf(row.LogoKey),
		LogoIsFavicon: row.LogoIsFavicon,
		Color:         row.Color,
		Position:      row.Position,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// Update modifie une app.
func (s *SidebarAppService) Update(
	ctx context.Context,
	id uuid.UUID,
	in SidebarAppInput,
) (SidebarApp, error) {
	clean, err := in.clean()
	if err != nil {
		return SidebarApp{}, err
	}

	current, err := s.get(ctx, id)
	if err != nil {
		return SidebarApp{}, err
	}

	// Le rang n'est pas toujours du voyage : le formulaire des champs ne le
	// touche pas, seule la reorganisation le deplace.
	position := current.Position
	if clean.Position != nil {
		position = *clean.Position
	}

	row, err := s.q.UpdateSidebarApp(ctx, db.UpdateSidebarAppParams{
		ID:       id,
		Name:     clean.Name,
		Url:      clean.URL,
		Color:    clean.Color,
		Position: position,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SidebarApp{}, domain.ErrNotFound
		}

		return SidebarApp{}, fmt.Errorf("mise a jour de l'app : %w", err)
	}

	return SidebarApp{
		ID:            row.ID,
		Name:          row.Name,
		URL:           row.Url,
		LogoURL:       logoURLOf(row.LogoKey),
		LogoIsFavicon: row.LogoIsFavicon,
		Color:         row.Color,
		Position:      row.Position,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// Delete retire une app du rail.
//
// Le fichier du logo part avec elle : plus rien ne le designe, et le garder
// ferait grossir le disque d'une image morte a chaque suppression.
func (s *SidebarAppService) Delete(ctx context.Context, id uuid.UUID) error {
	key, err := s.q.DeleteSidebarApp(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}

		return fmt.Errorf("suppression de l'app : %w", err)
	}

	if key != nil && *key != "" {
		_ = s.files.Remove(*key)
	}

	return nil
}

// SetLogo range le fichier envoye et le rattache a l'app.
func (s *SidebarAppService) SetLogo(
	ctx context.Context,
	id uuid.UUID,
	contentType string,
	content io.Reader,
) (SidebarApp, error) {
	// Le type annonce peut porter un parametre — « image/svg+xml; charset=utf-8 »
	// — que le navigateur ajoute de lui-meme.
	if index := strings.IndexByte(contentType, ';'); index >= 0 {
		contentType = contentType[:index]
	}

	if !sidebarLogoTypes[strings.TrimSpace(contentType)] {
		return SidebarApp{}, domain.ErrValidation.WithDetails(map[string]any{
			"file": "Format accepté : SVG, PNG, JPEG, WEBP ou GIF",
		})
	}

	previous, err := s.get(ctx, id)
	if err != nil {
		return SidebarApp{}, err
	}

	key, _, err := s.files.Save(content, maxSidebarLogo)
	if err != nil {
		if errors.Is(err, storage.ErrTooLarge) {
			return SidebarApp{}, domain.ErrValidation.WithDetails(map[string]any{
				"file": "Le logo dépasse 512 Ko",
			})
		}

		return SidebarApp{}, fmt.Errorf("enregistrement du logo : %w", err)
	}

	row, err := s.q.SetSidebarAppLogo(ctx, db.SetSidebarAppLogoParams{
		ID: id, LogoKey: &key, IsFavicon: false,
	})
	if err != nil {
		// Le fichier vient d'etre ecrit et ne sera jamais designe : l'effacer
		// evite de laisser un orphelin derriere une erreur.
		_ = s.files.Remove(key)

		if errors.Is(err, pgx.ErrNoRows) {
			return SidebarApp{}, domain.ErrNotFound
		}

		return SidebarApp{}, fmt.Errorf("rattachement du logo : %w", err)
	}

	// L'ancien logo n'est plus designe : il part apres coup, pour que l'echec
	// d'un effacement ne fasse pas echouer un depot reussi.
	if previous.LogoKey != nil && *previous.LogoKey != "" {
		_ = s.files.Remove(*previous.LogoKey)
	}

	return SidebarApp{
		ID:            row.ID,
		Name:          row.Name,
		URL:           row.Url,
		LogoURL:       logoURLOf(row.LogoKey),
		LogoIsFavicon: row.LogoIsFavicon,
		Color:         row.Color,
		Position:      row.Position,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// RemoveLogo detache le logo et efface son fichier.
func (s *SidebarAppService) RemoveLogo(ctx context.Context, id uuid.UUID) (SidebarApp, error) {
	previous, err := s.get(ctx, id)
	if err != nil {
		return SidebarApp{}, err
	}

	row, err := s.q.SetSidebarAppLogo(ctx, db.SetSidebarAppLogoParams{
		ID: id, LogoKey: nil, IsFavicon: false,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SidebarApp{}, domain.ErrNotFound
		}

		return SidebarApp{}, fmt.Errorf("retrait du logo : %w", err)
	}

	if previous.LogoKey != nil && *previous.LogoKey != "" {
		_ = s.files.Remove(*previous.LogoKey)
	}

	return SidebarApp{
		ID:            row.ID,
		Name:          row.Name,
		URL:           row.Url,
		LogoURL:       logoURLOf(row.LogoKey),
		LogoIsFavicon: row.LogoIsFavicon,
		Color:         row.Color,
		Position:      row.Position,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// RefetchLogo repasse le logo en automatique.
//
// Le logo courant part — fichier compris — et la recuperation rouvre. Le job la
// reprendra a son prochain passage : l'app revient donc sans logo, et le sien
// apparait quelques instants plus tard. Aller le chercher ici ferait attendre
// l'ecran sur un serveur qu'on ne maitrise pas.
func (s *SidebarAppService) RefetchLogo(ctx context.Context, id uuid.UUID) (SidebarApp, error) {
	previous, err := s.get(ctx, id)
	if err != nil {
		return SidebarApp{}, err
	}

	row, err := s.q.ResetSidebarAppFavicon(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SidebarApp{}, domain.ErrNotFound
		}

		return SidebarApp{}, fmt.Errorf("relance de la recuperation : %w", err)
	}

	if previous.LogoKey != nil && *previous.LogoKey != "" {
		_ = s.files.Remove(*previous.LogoKey)
	}

	return SidebarApp{
		ID:            row.ID,
		Name:          row.Name,
		URL:           row.Url,
		LogoURL:       logoURLOf(row.LogoKey),
		LogoIsFavicon: row.LogoIsFavicon,
		Color:         row.Color,
		Position:      row.Position,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// OpenLogo ouvre le fichier d'un logo.
//
// La cle doit designer le logo d'une app vivante : le magasin est commun aux
// avatars et aux pieces jointes, et servir n'importe quelle cle par cette porte
// laisserait lire un document sans verifier les droits qui le protegent.
func (s *SidebarAppService) OpenLogo(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, domain.ErrNotFound
	}

	used, err := s.q.SidebarAppLogoKeyInUse(ctx, &key)
	if err != nil {
		return nil, fmt.Errorf("verification du logo : %w", err)
	}
	if !used {
		return nil, domain.ErrNotFound
	}

	content, err := s.files.Open(key)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	return content, nil
}

// get lit une app, ou rend 404.
func (s *SidebarAppService) get(ctx context.Context, id uuid.UUID) (db.GetSidebarAppRow, error) {
	row, err := s.q.GetSidebarApp(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GetSidebarAppRow{}, domain.ErrNotFound
		}

		return db.GetSidebarAppRow{}, fmt.Errorf("lecture de l'app : %w", err)
	}

	return row, nil
}
