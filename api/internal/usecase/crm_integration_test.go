//go:build integration

// Tests d'integration du module CRM.
//
// Isoles derriere le tag `integration` comme ceux de l'authentification : `go
// test ./...`, lance par `make check` et par le Dockerfile sans base, reste
// vert. Ils ne sont donc jamais ignores en silence.
//
// Ce qu'ils figent, ce sont les invariants que le schema porte et que le code
// seul ne montre pas : la cle etrangere composite entre client et contact
// principal, les compteurs tenus par declencheur, et la prise de possession
// d'un contact libre qui ne peut jouer qu'une fois.
package usecase_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// newCRM ouvre la base et construit les deux services du module. Echoue
// bruyamment si TEST_DATABASE_URL manque, pour la meme raison que newService.
func newCRM(t *testing.T) (*usecase.ClientService, *usecase.ContactService, *pgxpool.Pool) {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL est obligatoire : ces tests exigent une vraie base (make up, puis make test-integration)")
	}

	pool, err := repository.NewPool(context.Background(), url, repository.DefaultPoolConfig())
	if err != nil {
		t.Fatalf("ouverture du pool : %v", err)
	}
	t.Cleanup(pool.Close)

	return usecase.NewClientService(pool), usecase.NewContactService(pool), pool
}

// uniqueName rend un nom que rien d'autre ne porte : l'unicite du nom de
// client est justement ce que plusieurs tests eprouvent, et deux executions
// successives ne doivent pas se gener.
func uniqueName(prefix string) string {
	return prefix + "-" + uuid.NewString()[:8]
}

// dropClient efface pour de bon un client et ses contacts.
//
// La designation est retiree avant tout : la cle etrangere composite relie les
// deux tables dans les deux sens, et supprimer sans defaire ce lien echouerait.
func dropClient(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()

		if _, err := pool.Exec(ctx, `UPDATE clients SET primary_contact_id = NULL WHERE id = $1`, id); err != nil {
			t.Errorf("nettoyage de la designation : %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM contacts WHERE client_id = $1`, id); err != nil {
			t.Errorf("nettoyage des contacts : %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, id); err != nil {
			t.Errorf("nettoyage du client : %v", err)
		}
	})
}

// pro decrit un professionnel au SIRET valide et unique : la creation l'exige,
// et refuse deux clients sur le meme etablissement.
func pro(name string) usecase.CreateClientInput {
	return usecase.CreateClientInput{
		Name: name,
		Identity: usecase.ClientIdentity{
			Kind:      usecase.ClientProfessionnel,
			LegalName: name,
			Siret:     randomSiret(),
		},
	}
}

func adopting(in usecase.CreateClientInput, contactID uuid.UUID) usecase.CreateClientInput {
	in.ContactID = &contactID
	return in
}

// randomSiret tire treize chiffres et calcule la cle de Luhn du quatorzieme.
func randomSiret() string {
	digits := make([]int, 14)
	for i := range 13 {
		digits[i] = rand.IntN(10)
	}

	for check := range 10 {
		digits[13] = check
		sum := 0
		for i, d := range digits {
			if (14-i)%2 == 0 {
				d *= 2
				if d > 9 {
					d -= 9
				}
			}
			sum += d
		}
		if sum%10 == 0 {
			break
		}
	}

	var b strings.Builder
	for _, d := range digits {
		b.WriteByte(byte('0' + d))
	}

	return b.String()
}

func dropContact(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()

		if _, err := pool.Exec(ctx, `UPDATE clients SET primary_contact_id = NULL WHERE primary_contact_id = $1`, id); err != nil {
			t.Errorf("nettoyage de la designation : %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM contacts WHERE id = $1`, id); err != nil {
			t.Errorf("nettoyage du contact : %v", err)
		}
	})
}

func TestCreateClientRefuseUnNomDejaPris(t *testing.T) {
	clients, _, pool := newCRM(t)
	ctx := context.Background()

	name := uniqueName("Client")

	first, err := clients.Create(ctx, pro(name))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, first.ID)

	// Casse differente : l'index unique porte sur lower(name), le doublon doit
	// etre reconnu malgre la majuscule.
	_, err = clients.Create(ctx, pro(strings.ToUpper(name)))
	if err == nil {
		t.Fatal("un second client du meme nom a ete cree")
	}

	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Status != http.StatusConflict {
		t.Fatalf("erreur attendue en conflit, recue : %v", err)
	}
}

func TestContactLibreEstAdopteParUnSeulClient(t *testing.T) {
	clients, contacts, pool := newCRM(t)
	ctx := context.Background()

	contact, err := contacts.Create(ctx, usecase.CreateContactInput{Firstname: "Zoe", Lastname: "Libre"})
	if err != nil {
		t.Fatalf("creation du contact : %v", err)
	}
	dropContact(t, pool, contact.ID)

	if contact.ClientID != nil {
		t.Fatalf("un contact cree sans client devrait etre libre, client_id = %v", contact.ClientID)
	}

	first, err := clients.Create(ctx, adopting(pro(uniqueName("Adoptant")), contact.ID))
	if err != nil {
		t.Fatalf("creation du client adoptant : %v", err)
	}
	dropClient(t, pool, first.ID)

	if first.PrimaryContact == nil || first.PrimaryContact.ID != contact.ID {
		t.Fatalf("le contact adopte devrait etre principal, recu : %+v", first.PrimaryContact)
	}

	// Le meme contact ne peut pas etre repris : la prise de possession s'ecrit
	// `WHERE client_id IS NULL` et ne joue donc qu'une fois.
	_, err = clients.Create(ctx, adopting(pro(uniqueName("Rival")), contact.ID))
	if err == nil {
		t.Fatal("un second client a pris un contact deja rattache")
	}
}

func TestContactPrincipalDoitAppartenirAuClient(t *testing.T) {
	clients, contacts, pool := newCRM(t)
	ctx := context.Background()

	owner, err := clients.Create(ctx, pro(uniqueName("Proprietaire")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, owner.ID)

	other, err := clients.Create(ctx, pro(uniqueName("Autre")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, other.ID)

	contact, err := contacts.Create(ctx, usecase.CreateContactInput{
		ClientID:  &owner.ID,
		Firstname: "Paul",
		Lastname:  "Interne",
	})
	if err != nil {
		t.Fatalf("creation du contact : %v", err)
	}

	// Le premier contact d'un client devient principal sans qu'on le demande.
	if !contact.IsPrimary {
		t.Error("le premier contact d'un client devrait etre principal")
	}

	if err := clients.SetPrimaryContact(ctx, other.ID, &contact.ID); err == nil {
		t.Fatal("un client a pu designer le contact d'un autre")
	}
}

func TestSupprimerUnContactRetireSaDesignation(t *testing.T) {
	clients, contacts, pool := newCRM(t)
	ctx := context.Background()

	client, err := clients.Create(ctx, pro(uniqueName("Designation")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	contact, err := contacts.Create(ctx, usecase.CreateContactInput{
		ClientID:  &client.ID,
		Firstname: "Ana",
		Lastname:  "Partante",
	})
	if err != nil {
		t.Fatalf("creation du contact : %v", err)
	}

	if err := contacts.Delete(ctx, contact.ID); err != nil {
		t.Fatalf("suppression du contact : %v", err)
	}

	after, err := clients.Get(ctx, client.ID)
	if err != nil {
		t.Fatalf("relecture du client : %v", err)
	}

	if after.PrimaryContact != nil {
		t.Fatalf("la designation aurait du etre retiree, recu : %+v", after.PrimaryContact)
	}
	if after.ContactsCount != 0 {
		t.Errorf("le contact supprime est encore compte : %d", after.ContactsCount)
	}
}

func TestRenommerUnClientAvecSonPropreNom(t *testing.T) {
	clients, _, pool := newCRM(t)
	ctx := context.Background()

	name := uniqueName("Stable")

	client, err := clients.Create(ctx, pro(name))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	// Renommer sans rien changer ne doit pas buter sur son propre
	// enregistrement.
	if _, err := clients.Update(ctx, client.ID, usecase.UpdateClientInput{
		Name:   name,
		Status: "lead",
	}); err != nil {
		t.Fatalf("renommage a l'identique refuse : %v", err)
	}
}

func TestCompteurDeComptesPortail(t *testing.T) {
	clients, _, pool := newCRM(t)
	ctx := context.Background()

	client, err := clients.Create(ctx, pro(uniqueName("Portail")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	// Le compteur est tenu par declencheur : on ecrit le compte directement en
	// base, puisque aucun service ne sait encore rattacher quelqu'un.
	var accountID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role, client_id)
		 VALUES ($1, 'x', 'client', $2) RETURNING id`,
		"portail-"+uuid.NewString()+"@piilot.test", client.ID,
	).Scan(&accountID)
	if err != nil {
		t.Fatalf("creation du compte : %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, accountID); err != nil {
			t.Errorf("nettoyage du compte : %v", err)
		}
	})

	after, err := clients.Get(ctx, client.ID)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}

	if after.PortalUsers != 1 {
		t.Errorf("portal_users attendu a 1, recu %d", after.PortalUsers)
	}
	if len(after.Accounts) != 1 {
		t.Fatalf("la fiche devrait lister le compte, recu %d", len(after.Accounts))
	}

	// Suppression logique : le declencheur doit la voir comme un depart.
	if _, err := pool.Exec(ctx, `UPDATE users SET deleted_at = now() WHERE id = $1`, accountID); err != nil {
		t.Fatalf("suppression logique : %v", err)
	}

	after, err = clients.Get(ctx, client.ID)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}

	if after.PortalUsers != 0 {
		t.Errorf("portal_users attendu a 0 apres suppression, recu %d", after.PortalUsers)
	}
}

func TestSupprimerUnClientQuiPorteDesProjets(t *testing.T) {
	clients, _, pool := newCRM(t)
	ctx := context.Background()

	client, err := clients.Create(ctx, pro(uniqueName("AvecProjet")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	var projectID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO projects (client_id, name) VALUES ($1, $2) RETURNING id`,
		client.ID, uniqueName("Projet"),
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creation du projet : %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, projectID); err != nil {
			t.Errorf("nettoyage du projet : %v", err)
		}
	})

	// Le compteur de projets actifs suit le declencheur pose en 000011.
	withProject, err := clients.Get(ctx, client.ID)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if withProject.ProjectsActive != 1 {
		t.Errorf("projects_active attendu a 1, recu %d", withProject.ProjectsActive)
	}

	if err := clients.Delete(ctx, client.ID); err == nil {
		t.Fatal("un client portant un projet a ete supprime")
	}
}

func TestSupprimerUnClientEmporteSesContacts(t *testing.T) {
	clients, contacts, pool := newCRM(t)
	ctx := context.Background()

	client, err := clients.Create(ctx, pro(uniqueName("Vide")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	contact, err := contacts.Create(ctx, usecase.CreateContactInput{
		ClientID:  &client.ID,
		Firstname: "Leo",
		Lastname:  "Suivant",
	})
	if err != nil {
		t.Fatalf("creation du contact : %v", err)
	}

	if err := clients.Delete(ctx, client.ID); err != nil {
		t.Fatalf("suppression du client : %v", err)
	}

	if _, err := clients.Get(ctx, client.ID); err == nil {
		t.Error("le client supprime est encore lisible")
	}

	var alive bool
	err = pool.QueryRow(ctx,
		`SELECT deleted_at IS NULL FROM contacts WHERE id = $1`, contact.ID,
	).Scan(&alive)
	if err != nil {
		t.Fatalf("relecture du contact : %v", err)
	}

	if alive {
		t.Error("le contact aurait du etre efface avec son client")
	}
}

func TestDeuxContactsNePartagentPasUneAdresseChezUnClient(t *testing.T) {
	clients, contacts, pool := newCRM(t)
	ctx := context.Background()

	client, err := clients.Create(ctx, pro(uniqueName("Doublon")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	email := "doublon-" + uuid.NewString()[:8] + "@piilot.test"

	if _, err := contacts.Create(ctx, usecase.CreateContactInput{
		ClientID: &client.ID, Firstname: "Claire", Lastname: "Fontaine", Email: email,
	}); err != nil {
		t.Fatalf("premier contact : %v", err)
	}

	// Meme adresse, casse differente : l'index porte sur lower(email).
	_, err = contacts.Create(ctx, usecase.CreateContactInput{
		ClientID: &client.ID, Firstname: "Claire", Lastname: "Fontaine", Email: strings.ToUpper(email),
	})
	if err == nil {
		t.Fatal("une adresse a ete inscrite deux fois chez le meme client")
	}

	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Status != http.StatusConflict {
		t.Fatalf("conflit attendu, recu : %v", err)
	}

	// Deux homonymes sans adresse restent possibles : rien ne dit qu'il s'agit
	// de la meme personne.
	for range 2 {
		if _, err := contacts.Create(ctx, usecase.CreateContactInput{
			ClientID: &client.ID, Firstname: "Sans", Lastname: "Adresse",
		}); err != nil {
			t.Fatalf("homonyme sans adresse refuse : %v", err)
		}
	}
}

func TestDeplacerUnClientDansLePipeline(t *testing.T) {
	clients, _, pool := newCRM(t)
	ctx := context.Background()

	client, err := clients.Create(ctx, pro(uniqueName("Pipeline")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	// Un client neuf entre en tete de pipeline.
	if client.Status != "lead" {
		t.Errorf("statut initial attendu « lead », recu %q", client.Status)
	}

	moved, err := clients.MoveStatus(ctx, client.ID, "devis")
	if err != nil {
		t.Fatalf("deplacement : %v", err)
	}
	if moved.Status != "devis" {
		t.Errorf("statut attendu « devis », recu %q", moved.Status)
	}

	if _, err := clients.MoveStatus(ctx, client.ID, "inconnu"); err == nil {
		t.Error("un statut hors de la liste a ete accepte")
	}
}

func TestDeplacerUneCarteNEffacePasLesCoordonnees(t *testing.T) {
	clients, _, pool := newCRM(t)
	ctx := context.Background()

	client, err := clients.Create(ctx, pro(uniqueName("Coordonnees")))
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	siret := randomSiret()
	if _, err := clients.Update(ctx, client.ID, usecase.UpdateClientInput{
		Name:    client.Name,
		Status:  "actif",
		Website: "https://exemple.fr",
		Identity: usecase.ClientIdentity{
			City:      "Lille",
			Siret:     siret,
			LegalName: client.Name,
		},
	}); err != nil {
		t.Fatalf("modification : %v", err)
	}

	// Le deplacement n'ecrit que le statut : c'est ce qui distingue
	// MoveStatus d'Update, et ce que cette assertion protege.
	if _, err := clients.MoveStatus(ctx, client.ID, "veille"); err != nil {
		t.Fatalf("deplacement : %v", err)
	}

	after, err := clients.Get(ctx, client.ID)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}

	if after.Status != "veille" {
		t.Errorf("statut attendu « veille », recu %q", after.Status)
	}
	if after.Website != "https://exemple.fr" || after.City != "Lille" || after.Siret != siret {
		t.Errorf("les coordonnees ont ete perdues : %+v", after.CrmClientItem)
	}
}

func TestUnParticulierSInscritAvecSonContactPrincipal(t *testing.T) {
	clients, _, pool := newCRM(t)
	ctx := context.Background()

	lastname := uniqueName("Martin")
	client, err := clients.Create(ctx, usecase.CreateClientInput{
		Identity: usecase.ClientIdentity{
			Kind:       usecase.ClientParticulier,
			Address:    "12 rue des Lilas",
			PostalCode: "59000",
			City:       "Lille",
			// Un particulier n'a pas de SIRET : ce qui arrive est ignore.
			Siret:           "55203253400646",
			RegistryChecked: true,
		},
		Person: usecase.NewContactInput{Firstname: "Camille", Lastname: lastname, Email: "camille@example.fr", Phone: "+33612345678"},
	})
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, client.ID)

	if client.Name != "Camille "+lastname {
		t.Errorf("nom : %q", client.Name)
	}
	if client.Kind != usecase.ClientParticulier || client.Siret != "" || client.RegistryCheckedAt != nil {
		t.Errorf("identite : %+v", client)
	}
	if client.Phone != "+33612345678" || client.City != "Lille" {
		t.Errorf("coordonnees : %+v", client)
	}
	if client.PrimaryContact == nil || client.PrimaryContact.Lastname != lastname {
		t.Fatalf("contact principal : %+v", client.PrimaryContact)
	}

	detail, err := clients.Get(ctx, client.ID)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if detail.ContactsCount != 1 || detail.PrimaryContact == nil {
		t.Errorf("fiche : %d contact(s), principal %+v", detail.ContactsCount, detail.PrimaryContact)
	}
}

func TestUnParticulierSansNomEstRefuse(t *testing.T) {
	clients, _, _ := newCRM(t)

	_, err := clients.Create(context.Background(), usecase.CreateClientInput{
		Identity: usecase.ClientIdentity{Kind: usecase.ClientParticulier},
		Person:   usecase.NewContactInput{Firstname: "Camille"},
	})

	var derr *domain.Error
	if !errors.As(err, &derr) || derr.Code != domain.ErrValidation.Code || derr.Details["lastname"] == nil {
		t.Fatalf("attendu un refus sur le nom, recu %v", err)
	}
}

func TestUnProfessionnelExigeUnSiretValideEtUnique(t *testing.T) {
	clients, _, pool := newCRM(t)
	ctx := context.Background()

	var derr *domain.Error

	_, err := clients.Create(ctx, usecase.CreateClientInput{
		Identity: usecase.ClientIdentity{Kind: usecase.ClientProfessionnel, LegalName: uniqueName("Sans SIRET")},
	})
	if !errors.As(err, &derr) || derr.Details["siret"] == nil {
		t.Fatalf("SIRET absent accepte : %v", err)
	}

	_, err = clients.Create(ctx, usecase.CreateClientInput{
		Identity: usecase.ClientIdentity{Kind: usecase.ClientProfessionnel, LegalName: uniqueName("Cle fausse"), Siret: "55203253400647"},
	})
	if !errors.As(err, &derr) || derr.Details["siret"] == nil {
		t.Fatalf("SIRET a la cle fausse accepte : %v", err)
	}

	in := pro(uniqueName("Registre"))
	in.Name = ""
	in.Identity.Siret = in.Identity.Siret[:9] + " " + in.Identity.Siret[9:]
	in.Identity.LegalForm = "SAS, société par actions simplifiée"
	in.Identity.RegistryChecked = true

	first, err := clients.Create(ctx, in)
	if err != nil {
		t.Fatalf("creation : %v", err)
	}
	dropClient(t, pool, first.ID)

	// Le nom d'usage reprend la raison sociale, le SIRET perd ses espaces et
	// le numero de TVA se deduit du SIREN.
	if first.Name != in.Identity.LegalName || len(first.Siret) != 14 {
		t.Errorf("identite : %+v", first)
	}
	if !strings.HasPrefix(first.VatNumber, "FR") || !strings.HasSuffix(first.VatNumber, first.Siret[:9]) {
		t.Errorf("TVA : %q", first.VatNumber)
	}
	if first.RegistryCheckedAt == nil {
		t.Error("la verification au registre n'a pas ete datee")
	}

	dup := pro(uniqueName("Doublon"))
	dup.Identity.Siret = first.Siret
	_, err = clients.Create(ctx, dup)
	if !errors.As(err, &derr) || derr.Code != domain.ErrConflict.Code || derr.Details["siret"] == nil {
		t.Fatalf("doublon de SIRET accepte : %v", err)
	}

	// Garder le SIRET garde la date ; en changer sans verifier l'efface.
	update := usecase.UpdateClientInput{Name: first.Name, Status: "lead", Identity: usecase.ClientIdentity{
		Kind: usecase.ClientProfessionnel, LegalName: first.LegalName, Siret: first.Siret,
	}}
	kept, err := clients.Update(ctx, first.ID, update)
	if err != nil || kept.RegistryCheckedAt == nil {
		t.Fatalf("date perdue sans changement de SIRET : %v, %+v", err, kept.RegistryCheckedAt)
	}

	update.Identity.Siret = randomSiret()
	changed, err := clients.Update(ctx, first.ID, update)
	if err != nil || changed.RegistryCheckedAt != nil {
		t.Fatalf("date gardee pour un autre SIRET : %v, %+v", err, changed.RegistryCheckedAt)
	}
}
