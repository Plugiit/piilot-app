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

	Notifications *Notifications

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
	// projet. Le CRM leur donnera leurs propres ecrans.
	r.Get("/clients", deps.Guard.RequirePermission("clients.read"), deps.Projects.ListClients)

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
