package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

// SidebarAppService est le contrat dont le rail et son ecran ont besoin.
type SidebarAppService interface {
	List(ctx context.Context) ([]usecase.SidebarApp, error)
	Create(ctx context.Context, in usecase.SidebarAppInput) (usecase.SidebarApp, error)
	Update(ctx context.Context, id uuid.UUID, in usecase.SidebarAppInput) (usecase.SidebarApp, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetLogo(ctx context.Context, id uuid.UUID, contentType string, content io.Reader) (usecase.SidebarApp, error)
	RemoveLogo(ctx context.Context, id uuid.UUID) (usecase.SidebarApp, error)
	RefetchLogo(ctx context.Context, id uuid.UUID) (usecase.SidebarApp, error)
	OpenLogo(ctx context.Context, key string) (io.ReadCloser, error)
}

// SidebarApps porte les endpoints des apps du rail.
type SidebarApps struct {
	svc SidebarAppService
}

// NewSidebarApps construit le handler.
func NewSidebarApps(svc SidebarAppService) *SidebarApps {
	return &SidebarApps{svc: svc}
}

// sidebarAppBody est ce que le formulaire envoie.
type sidebarAppBody struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Color    string `json:"color"`
	Position *int32 `json:"position"`
}

func (b sidebarAppBody) input() usecase.SidebarAppInput {
	return usecase.SidebarAppInput{
		Name:     b.Name,
		URL:      b.URL,
		Color:    b.Color,
		Position: b.Position,
	}
}

// List sert le rail et l'ecran qui le regle.
func (h *SidebarApps) List(c fiber.Ctx) error {
	items, err := h.svc.List(c.Context())
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": items})
}

// Create ajoute une app au rail.
func (h *SidebarApps) Create(c fiber.Ctx) error {
	var body sidebarAppBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	app, err := h.svc.Create(c.Context(), body.input())
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(app)
}

// Update modifie une app.
func (h *SidebarApps) Update(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body sidebarAppBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	app, err := h.svc.Update(c.Context(), id, body.input())
	if err != nil {
		return err
	}

	return c.JSON(app)
}

// Delete retire une app du rail.
func (h *SidebarApps) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// UploadLogo depose le logo d'une app.
func (h *SidebarApps) UploadLogo(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	header, err := c.FormFile("file")
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{
			"file": "Aucun fichier reçu sous le champ « file »",
		})
	}

	content, err := header.Open()
	if err != nil {
		return fmt.Errorf("lecture du fichier envoye : %w", err)
	}
	defer func() { _ = content.Close() }()

	app, err := h.svc.SetLogo(c.Context(), id, header.Header.Get("Content-Type"), content)
	if err != nil {
		return err
	}

	return c.JSON(app)
}

// DeleteLogo retire le logo d'une app.
func (h *SidebarApps) DeleteLogo(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	app, err := h.svc.RemoveLogo(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(app)
}

// RefetchLogo repasse le logo en automatique.
//
// Repond tout de suite, sans le logo : la recuperation sort vers un serveur
// tiers et se fait en tache de fond. L'app revient donc sur sa pastille, et son
// logo arrive au passage suivant du job — dans la minute.
func (h *SidebarApps) RefetchLogo(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	app, err := h.svc.RefetchLogo(c.Context(), id)
	if err != nil {
		return err
	}

	// 202 et non 200 : la demande est enregistree, le resultat viendra apres.
	return c.Status(fiber.StatusAccepted).JSON(app)
}

// maxSidebarLogoBytes borne ce que la lecture d'un logo met en memoire. Aligne
// sur la limite posee au depot : un fichier plus gros n'a pas pu entrer par la.
const maxSidebarLogoBytes = 512 << 10

// Logo sert le fichier d'un logo.
//
// Le type est devine au contenu plutot que stocke, comme pour les photos de
// profil : se fier a l'entete annonce au depot reviendrait a laisser le
// deposant choisir comment le navigateur interprete le fichier.
//
// Le SVG est le cas que `DetectContentType` ne sait pas nommer — il y voit du
// texte ou du XML. Il est reconnu a sa racine, puis servi sous une politique
// qui coupe tout : dans une balise `img`, un navigateur n'execute deja rien,
// et l'adresse ouverte seule ne peut rien faire non plus.
func (h *SidebarApps) Logo(c fiber.Ctx) error {
	content, err := h.svc.OpenLogo(c.Context(), c.Params("key"))
	if err != nil {
		return err
	}
	defer func() { _ = content.Close() }()

	// Lu en entier plutot que diffuse : Fiber ecrit le corps apres le retour du
	// handler, et un flux ferme d'ici la ne rend rien.
	data, err := io.ReadAll(io.LimitReader(content, maxSidebarLogoBytes))
	if err != nil {
		return fmt.Errorf("lecture du logo : %w", err)
	}

	kind := http.DetectContentType(data)
	if !strings.HasPrefix(kind, "image/") {
		if !looksLikeSVG(data) {
			return domain.ErrNotFound
		}

		kind = "image/svg+xml"
	}

	c.Set(fiber.HeaderContentType, kind)
	// Le navigateur s'en tient au type annonce : sans cela, il pourrait deviner
	// autre chose dans le contenu et l'interpreter comme un document.
	c.Set("X-Content-Type-Options", "nosniff")
	// Ceinture et bretelles pour le SVG : meme ouverte seule, l'adresse ne
	// charge rien et n'execute rien.
	c.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	// Les cles sont tirees au sort et un nouveau logo en produit une autre :
	// l'adresse ne change jamais de contenu, elle peut se garder longtemps.
	c.Set(fiber.HeaderCacheControl, "private, max-age=604800, immutable")

	return c.Send(data)
}

// looksLikeSVG reconnait un SVG a sa racine, en sautant ce qui peut le preceder
// — declaration XML, doctype, commentaires, espaces.
func looksLikeSVG(data []byte) bool {
	head := strings.TrimSpace(string(data[:min(len(data), 1024)]))

	for strings.HasPrefix(head, "<?") || strings.HasPrefix(head, "<!") {
		end := strings.IndexByte(head, '>')
		if end < 0 {
			return false
		}

		head = strings.TrimSpace(head[end+1:])
	}

	if !strings.HasPrefix(head, "<svg") {
		return false
	}

	// Le nom de la balise doit s'arreter la : sans ce controle, « <svgx » —
	// qui n'est pas un SVG — passerait pour un.
	rest := head[len("<svg"):]
	if rest == "" {
		return false
	}

	return rest[0] == '>' || rest[0] == '/' || rest[0] == ' ' || rest[0] == '\n' ||
		rest[0] == '\t' || rest[0] == '\r'
}
