package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Service est une prestation du referentiel de l'agence.
type Service struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	// Teinte de la pastille, en notation hexadecimale.
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ServiceTag est un service porte par un projet ou une tache : de quoi peindre
// une pastille et la nommer, pas la fiche du referentiel.
//
// Les listes qui en portent sont vides quand rien n'est attache, et un service
// retire du referentiel en disparait sans que la ligne qui le portait en
// souffre : la jointure de chargement ecarte les supprimes.
type ServiceTag struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}

// ServicePage est une page du tableau.
type ServicePage struct {
	Items    []Service `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// ServiceService sert le referentiel des prestations.
type ServiceService struct {
	q *db.Queries
}

func NewServiceService(pool *pgxpool.Pool) *ServiceService {
	return &ServiceService{q: db.New(pool)}
}

// ServiceInput est ce que le formulaire envoie, a la creation comme a la
// modification : les deux montrent les memes champs.
type ServiceInput struct {
	Name        string
	Description string
	Color       string
}

// defaultServiceColor est la teinte d'un service qui n'en choisit pas : le gris
// des libelles secondaires, qui ne fait ressortir aucune ligne en particulier.
const defaultServiceColor = "#73757c"

// clean valide et normalise ce que le formulaire envoie.
func (in ServiceInput) clean() (ServiceInput, error) {
	out := ServiceInput{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Color:       strings.TrimSpace(in.Color),
	}

	if out.Name == "" {
		return out, domain.ErrValidation.WithDetails(map[string]any{
			"name": "Le nom est requis",
		})
	}

	if out.Color == "" {
		out.Color = defaultServiceColor
	}

	return out, nil
}

// List renvoie une page du referentiel.
func (s *ServiceService) List(
	ctx context.Context,
	search *string,
	page, pageSize int,
) (ServicePage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	rows, err := s.q.ListServices(ctx, db.ListServicesParams{
		Search:     search,
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return ServicePage{}, fmt.Errorf("liste des services : %w", err)
	}

	total, err := s.q.CountServices(ctx, search)
	if err != nil {
		return ServicePage{}, fmt.Errorf("compte des services : %w", err)
	}

	items := make([]Service, 0, len(rows))
	for _, row := range rows {
		items = append(items, Service{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Color:       row.Color,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}

	return ServicePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Create ajoute une prestation au referentiel.
func (s *ServiceService) Create(ctx context.Context, in ServiceInput) (Service, error) {
	clean, err := in.clean()
	if err != nil {
		return Service{}, err
	}

	row, err := s.q.CreateService(ctx, db.CreateServiceParams{
		Name:        clean.Name,
		Description: clean.Description,
		Color:       clean.Color,
	})
	if err != nil {
		// Deux services du meme nom se confondraient dans un menu deroulant :
		// l'index unique le refuse, et le nom fautif est nomme a l'appelant.
		if isUniqueViolation(err) {
			return Service{}, domain.ErrValidation.WithDetails(map[string]any{
				"name": "Un service porte déjà ce nom",
			})
		}

		return Service{}, fmt.Errorf("creation du service : %w", err)
	}

	return Service{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Color:       row.Color,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

// Update modifie une prestation.
func (s *ServiceService) Update(
	ctx context.Context,
	id uuid.UUID,
	in ServiceInput,
) (Service, error) {
	clean, err := in.clean()
	if err != nil {
		return Service{}, err
	}

	row, err := s.q.UpdateService(ctx, db.UpdateServiceParams{
		ID:          id,
		Name:        clean.Name,
		Description: clean.Description,
		Color:       clean.Color,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Service{}, domain.ErrNotFound
		}
		if isUniqueViolation(err) {
			return Service{}, domain.ErrValidation.WithDetails(map[string]any{
				"name": "Un service porte déjà ce nom",
			})
		}

		return Service{}, fmt.Errorf("mise a jour du service : %w", err)
	}

	return Service{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Color:       row.Color,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

// Delete retire une prestation du referentiel.
//
// Suppression douce : un service retire a pu etiqueter des donnees passees, et
// l'effacer vraiment les rendrait illisibles.
func (s *ServiceService) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := s.q.DeleteService(ctx, id)
	if err != nil {
		return fmt.Errorf("suppression du service : %w", err)
	}

	// Zero ligne : l'identifiant ne designe rien de vivant.
	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}
