package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/plugiit-api-go/internal/middleware"
)

// Deps regroupe les handlers a monter sur le routeur.
type Deps struct {
	Health   *Health
	Auth     *Auth
	Projects *Projects
	Tasks    *Tasks
	Clients  *Clients
	Contacts *Contacts

	Notifications *Notifications
	Tickets       *Tickets
	Deliverables  *Deliverables
	Services      *Services
	SidebarApps   *SidebarApps
	TimeEntries   *TimeEntries

	Guard *middleware.Guard
}

// Register monte toutes les routes de l'API.
//
// Les endpoints sont organises par vue, pas par entite : chaque route renvoie
// exactement ce qu'un ecran affiche. C'est le choix qui evite le sur-fetch et
// garde 1 a 3 requetes SQL par appel.
func Register(app *fiber.App, deps Deps) {
	// Sondes, hors versionnement : leur contrat ne bouge pas.
	app.Get("/health/live", deps.Health.Live)
	app.Get("/health/ready", deps.Health.Ready)

	// Contrat HTTP, hors versionnement lui aussi : c'est lui qui porte les
	// versions des routes. Le front le consomme pour generer ses types.
	app.Get("/openapi.json", openapiSpec)

	v1 := app.Group("/api/v1")

	registerAuthRoutes(v1.Group("/auth"), deps)
	registerAdminRoutes(v1.Group("/admin"), deps)
}

// registerAuthRoutes monte les endpoints publics d'authentification.
//
// login, logout et refresh sont ouverts : ils portent leur propre preuve
// (identifiants ou jeton de rafraichissement) et ne peuvent donc pas exiger
// une session deja etablie.
func registerAuthRoutes(r fiber.Router, deps Deps) {
	r.Post("/login", deps.Auth.Login)
	r.Post("/logout", deps.Auth.Logout)
	r.Post("/refresh", deps.Auth.Refresh)

	// La garde vient AVANT le handler : Fiber execute les handlers dans l'ordre
	// ou ils sont passes. Une garde placee apres ne s'executerait que si le
	// handler appelait Next(), donc jamais — elle ne protegerait rien.
	r.Get("/me", deps.Guard.Authenticated, deps.Auth.Me)

	// Reglages du compte appelant. Pas de permission a verifier au-dela de
	// l'authentification : chacun dispose du sien, et rien ici ne touche aux
	// droits — ni le role, ni l'etat du compte.
	r.Patch("/me", deps.Guard.Authenticated, deps.Auth.UpdateMe)
	r.Post("/me/password", deps.Guard.Authenticated, deps.Auth.ChangePassword)
	r.Post("/me/avatar", deps.Guard.Authenticated, deps.Auth.UploadAvatar)
	r.Delete("/me/avatar", deps.Guard.Authenticated, deps.Auth.DeleteAvatar)

	// Les photos se lisent entre comptes connectes : elles s'affichent dans les
	// equipes et les affectations, pas seulement sur ses propres reglages.
	r.Get("/avatars/:key", deps.Guard.Authenticated, deps.Auth.Avatar)
}

// registerAdminRoutes monte les endpoints de l'admin. Tout est authentifie et
// reserve aux roles internes.
//
// Un endpoint par vue : chaque GET rend exactement ce qu'un ecran affiche, et
// aucun ecran ne complete sa reponse par un second appel. La liste des projets
// porte donc ses equipes et ses compteurs, le tableau porte ses affectations,
// le panneau de tache porte ses sous-taches et son journal.
//
// Les commentaires d'une tache font exception et ont leur propre route : ils
// vivent dans un onglet, et les charger a l'ouverture du panneau ferait payer a
// chaque consultation une liste que la plupart ne regardent pas.
//
// RequireRole ne filtre que l'espace — « ce compte est interne ». Le droit
// precis se verifie par permission, route par route, pour qu'un changement de
// politique se fasse en base et non dans le code.
func registerAdminRoutes(r fiber.Router, deps Deps) {
	r.Use(deps.Guard.Authenticated, deps.Guard.RequireRole("admin", "team"))

	r.Get("/dashboard", deps.Guard.RequirePermission("projects.read"), deps.Projects.Dashboard)

	// Clients : le strict necessaire au champ « Client » du formulaire de
	// projet — un menu deroulant, pas un ecran.
	r.Get("/clients", deps.Guard.RequirePermission("clients.read"), deps.Projects.ListClients)

	// CRM. Le tableau des clients a sa propre route parce qu'il a son propre
	// contenu : les agregats qu'une ligne affiche n'ont rien a faire dans le
	// menu deroulant ci-dessus, et l'y ajouter l'alourdirait pour tout le monde.
	// Les contacts partagent les permissions des clients : un contact n'existe
	// pas sans son entreprise, et qui peut voir l'une voit l'autre. Un couple
	// `contacts.read` / `contacts.write` n'aurait jamais ete donne separement.
	crm := r.Group("/crm")
	crm.Get("/clients", deps.Guard.RequirePermission("clients.read"), deps.Clients.List)
	crm.Post("/clients", deps.Guard.RequirePermission("clients.write"), deps.Clients.Create)
	// Avant « /:id » : sans quoi le routeur prendrait « board » pour un
	// identifiant de client.
	crm.Get("/clients/board", deps.Guard.RequirePermission("clients.read"), deps.Clients.Board)
	crm.Get("/clients/:id", deps.Guard.RequirePermission("clients.read"), deps.Clients.Get)
	crm.Patch("/clients/:id", deps.Guard.RequirePermission("clients.write"), deps.Clients.Update)
	crm.Delete("/clients/:id", deps.Guard.RequirePermission("clients.write"), deps.Clients.Delete)
	crm.Put("/clients/:id/status", deps.Guard.RequirePermission("clients.write"), deps.Clients.MoveStatus)
	crm.Get("/clients/:id/contacts", deps.Guard.RequirePermission("clients.read"), deps.Contacts.ListOfClient)
	crm.Put("/clients/:id/primary-contact", deps.Guard.RequirePermission("clients.write"), deps.Clients.SetPrimaryContact)
	// Avant « /contacts » sans parametre ? Non : chemin distinct, aucun conflit.
	// Les contacts libres ont leur route parce qu'ils servent un autre ecran —
	// le formulaire de creation d'un client, qui ne peut adopter qu'eux.
	crm.Get("/contacts/free", deps.Guard.RequirePermission("clients.read"), deps.Contacts.ListFree)
	crm.Get("/contacts", deps.Guard.RequirePermission("clients.read"), deps.Contacts.List)
	crm.Post("/contacts", deps.Guard.RequirePermission("clients.write"), deps.Contacts.Create)
	crm.Patch("/contacts/:id", deps.Guard.RequirePermission("clients.write"), deps.Contacts.Update)
	crm.Delete("/contacts/:id", deps.Guard.RequirePermission("clients.write"), deps.Contacts.Delete)

	// Comptes internes, pour les champs d'affectation. Lecture seule : gerer
	// les comptes est un autre ecran, et une autre permission.
	r.Get("/users", deps.Guard.RequirePermission("users.read"), deps.Projects.ListPeople)

	projects := r.Group("/projects")
	projects.Get("", deps.Guard.RequirePermission("projects.read"), deps.Projects.List)
	projects.Post("", deps.Guard.RequirePermission("projects.write"), deps.Projects.Create)
	// Avant « /:id », sans quoi le routeur prendrait « favorites » pour un
	// identifiant de projet : Fiber essaie les routes dans l'ordre ou elles
	// sont montees, la plus specifique doit donc passer la premiere.
	projects.Get("/favorites", deps.Guard.RequirePermission("projects.read"), deps.Projects.Favorites)
	projects.Get("/:id", deps.Guard.RequirePermission("projects.read"), deps.Projects.Get)
	projects.Patch("/:id", deps.Guard.RequirePermission("projects.write"), deps.Projects.Update)
	projects.Delete("/:id", deps.Guard.RequirePermission("projects.write"), deps.Projects.Delete)
	projects.Put("/:id/team", deps.Guard.RequirePermission("projects.write"), deps.Projects.SetTeam)

	// Pieces jointes. L'envoi est sous le projet — c'est lui qui les porte a
	// l'ecran — mais le telechargement et la suppression passent par
	// l'identifiant du fichier : une fois deposee, une piece jointe se
	// designe seule.
	projects.Post("/:id/files", deps.Guard.RequirePermission("projects.write"), deps.Projects.UploadFile)

	// L'etoile est un marquage personnel : elle ne demande pas le droit
	// d'ecrire sur le projet, seulement celui de le voir.
	projects.Put("/:id/favorite", deps.Guard.RequirePermission("projects.read"), deps.Projects.Favorite)
	projects.Delete("/:id/favorite", deps.Guard.RequirePermission("projects.read"), deps.Projects.Unfavorite)

	files := r.Group("/files")
	files.Get("/:id", deps.Guard.RequirePermission("projects.read"), deps.Projects.DownloadFile)
	files.Delete("/:id", deps.Guard.RequirePermission("projects.write"), deps.Projects.DeleteFile)

	// Le tableau des taches est une vue du projet, sa creation aussi : les deux
	// vivent sous le projet parce que c'est lui qui les porte a l'ecran.
	projects.Get("/:id/tasks", deps.Guard.RequirePermission("tasks.read"), deps.Tasks.Board)
	projects.Post("/:id/tasks", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.Create)

	// Les tickets d'un projet vivent sous le projet pour la meme raison que
	// ses taches : c'est sa fiche qui les porte a l'ecran. La permission est
	// celle des tickets et non celle des projets — c'est la donnee lue qui la
	// commande.
	projects.Get("/:id/tickets", deps.Guard.RequirePermission("tickets.read"), deps.Tickets.ProjectList)
	projects.Get("/:id/tickets/board", deps.Guard.RequirePermission("tickets.read"), deps.Tickets.ProjectBoard)

	// Une fois ouverte, une tache se manipule par son seul identifiant : le
	// panneau lateral se partage par lien, sans le projet dans l'adresse.
	tasks := r.Group("/tasks")
	// L'ecran « Taches » du module, qui traverse les projets. Monte avant
	// « /:id » par principe, meme si les deux chemins ne se confondent pas.
	tasks.Get("", deps.Guard.RequirePermission("tasks.read"), deps.Tasks.List)
	tasks.Get("/:id", deps.Guard.RequirePermission("tasks.read"), deps.Tasks.Get)
	tasks.Patch("/:id", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.Update)
	tasks.Delete("/:id", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.Delete)
	tasks.Post("/:id/move", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.Move)
	tasks.Put("/:id/assignees", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.SetAssignees)
	tasks.Post("/:id/subtasks", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.AddSubtask)
	tasks.Post("/:id/files", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.UploadFile)

	// Tickets confies au compte appelant.
	//
	// Pas de parametre `assignee_id` : l'identifiant vient de la session, sinon
	// changer l'adresse suffirait a lire les tickets de quelqu'un d'autre.
	tickets := r.Group("/tickets")
	tickets.Get("/mine", deps.Guard.RequirePermission("tickets.read"), deps.Tickets.Mine)
	tickets.Get("/mine/board", deps.Guard.RequirePermission("tickets.read"), deps.Tickets.MineBoard)
	tickets.Post("", deps.Guard.RequirePermission("tickets.write"), deps.Tickets.Create)

	// La fiche et son registre. « /mine » et « /mine/board » sont montes plus
	// haut : places apres, « mine » serait lu comme un identifiant et rejete
	// par l'analyse de l'UUID.
	tickets.Get("/:id", deps.Guard.RequirePermission("tickets.read"), deps.Tickets.Get)
	tickets.Post("/:id/messages", deps.Guard.RequirePermission("tickets.write"), deps.Tickets.PostMessage)
	tickets.Patch("/:id", deps.Guard.RequirePermission("tickets.write"), deps.Tickets.Rename)

	// Livrables.
	//
	// Le depot vit sous le projet — c'est lui qui porte le livrable a l'ecran —
	// tandis que la file d'attente traverse les projets : elle a son groupe.
	projects.Post("/:id/deliverables", deps.Guard.RequirePermission("deliverables.write"), deps.Deliverables.Create)

	deliverables := r.Group("/deliverables")
	deliverables.Get("", deps.Guard.RequirePermission("deliverables.read"), deps.Deliverables.List)
	deliverables.Get("/:id/versions", deps.Guard.RequirePermission("deliverables.read"), deps.Deliverables.Versions)
	deliverables.Post("/:id/versions", deps.Guard.RequirePermission("deliverables.write"), deps.Deliverables.Submit)

	// Trancher demande un droit a part, que le role client porte aussi : c'est
	// le seul geste d'ecriture que le portail lui accorde.
	deliverables.Post("/:id/decision", deps.Guard.RequirePermission("deliverables.validate"), deps.Deliverables.Decide)

	// Referentiel des prestations de l'agence.
	//
	// Il n'a pas de vue a lui seul : c'est une nomenclature qu'on tient a jour,
	// et les quatre verbes suffisent. Sous les droits des projets — c'est le
	// module qui la porte, et elle n'a pas de permission propre.
	services := r.Group("/services")
	services.Get("", deps.Guard.RequirePermission("projects.read"), deps.Services.List)
	services.Post("", deps.Guard.RequirePermission("projects.write"), deps.Services.Create)
	services.Patch("/:id", deps.Guard.RequirePermission("projects.write"), deps.Services.Update)
	services.Delete("/:id", deps.Guard.RequirePermission("projects.write"), deps.Services.Delete)

	// Temps passe.
	//
	// Pas de parametre de compte : chacun saisit et relit le sien, et
	// l'identifiant vient de la session. Sous les droits des projets — pointer
	// des heures est un geste du module, pas une permission a part.
	temps := r.Group("/time-entries")
	temps.Get("", deps.Guard.RequirePermission("projects.read"), deps.TimeEntries.Sheet)
	temps.Post("", deps.Guard.RequirePermission("projects.read"), deps.TimeEntries.Create)
	temps.Patch("/:id", deps.Guard.RequirePermission("projects.read"), deps.TimeEntries.Update)
	temps.Delete("/:id", deps.Guard.RequirePermission("projects.read"), deps.TimeEntries.Delete)

	// Applications jointes depuis le rail.
	//
	// La lecture est ouverte a tout compte connecte : le rail s'affiche sur
	// chaque page, et exiger un droit d'administration le viderait pour la
	// moitie de l'equipe. Les modifier reste reserve.
	apps := r.Group("/sidebar-apps")
	apps.Get("", deps.Guard.Authenticated, deps.SidebarApps.List)

	// Les logos precedent « /:id » : montes apres, « logos » serait lu comme un
	// identifiant et rejete par l'analyse de l'UUID.
	apps.Get("/logos/:key", deps.Guard.Authenticated, deps.SidebarApps.Logo)

	apps.Post("", deps.Guard.RequirePermission("users.write"), deps.SidebarApps.Create)
	apps.Patch("/:id", deps.Guard.RequirePermission("users.write"), deps.SidebarApps.Update)
	apps.Delete("/:id", deps.Guard.RequirePermission("users.write"), deps.SidebarApps.Delete)
	apps.Post("/:id/logo", deps.Guard.RequirePermission("users.write"), deps.SidebarApps.UploadLogo)
	apps.Delete("/:id/logo", deps.Guard.RequirePermission("users.write"), deps.SidebarApps.DeleteLogo)

	// Repasse le logo en automatique : la demande est enregistree, le job la
	// sert. Monte avant « /:id/logo » ne changerait rien, les chemins ne se
	// confondant pas.
	apps.Post("/:id/logo/auto", deps.Guard.RequirePermission("users.write"), deps.SidebarApps.RefetchLogo)

	// Notifications du compte appelant. Aucune permission a verifier au-dela de
	// l'authentification : chacun ne voit que les siennes, la clause est dans
	// la requete.
	notifications := r.Group("/notifications")
	notifications.Get("", deps.Notifications.List)
	// Le flux precede « /:id » : monte apres, « stream » serait lu comme un
	// identifiant et rejete par l'analyse de l'UUID.
	notifications.Get("/stream", deps.Notifications.Stream)
	notifications.Post("/read", deps.Notifications.MarkAllRead)
	notifications.Post("/:id/read", deps.Notifications.MarkRead)
	tasks.Get("/:id/comments", deps.Guard.RequirePermission("tasks.read"), deps.Tasks.Comments)
	tasks.Post("/:id/comments", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.AddComment)

	// Les sous-taches se modifient par leur propre identifiant : passer par la
	// tache imposerait de la retrouver pour cocher une case.
	subtasks := r.Group("/subtasks")
	subtasks.Patch("/:id", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.UpdateSubtask)
	subtasks.Delete("/:id", deps.Guard.RequirePermission("tasks.write"), deps.Tasks.DeleteSubtask)
}
