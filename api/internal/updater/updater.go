// Package updater met a jour le conteneur de l'application : il tire la
// nouvelle image et recree le conteneur a l'identique, en revenant a l'ancien
// si le nouveau ne demarre pas.
//
// Il tourne dans un conteneur a part, le seul de l'installation a acceder au
// socket Docker. Il n'expose aucun port et ne recoit d'ordre que par la base :
// meme une base compromise ne peut lui faire faire qu'une chose, mettre
// l'application a jour vers l'image publiee.
package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/plugiit/piilot-app/api/internal/docker"
)

// Etiquettes que Docker Compose pose sur ses conteneurs. Elles disent a quel
// projet et a quel service un conteneur appartient : c'est ainsi que l'updater
// retrouve l'application sans qu'on ait a la lui designer.
const (
	labelProject = "com.docker.compose.project"
	labelService = "com.docker.compose.service"
)

// Reglages de la recreation.
const (
	// stopGrace : le temps laisse a l'API pour finir ses requetes en cours.
	stopGrace = 30 * time.Second
	// healthTimeout : au-dela, le nouveau conteneur est juge en echec. Large :
	// les migrations s'appliquent au demarrage, et la sonde du conteneur
	// laisse deja trente secondes de mise en route.
	healthTimeout = 3 * time.Minute
	// switchDelay : entre la nouvelle version saine et l'arret de l'ancienne.
	// La passerelle sonde chaque seconde et attend deux sondes reussies avant
	// d'envoyer du trafic a une instance : la bascule est faite bien avant.
	switchDelay = 5 * time.Second
)

// Docker est ce dont l'updater a besoin du moteur. Declare ici pour que les
// tests le remplacent.
type Docker interface {
	InspectContainer(ctx context.Context, id string) (docker.Container, error)
	FindContainers(ctx context.Context, labels ...string) ([]string, error)
	InspectImage(ctx context.Context, ref string) (docker.Image, error)
	Pull(ctx context.Context, repository, tag string) error
	StopContainer(ctx context.Context, id string, grace time.Duration) error
	StartContainer(ctx context.Context, id string) error
	RenameContainer(ctx context.Context, id, name string) error
	RemoveContainer(ctx context.Context, id string) error
	CreateContainer(ctx context.Context, name string, body any) (string, error)
	ConnectNetwork(ctx context.Context, network, container string, endpoint docker.Endpoint) error
}

// ErrUpToDate est rendu quand l'image tiree est celle qui tourne deja.
var ErrUpToDate = errors.New("l'image publiee est celle qui tourne deja")

// Updater met a jour le service `Service` du projet Compose dont il fait
// partie.
type Updater struct {
	Docker  Docker
	Service string
	// Self est l'identifiant du conteneur de l'updater : Docker le donne comme
	// nom d'hote. Il sert a trouver le projet Compose.
	Self string
	// Sleep est remplace en test, pour ne pas attendre de vraies secondes.
	Sleep func(time.Duration)
}

// Target retrouve le conteneur a mettre a jour : celui du service `Service`,
// dans le meme projet Compose que l'updater.
func (u *Updater) Target(ctx context.Context) (docker.Container, error) {
	self, err := u.Docker.InspectContainer(ctx, u.Self)
	if err != nil {
		return docker.Container{}, fmt.Errorf("l'updater ne se trouve pas lui-meme (%s) : %w", u.Self, err)
	}

	project := self.Labels()[labelProject]
	if project == "" {
		return docker.Container{}, errors.New("l'updater ne tourne pas dans un projet Docker Compose")
	}

	ids, err := u.Docker.FindContainers(ctx, labelProject+"="+project, labelService+"="+u.Service)
	if err != nil {
		return docker.Container{}, err
	}

	// Plusieurs conteneurs pour le service : un reste d'une mise a jour
	// interrompue. On prend celui qui tourne.
	var found []docker.Container
	for _, id := range ids {
		c, err := u.Docker.InspectContainer(ctx, id)
		if err != nil {
			return docker.Container{}, err
		}
		if c.State.Running {
			found = append(found, c)
		}
	}

	// Pendant une mise a jour sans coupure, la nouvelle version tourne un
	// moment a cote de l'ancienne, sous un nom provisoire : la cible reste
	// l'ancienne.
	if len(found) > 1 {
		var settled []docker.Container
		for _, c := range found {
			if !strings.Contains(c.Name, "-suivante-") {
				settled = append(settled, c)
			}
		}
		if len(settled) == 1 {
			found = settled
		}
	}

	switch len(found) {
	case 0:
		return docker.Container{}, fmt.Errorf("aucun conteneur « %s » en marche dans le projet %s", u.Service, project)
	case 1:
		return found[0], nil
	default:
		return docker.Container{}, fmt.Errorf("plusieurs conteneurs « %s » en marche dans le projet %s", u.Service, project)
	}
}

// Update tire la derniere image du conteneur `target` et le recree avec.
// `step` recoit chaque etape franchie, pour le suivi a l'ecran.
//
// La nouvelle version demarre a cote de l'ancienne, qui ne s'arrete qu'une
// fois la nouvelle saine : la passerelle bascule le trafic de l'une a
// l'autre, sans coupure. Si la nouvelle ne demarre pas, elle est supprimee et
// l'ancienne n'a jamais cesse de servir.
//
// Une application qui publie un port sur l'hote ne peut pas tourner en deux
// exemplaires — le port est deja pris. Elle est alors arretee avant que la
// nouvelle ne demarre, avec une courte coupure, et remise en route si la
// nouvelle echoue.
//
// Dans les deux cas, les migrations appliquees par la nouvelle version ne se
// defont pas — d'ou la sauvegarde que l'ecran demande avant de lancer.
func (u *Updater) Update(ctx context.Context, target docker.Container, step func(string)) error {
	ref := target.ConfigImage()
	repository, tag, err := splitReference(ref)
	if err != nil {
		return err
	}

	// L'image en cours se lit avant le tirage : celui-ci lui retire son
	// etiquette, et avec le stockage containerd de Docker, une image sans
	// etiquette ne se lit plus par son identifiant, meme quand un conteneur
	// l'utilise encore.
	current, err := u.Docker.InspectImage(ctx, target.Image)
	if err != nil {
		return fmt.Errorf("lecture de l'image en cours : %w", err)
	}

	step("Téléchargement de " + ref)
	if err := u.Docker.Pull(ctx, repository, tag); err != nil {
		return err
	}

	fresh, err := u.Docker.InspectImage(ctx, ref)
	if err != nil {
		return fmt.Errorf("lecture de l'image tiree : %w", err)
	}
	if fresh.ID == target.Image {
		return fmt.Errorf("%w (%s) : l'image est-elle epinglee sur une version ?", ErrUpToDate, ref)
	}

	body, extra, err := createBody(target, current, ref)
	if err != nil {
		return err
	}

	name := strings.TrimPrefix(target.Name, "/")

	if !publishesPorts(target.HostConfig) {
		return u.swap(ctx, target, name, body, extra, step)
	}

	backup := fmt.Sprintf("%s-avant-maj-%d", name, time.Now().Unix())

	step("Arrêt de la version en cours")
	if err := u.Docker.StopContainer(ctx, target.ID, stopGrace); err != nil {
		return err
	}
	if err := u.Docker.RenameContainer(ctx, target.ID, backup); err != nil {
		return u.restore(ctx, target.ID, name, "", fmt.Errorf("mise de cote de l'ancien conteneur : %w", err))
	}

	step("Démarrage de la nouvelle version")
	created, err := u.Docker.CreateContainer(ctx, name, body)
	if err != nil {
		return u.restore(ctx, target.ID, name, "", fmt.Errorf("creation du nouveau conteneur : %w", err))
	}

	for network, endpoint := range extra {
		if err := u.Docker.ConnectNetwork(ctx, network, created, endpoint); err != nil {
			return u.restore(ctx, target.ID, name, created, fmt.Errorf("branchement sur %s : %w", network, err))
		}
	}

	if err := u.Docker.StartContainer(ctx, created); err != nil {
		return u.restore(ctx, target.ID, name, created, fmt.Errorf("demarrage : %w", err))
	}

	step("Vérification de la nouvelle version")
	if err := u.waitHealthy(ctx, created); err != nil {
		return u.restore(ctx, target.ID, name, created, err)
	}

	step("Nettoyage")
	if err := u.Docker.RemoveContainer(ctx, target.ID); err != nil {
		// L'application tourne sur sa nouvelle version : l'ancien conteneur,
		// arrete, ne gene rien. On le signale sans faire echouer la mise a jour.
		step("Ancien conteneur conservé (" + backup + ") : " + err.Error())
	}

	return nil
}

// swap remplace l'application sans coupure : la nouvelle version demarre a
// cote de l'ancienne, sous un nom provisoire, et reprend le nom de l'ancienne
// une fois celle-ci arretee.
func (u *Updater) swap(ctx context.Context, target docker.Container, name string, body map[string]any, extra map[string]docker.Endpoint, step func(string)) error {
	sleep := u.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	next := fmt.Sprintf("%s-suivante-%d", name, time.Now().Unix())

	// Tant que l'ancienne tourne, un echec ne coupe rien : il suffit de
	// retirer la nouvelle.
	abandon := func(created string, cause error) error {
		if created != "" {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
			defer cancel()
			if err := u.Docker.RemoveContainer(cleanup, created); err != nil {
				return fmt.Errorf("%w ; nouvelle version non supprimee (%s) : %v", cause, next, err)
			}
		}
		return fmt.Errorf("%w ; la version en cours n'a pas ete interrompue", cause)
	}

	step("Démarrage de la nouvelle version, à côté de l'actuelle")
	created, err := u.Docker.CreateContainer(ctx, next, body)
	if err != nil {
		return abandon("", fmt.Errorf("creation du nouveau conteneur : %w", err))
	}

	for network, endpoint := range extra {
		if err := u.Docker.ConnectNetwork(ctx, network, created, endpoint); err != nil {
			return abandon(created, fmt.Errorf("branchement sur %s : %w", network, err))
		}
	}

	if err := u.Docker.StartContainer(ctx, created); err != nil {
		return abandon(created, fmt.Errorf("demarrage : %w", err))
	}

	step("Vérification de la nouvelle version")
	if err := u.waitHealthy(ctx, created); err != nil {
		return abandon(created, err)
	}

	step("Bascule du trafic vers la nouvelle version")
	sleep(switchDelay)

	// La nouvelle version sert : ce qui suit ne peut plus faire echouer la
	// mise a jour, seulement laisser un reste a nettoyer.
	step("Arrêt de l'ancienne version")
	if err := u.Docker.StopContainer(ctx, target.ID, stopGrace); err != nil {
		step("Ancienne version non arrêtée : " + err.Error())
	}
	if err := u.Docker.RemoveContainer(ctx, target.ID); err != nil {
		backup := fmt.Sprintf("%s-avant-maj-%d", name, time.Now().Unix())
		step("Ancien conteneur conservé : " + err.Error())
		if err := u.Docker.RenameContainer(ctx, target.ID, backup); err != nil {
			step("Nouvelle version gardée sous le nom " + next)
			return nil
		}
	}

	if err := u.Docker.RenameContainer(ctx, created, name); err != nil {
		step("Nouvelle version gardée sous le nom " + next + " : " + err.Error())
	}

	return nil
}

// publishesPorts dit si le conteneur publie un port sur l'hote — auquel cas
// deux exemplaires ne peuvent pas tourner ensemble.
func publishesPorts(hostConfig json.RawMessage) bool {
	var host struct {
		PortBindings map[string][]struct {
			HostPort string `json:"HostPort"`
		} `json:"PortBindings"`
	}
	if err := json.Unmarshal(hostConfig, &host); err != nil {
		// Dans le doute, la voie prudente : arreter avant de recreer.
		return true
	}
	for _, bindings := range host.PortBindings {
		if len(bindings) > 0 {
			return true
		}
	}
	return false
}

// restore remet l'ancien conteneur en route apres un echec, et rend l'erreur
// d'origine, completee si la remise en route echoue elle aussi.
func (u *Updater) restore(ctx context.Context, oldID, name, created string, cause error) error {
	// Contexte propre : si celui de la mise a jour a expire, il faut quand
	// meme pouvoir remettre l'application en route.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()

	var problems []string
	if created != "" {
		if err := u.Docker.RemoveContainer(ctx, created); err != nil {
			problems = append(problems, "suppression du nouveau conteneur : "+err.Error())
		}
	}
	if err := u.Docker.RenameContainer(ctx, oldID, name); err != nil {
		problems = append(problems, "renommage de l'ancien conteneur : "+err.Error())
	}
	if err := u.Docker.StartContainer(ctx, oldID); err != nil {
		problems = append(problems, "redemarrage de l'ancien conteneur : "+err.Error())
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w ; retour arriere incomplet : %s", cause, strings.Join(problems, " ; "))
	}

	return fmt.Errorf("%w ; version precedente remise en route", cause)
}

// waitHealthy attend que le conteneur soit sain — ou simplement en marche,
// s'il n'a pas de sonde.
func (u *Updater) waitHealthy(ctx context.Context, id string) error {
	sleep := u.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	deadline := time.Now().Add(healthTimeout)
	for {
		c, err := u.Docker.InspectContainer(ctx, id)
		if err != nil {
			return err
		}

		switch {
		// Avec une politique de redemarrage, un conteneur qui plante au
		// demarrage est relance en boucle et reste « running » entre deux
		// essais : sans ce cas, on attendrait la sonde pour rien.
		case c.State.Status == "restarting":
			return errors.New("la nouvelle version redemarre en boucle")
		case !c.State.Running && c.State.Status != "created":
			return fmt.Errorf("la nouvelle version s'est arretee au demarrage (%s)", c.State.Status)
		case c.State.Health == nil && c.State.Running:
			return nil
		case c.State.Health != nil && c.State.Health.Status == "healthy":
			return nil
		case c.State.Health != nil && c.State.Health.Status == "unhealthy":
			return errors.New("la nouvelle version ne repond pas a sa sonde de sante")
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("la nouvelle version n'est pas saine apres %s", healthTimeout)
		}
		sleep(2 * time.Second)
	}
}

// splitReference separe « registre/depot:tag » en depot et tag.
func splitReference(ref string) (string, string, error) {
	if strings.Contains(ref, "@") {
		return "", "", fmt.Errorf("l'image %s est epinglee par empreinte : rien a mettre a jour", ref)
	}

	slash := strings.LastIndex(ref, "/")
	colon := strings.LastIndex(ref, ":")
	if colon > slash {
		return ref[:colon], ref[colon+1:], nil
	}

	return ref, "latest", nil
}

// createBody prepare la configuration du conteneur recree.
//
// Tout est recopie de l'ancien — variables, volumes, ports, etiquettes,
// politique de redemarrage, reseaux — sauf ce qui venait de l'ancienne image :
// une variable ou une etiquette identique a celle de l'image n'a pas ete
// choisie par l'installation, elle doit suivre la nouvelle image. Sans ce tri,
// la nouvelle version tournerait avec les valeurs par defaut de l'ancienne, et
// garderait l'etiquette de version de l'ancienne.
//
// Le premier reseau part dans la creation ; les autres sont rendus a part, a
// brancher avant le demarrage — avant l'API 1.44, la creation n'en accepte
// qu'un.
func createBody(target docker.Container, current docker.Image, ref string) (map[string]any, map[string]docker.Endpoint, error) {
	var cfg, image, host map[string]any
	if err := json.Unmarshal(target.Config, &cfg); err != nil {
		return nil, nil, fmt.Errorf("configuration du conteneur illisible : %w", err)
	}
	if err := json.Unmarshal(current.Config, &image); err != nil {
		return nil, nil, fmt.Errorf("configuration de l'image illisible : %w", err)
	}
	if err := json.Unmarshal(target.HostConfig, &host); err != nil {
		return nil, nil, fmt.Errorf("configuration d'hote illisible : %w", err)
	}

	cfg["Image"] = ref

	// Le nom d'hote par defaut est l'identifiant court de l'ancien conteneur :
	// le garder ferait porter au nouveau le nom de l'ancien.
	if hostname, _ := cfg["Hostname"].(string); strings.HasPrefix(target.ID, hostname) {
		delete(cfg, "Hostname")
	}

	cfg["Env"] = withoutInherited(cfg["Env"], image["Env"])
	cfg["Labels"] = withoutInheritedLabels(cfg["Labels"], image["Labels"])

	for _, key := range []string{"Cmd", "Entrypoint", "Healthcheck", "WorkingDir", "User", "ExposedPorts", "Volumes", "StopSignal", "Shell", "OnBuild"} {
		if reflect.DeepEqual(cfg[key], image[key]) {
			delete(cfg, key)
		}
	}

	extra := map[string]docker.Endpoint{}
	var first string
	for network := range target.NetworkSettings.Networks {
		if first == "" || network < first {
			first = network
		}
	}
	endpoints := map[string]any{}
	for network, endpoint := range target.NetworkSettings.Networks {
		endpoint.Aliases = withoutID(endpoint.Aliases, target.ID)
		if network == first {
			endpoints[network] = endpoint
		} else {
			extra[network] = endpoint
		}
	}

	body := map[string]any{}
	for key, value := range cfg {
		body[key] = value
	}
	body["HostConfig"] = host
	if len(endpoints) > 0 {
		body["NetworkingConfig"] = map[string]any{"EndpointsConfig": endpoints}
	}

	return body, extra, nil
}

// withoutInherited retire d'une liste de variables celles que l'image fixait.
func withoutInherited(container, image any) []any {
	inherited := map[string]bool{}
	if list, ok := image.([]any); ok {
		for _, v := range list {
			if s, ok := v.(string); ok {
				inherited[s] = true
			}
		}
	}

	out := []any{}
	if list, ok := container.([]any); ok {
		for _, v := range list {
			if s, ok := v.(string); ok && inherited[s] {
				continue
			}
			out = append(out, v)
		}
	}

	return out
}

// withoutInheritedLabels retire les etiquettes identiques a celles de l'image.
func withoutInheritedLabels(container, image any) map[string]any {
	inherited, _ := image.(map[string]any)

	out := map[string]any{}
	if labels, ok := container.(map[string]any); ok {
		for key, value := range labels {
			if inherited != nil && reflect.DeepEqual(inherited[key], value) {
				continue
			}
			out[key] = value
		}
	}

	return out
}

// withoutID retire l'alias que le moteur derive de l'identifiant du conteneur.
func withoutID(aliases []string, id string) []string {
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if strings.HasPrefix(id, alias) {
			continue
		}
		out = append(out, alias)
	}

	return out
}
