package uptime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Kuma lit une instance Uptime Kuma : /metrics pour l'etat courant, le
// webhook de notification pour les changements d'etat.
type Kuma struct {
	// URL de l'instance, sans /metrics.
	BaseURL string
	// Cle d'API (Parametres > Cles API). Vide : /metrics est ouvert, ou
	// protege autrement que par Piilot.
	APIKey string
	Client *http.Client
}

var _ Provider = (*Kuma)(nil)

// maxMetricsBody borne la lecture de /metrics : quelques Ko par site, une
// instance de mille sites tient largement dedans.
const maxMetricsBody = 16 << 20

// ListMonitors lit /metrics et regroupe les mesures par site.
//
// L'authentification est celle de Kuma : Basic, nom d'utilisateur vide, la
// cle d'API pour mot de passe.
func (k *Kuma) ListMonitors(ctx context.Context) ([]Monitor, error) {
	client := k.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, MetricsURL(k.BaseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("adresse de Kuma : %w", err)
	}
	if k.APIKey != "" {
		req.SetBasicAuth("", k.APIKey)
	}
	req.Header.Set("Accept", "text/plain")

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("appel de Kuma : %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return nil, ErrUnauthorized
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("Kuma répond %d", res.StatusCode)
	}

	samples, err := ParsePrometheus(io.LimitReader(res.Body, maxMetricsBody))
	if err != nil {
		return nil, err
	}
	monitors := MonitorsFromSamples(samples)
	if len(monitors) == 0 && !hasMonitorMetrics(samples) {
		// Une page qui repond 200 sans aucune mesure de site n'est pas
		// Kuma : une mauvaise adresse, ou un proxy qui renvoie sa page.
		return nil, errors.New("aucune mesure monitor_status : l’adresse ne pointe pas vers Uptime Kuma")
	}
	return monitors, nil
}

// MetricsURL ajoute /metrics a l'adresse de l'instance, sans doubler la
// barre ni le suffixe.
func MetricsURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/metrics") {
		return base
	}
	return base + "/metrics"
}

func hasMonitorMetrics(samples []Sample) bool {
	for _, s := range samples {
		if strings.HasPrefix(s.Name, "monitor_") {
			return true
		}
	}
	return false
}

// MonitorsFromSamples regroupe les mesures de Kuma par site. Un site
// existe des qu'il a un monitor_status ; les autres mesures le completent.
func MonitorsFromSamples(samples []Sample) []Monitor {
	byKey := map[string]*Monitor{}
	var order []string

	get := func(labels map[string]string) *Monitor {
		key := monitorKey(labels)
		if key == "" {
			return nil
		}
		m, ok := byKey[key]
		if !ok {
			m = &Monitor{
				Key:      key,
				Name:     labels["monitor_name"],
				Type:     cleanLabel(labels["monitor_type"]),
				URL:      cleanLabel(labels["monitor_url"]),
				Hostname: cleanLabel(labels["monitor_hostname"]),
				Status:   -1,
			}
			byKey[key] = m
			order = append(order, key)
		}
		return m
	}

	for _, s := range samples {
		if math.IsNaN(s.Value) || math.IsInf(s.Value, 0) {
			continue
		}
		switch s.Name {
		case "monitor_status":
			if m := get(s.Labels); m != nil {
				m.Status = Status(int(s.Value))
			}
		case "monitor_response_time":
			// -1 : Kuma n'a pas de temps de reponse a donner.
			if m := get(s.Labels); m != nil && s.Value >= 0 {
				ms := int(math.Round(s.Value))
				m.ResponseMS = &ms
			}
		case "monitor_cert_days_remaining":
			if m := get(s.Labels); m != nil {
				days := int(s.Value)
				m.CertDays = &days
			}
		case "monitor_cert_is_valid":
			if m := get(s.Labels); m != nil {
				valid := s.Value == 1
				m.CertValid = &valid
			}
		}
	}

	out := make([]Monitor, 0, len(order))
	for _, key := range order {
		m := byKey[key]
		// Sans monitor_status, ce n'est pas un site : une mesure orpheline.
		if !m.Status.Valid() {
			continue
		}
		out = append(out, *m)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// monitorKey : l'identifiant de Kuma quand il le donne (2.x), le nom sinon
// (1.x). Le nom suffit a distinguer les sites d'une instance — Kuma
// n'interdit pas les doublons, mais une instance qui en a ne s'y retrouve
// pas elle-meme.
func monitorKey(labels map[string]string) string {
	if id := cleanLabel(labels["monitor_id"]); id != "" {
		return "id:" + id
	}
	if name := labels["monitor_name"]; name != "" {
		return "name:" + name
	}
	return ""
}

// cleanLabel : Kuma ecrit « null » pour un champ vide.
func cleanLabel(v string) string {
	v = strings.TrimSpace(v)
	if v == "null" || v == "undefined" {
		return ""
	}
	return v
}

// kumaWebhook est le corps du webhook de notification de Kuma (type
// « Webhook », corps JSON).
type kumaWebhook struct {
	Heartbeat *struct {
		MonitorID json.Number `json:"monitorID"`
		Status    *int        `json:"status"`
		Time      string      `json:"time"`
		Msg       string      `json:"msg"`
	} `json:"heartbeat"`
	Monitor *struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	} `json:"monitor"`
	Msg string `json:"msg"`
}

// ParseEvent lit une notification de Kuma. L'essai d'une notification depuis
// Kuma n'a ni battement ni site : ok est faux.
func (k *Kuma) ParseEvent(body []byte) (Event, bool, error) {
	return ParseKumaEvent(body, time.Now())
}

// ParseKumaEvent lit une notification ; now date celle qui n'a pas d'heure
// lisible.
func ParseKumaEvent(body []byte, now time.Time) (Event, bool, error) {
	var w kumaWebhook
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&w); err != nil {
		return Event{}, false, fmt.Errorf("notification illisible : %w", err)
	}
	if w.Heartbeat == nil || w.Heartbeat.Status == nil || w.Monitor == nil {
		return Event{}, false, nil
	}

	ev := Event{Status: Status(*w.Heartbeat.Status), Message: strings.TrimSpace(w.Heartbeat.Msg), At: now}
	if !ev.Status.Valid() {
		return Event{}, false, fmt.Errorf("état inconnu : %d", *w.Heartbeat.Status)
	}
	if ev.Message == "" {
		ev.Message = strings.TrimSpace(w.Msg)
	}

	id := w.Monitor.ID.String()
	if id == "" {
		id = w.Heartbeat.MonitorID.String()
	}
	if _, err := strconv.Atoi(id); err == nil {
		ev.Keys = append(ev.Keys, "id:"+id)
	}
	if w.Monitor.Name != "" {
		ev.Keys = append(ev.Keys, "name:"+w.Monitor.Name)
	}
	if len(ev.Keys) == 0 {
		return Event{}, false, nil
	}

	// Kuma date ses battements en UTC, sans fuseau.
	for _, layout := range []string{"2006-01-02 15:04:05.000", "2006-01-02 15:04:05", time.RFC3339Nano} {
		if t, err := time.ParseInLocation(layout, w.Heartbeat.Time, time.UTC); err == nil {
			ev.At = t
			break
		}
	}
	return ev, true, nil
}
