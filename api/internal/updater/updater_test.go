package updater

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/plugiit/piilot-app/api/internal/docker"
)

// fakeDocker simule le moteur : des conteneurs et des images en memoire, et le
// journal des appels qui changent quelque chose.
type fakeDocker struct {
	containers map[string]docker.Container
	images     map[string]docker.Image
	// Image que le tirage installe sous la reference.
	pulled string
	// Etat de sante que prendra le conteneur cree.
	health string
	calls  []string
	body   map[string]any
}

func (f *fakeDocker) log(call string) { f.calls = append(f.calls, call) }

func (f *fakeDocker) InspectContainer(_ context.Context, id string) (docker.Container, error) {
	c, ok := f.containers[id]
	if !ok {
		return docker.Container{}, docker.ErrNotFound
	}
	return c, nil
}

func (f *fakeDocker) FindContainers(_ context.Context, labels ...string) ([]string, error) {
	var ids []string
	for id, c := range f.containers {
		match := true
		for _, label := range labels {
			key, value, _ := strings.Cut(label, "=")
			if c.Labels()[key] != value {
				match = false
			}
		}
		if match {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (f *fakeDocker) InspectImage(_ context.Context, ref string) (docker.Image, error) {
	img, ok := f.images[ref]
	if !ok {
		return docker.Image{}, docker.ErrNotFound
	}
	return img, nil
}

func (f *fakeDocker) Pull(_ context.Context, repository, tag string) error {
	f.log("pull " + repository + ":" + tag)
	f.images[repository+":"+tag] = f.images[f.pulled]
	return nil
}

func (f *fakeDocker) StopContainer(_ context.Context, id string, _ time.Duration) error {
	f.log("stop " + id)
	return nil
}

func (f *fakeDocker) StartContainer(_ context.Context, id string) error {
	f.log("start " + id)
	c := f.containers[id]
	c.State.Running = true
	if id == "new" {
		c.State.Health = &struct {
			Status string `json:"Status"`
		}{Status: f.health}
	}
	f.containers[id] = c
	return nil
}

func (f *fakeDocker) RenameContainer(_ context.Context, id, name string) error {
	f.log("rename " + id + " " + stripStamp(name))
	return nil
}

// stripStamp retire l'horodatage des noms provisoires, pour des journaux
// comparables d'un passage a l'autre.
func stripStamp(name string) string {
	for _, sep := range []string{"-avant-maj-", "-suivante-"} {
		if base, _, ok := strings.Cut(name, sep); ok {
			return base + sep + "*"
		}
	}
	return name
}

func (f *fakeDocker) RemoveContainer(_ context.Context, id string) error {
	f.log("remove " + id)
	return nil
}

func (f *fakeDocker) CreateContainer(_ context.Context, name string, body any) (string, error) {
	f.log("create " + stripStamp(name))
	f.body = body.(map[string]any)
	f.containers["new"] = docker.Container{ID: "new", State: docker.Container{}.State}
	return "new", nil
}

func (f *fakeDocker) ConnectNetwork(_ context.Context, network, container string, _ docker.Endpoint) error {
	f.log("connect " + network + " " + container)
	return nil
}

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// installation : un updater et une application dans le projet « piilot »,
// l'application sur l'image « old » avec une variable choisie et une heritee.
func installation(health string) *fakeDocker {
	return installationWith(health, map[string]any{"RestartPolicy": map[string]string{"Name": "unless-stopped"}})
}

// installationAvecPort publie le port de l'application sur l'hote, comme le
// fait docker-compose.selfhost.yml sans passerelle.
func installationAvecPort(health string) *fakeDocker {
	return installationWith(health, map[string]any{
		"RestartPolicy": map[string]string{"Name": "unless-stopped"},
		"PortBindings":  map[string]any{"8080/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": "8080"}}},
	})
}

func installationWith(health string, hostConfig map[string]any) *fakeDocker {
	app := docker.Container{
		ID:    "app123456789",
		Name:  "/piilot-app-1",
		Image: "sha256:old",
		Config: raw(map[string]any{
			"Image":    "ghcr.io/plugiit/piilot-app:latest",
			"Hostname": "app123456789",
			"Env":      []string{"DATABASE_URL=postgres://x", "PORT=8080"},
			"Labels": map[string]string{
				labelProject:                       "piilot",
				labelService:                       "app",
				"org.opencontainers.image.version": "0.4.1",
			},
			"Cmd": nil,
		}),
		HostConfig: raw(hostConfig),
	}
	app.State.Running = true
	app.NetworkSettings.Networks = map[string]docker.Endpoint{
		"piilot_default": {Aliases: []string{"app", "app123456789"}},
		"traefik":        {Aliases: []string{"app"}},
	}

	self := docker.Container{ID: "updater", Config: raw(map[string]any{
		"Labels": map[string]string{labelProject: "piilot", labelService: "updater"},
	})}
	self.State.Running = true

	return &fakeDocker{
		containers: map[string]docker.Container{"app123456789": app, "updater": self},
		images: map[string]docker.Image{
			"sha256:old": {ID: "sha256:old", Config: raw(map[string]any{
				"Env":    []string{"PORT=8080"},
				"Labels": map[string]string{"org.opencontainers.image.version": "0.4.1"},
				"Cmd":    nil,
			})},
			"sha256:new": {ID: "sha256:new"},
		},
		pulled: "sha256:new",
		health: health,
	}
}

func newUpdater(d *fakeDocker) *Updater {
	return &Updater{Docker: d, Service: "app", Self: "updater", Sleep: func(time.Duration) {}}
}

func TestTargetTrouveLApplicationDuMemeProjet(t *testing.T) {
	d := installation("healthy")

	target, err := newUpdater(d).Target(context.Background())
	if err != nil || target.ID != "app123456789" {
		t.Fatalf("Target = %q, %v", target.ID, err)
	}
}

func TestUpdateSansCoupureDemarreLaNouvelleAvantDArreterLAncienne(t *testing.T) {
	d := installation("healthy")
	u := newUpdater(d)
	target, _ := u.Target(context.Background())

	if err := u.Update(context.Background(), target, func(string) {}); err != nil {
		t.Fatalf("Update : %v", err)
	}

	want := []string{
		"pull ghcr.io/plugiit/piilot-app:latest",
		"create piilot-app-1-suivante-*",
		"connect traefik new",
		"start new",
		"stop app123456789",
		"remove app123456789",
		"rename new piilot-app-1",
	}
	if strings.Join(d.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("appels :\n%s\nattendu :\n%s", strings.Join(d.calls, "\n"), strings.Join(want, "\n"))
	}
}

func TestUpdateSansCoupureNeToucheJamaisALAncienneSiLaNouvelleEstMalade(t *testing.T) {
	d := installation("unhealthy")
	u := newUpdater(d)
	target, _ := u.Target(context.Background())

	err := u.Update(context.Background(), target, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "pas ete interrompue") {
		t.Fatalf("erreur = %v", err)
	}
	for _, call := range d.calls {
		if strings.Contains(call, "app123456789") {
			t.Fatalf("l'ancienne version a ete touchee : %s", call)
		}
	}
	if d.calls[len(d.calls)-1] != "remove new" {
		t.Errorf("la nouvelle version n'a pas ete retiree : %v", d.calls)
	}
}

func TestUpdateRecreeLeConteneurAvecLaNouvelleImage(t *testing.T) {
	d := installationAvecPort("healthy")
	u := newUpdater(d)
	target, _ := u.Target(context.Background())

	var steps []string
	if err := u.Update(context.Background(), target, func(s string) { steps = append(steps, s) }); err != nil {
		t.Fatalf("Update : %v", err)
	}

	want := []string{
		"pull ghcr.io/plugiit/piilot-app:latest",
		"stop app123456789",
		"rename app123456789 piilot-app-1-avant-maj-*",
		"create piilot-app-1",
		"connect traefik new",
		"start new",
		"remove app123456789",
	}
	if strings.Join(d.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("appels :\n%s\nattendu :\n%s", strings.Join(d.calls, "\n"), strings.Join(want, "\n"))
	}

	if d.body["Image"] != "ghcr.io/plugiit/piilot-app:latest" {
		t.Errorf("image = %v", d.body["Image"])
	}
	if _, kept := d.body["Hostname"]; kept {
		t.Error("le nom d'hote de l'ancien conteneur a ete repris")
	}
	// La variable choisie reste, celle que l'image fixait part avec elle.
	if env := d.body["Env"].([]any); len(env) != 1 || env[0] != "DATABASE_URL=postgres://x" {
		t.Errorf("variables = %v", env)
	}
	// L'etiquette de version etait celle de l'ancienne image : elle ne doit
	// pas masquer celle de la nouvelle.
	labels := d.body["Labels"].(map[string]any)
	if _, kept := labels["org.opencontainers.image.version"]; kept || labels[labelService] != "app" {
		t.Errorf("etiquettes = %v", labels)
	}
	if len(steps) == 0 {
		t.Error("aucune etape signalee")
	}
}

func TestUpdateRemetLAncienneVersionSiLaNouvelleEstMalade(t *testing.T) {
	d := installationAvecPort("unhealthy")
	u := newUpdater(d)
	target, _ := u.Target(context.Background())

	err := u.Update(context.Background(), target, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "remise en route") {
		t.Fatalf("erreur = %v", err)
	}

	tail := strings.Join(d.calls[len(d.calls)-3:], "|")
	if tail != "remove new|rename app123456789 piilot-app-1|start app123456789" {
		t.Errorf("retour arriere : %s", tail)
	}
}

func TestUpdateNeToucheARienQuandLImageNAPasChange(t *testing.T) {
	d := installation("healthy")
	d.pulled = "sha256:old"
	u := newUpdater(d)
	target, _ := u.Target(context.Background())

	err := u.Update(context.Background(), target, func(string) {})
	if !errors.Is(err, ErrUpToDate) {
		t.Fatalf("erreur = %v, attendu ErrUpToDate", err)
	}
	for _, call := range d.calls {
		if strings.HasPrefix(call, "stop") {
			t.Fatal("l'application a ete arretee pour rien")
		}
	}
}

func TestSplitReference(t *testing.T) {
	cas := map[string][2]string{
		"ghcr.io/plugiit/piilot-app:latest": {"ghcr.io/plugiit/piilot-app", "latest"},
		"ghcr.io/plugiit/piilot-app:0.4":    {"ghcr.io/plugiit/piilot-app", "0.4"},
		"ghcr.io/plugiit/piilot-app":        {"ghcr.io/plugiit/piilot-app", "latest"},
		"localhost:5000/piilot":             {"localhost:5000/piilot", "latest"},
		"localhost:5000/piilot:dev":         {"localhost:5000/piilot", "dev"},
	}
	for ref, want := range cas {
		repo, tag, err := splitReference(ref)
		if err != nil || repo != want[0] || tag != want[1] {
			t.Errorf("splitReference(%q) = %q, %q, %v", ref, repo, tag, err)
		}
	}
	if _, _, err := splitReference("ghcr.io/plugiit/piilot-app@sha256:abc"); err == nil {
		t.Error("une image epinglee par empreinte devrait etre refusee")
	}
}
