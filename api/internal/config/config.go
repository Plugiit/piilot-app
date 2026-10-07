// Package config charge et valide la configuration depuis l'environnement.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/plugiit/piilot-app/api/internal/spaces"
)

// Env identifie l'environnement d'execution.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvStaging     Env = "staging"
	EnvProduction  Env = "production"
)

// Config regroupe toute la configuration de l'API. Aucun autre package ne lit
// os.Getenv : la configuration est chargee une fois au demarrage et injectee,
// ce qui rend les handlers testables sans variables d'environnement.
type Config struct {
	Env  Env
	Port int

	DatabaseURL string
	RedisURL    string

	// Origines autorisees pour l'admin, servi depuis un autre domaine et
	// authentifie par cookie httpOnly (donc CORS avec credentials).
	AdminOrigins []string

	JWTSecret     []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	CookieDomain  string
	PublicBaseURL string

	// Spaces donne a chaque espace — connexion, equipe, administration,
	// portail client — son propre domaine. Vide, tout tient sur un seul.
	Spaces spaces.Spaces

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	// DrainDelay : au signal d'arret, l'instance se declare en arret et
	// continue de servir ce temps-la, pour que la passerelle l'ecarte avant
	// qu'elle ne ferme ses connexions. Nul en developpement.
	DrainDelay time.Duration

	// RunMigrations applique les migrations embarquees au demarrage. Actif par
	// defaut : le binaire porte son schema. A couper si les migrations sont
	// jouees par une etape de deploiement dediee.
	RunMigrations bool

	// Repertoire des pieces jointes des projets, et taille maximale d'un
	// fichier. Le stockage est sur disque local : suffisant pour une instance,
	// a remplacer par un stockage objet europeen le jour ou il y en aura
	// plusieurs derriere un repartiteur.
	FilesDir     string
	MaxUploadMiB int64

	// Repertoire du build de la SPA (web/dist). Renseigne, l'API sert aussi le
	// front sur la meme origine : une seule image, un seul domaine, ni CORS ni
	// URL d'API figee dans le bundle. Vide en developpement, ou Vite sert le
	// front et relaie /api vers l'API.
	StaticDir string

	// Verification des nouvelles versions, une fois toutes les quelques heures,
	// aupres des releases GitHub du depot. Coupable pour une instance qui ne
	// doit rien appeler au-dehors.
	UpdateCheck         bool
	UpdateRepository    string
	UpdateCheckInterval time.Duration

	// DeliverableReminderAfter : delai sans reponse du client au-dela duquel
	// un livrable est relance par e-mail, une fois. Nul pour ne jamais relancer.
	DeliverableReminderAfter time.Duration

	// GitWebhookSecret, s'il est renseigne, remplace le secret tire au sort
	// et garde en base : pour une installation decrite par son environnement.
	// E-mail entrant. Vides, il se regle dans Paramètres > E-mails entrants ;
	// renseignees, elles priment et l'ecran les montre en lecture seule.
	Inbound InboundEnv
	// Retention du journal d'audit.
	AuditRetention time.Duration

	// Au-dela de ce delai sans sauvegarde reussie, les admins sont prevenus.
	// 0 pour une instance sauvegardee autrement, qui ne veut pas d'alerte.
	BackupStaleAfter time.Duration
	GitWebhookSecret string

	// Serveur SMTP des e-mails (invitations, mot de passe oublie). Facultatif :
	// sans lui, les liens se copient depuis l'ecran des comptes.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPSecurity string
}

// Load lit la configuration depuis l'environnement et echoue si une valeur
// obligatoire manque — un demarrage bruyant vaut mieux qu'une API qui repond
// 500 sur la premiere requete.
func Load() (Config, error) {
	cfg := Config{
		Env:             Env(env("APP_ENV", string(EnvDevelopment))),
		Port:            envInt("PORT", 8080),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        env("REDIS_URL", "redis://localhost:6379/0"),
		AdminOrigins:    envList("ADMIN_ORIGINS", "http://localhost:5173"),
		JWTSecret:       []byte(os.Getenv("JWT_SECRET")),
		AccessTTL:       envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL:      envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		CookieDomain:    os.Getenv("COOKIE_DOMAIN"),
		PublicBaseURL:   env("PUBLIC_BASE_URL", "http://localhost:8080"),
		ReadTimeout:     envDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:    envDuration("WRITE_TIMEOUT", 30*time.Second),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		DrainDelay:      envDuration("DRAIN_DELAY", 5*time.Second),
		RunMigrations:   envBool("RUN_MIGRATIONS", true),
		FilesDir:        env("FILES_DIR", "./data/files"),
		MaxUploadMiB:    int64(envInt("MAX_UPLOAD_MIB", 25)),
		StaticDir:       os.Getenv("STATIC_DIR"),

		UpdateCheck:      envBool("UPDATE_CHECK", true),
		UpdateRepository: env("UPDATE_REPOSITORY", "Plugiit/piilot-app"),
		// Un quart d'heure par defaut : les requetes sont conditionnelles, un
		// « rien de nouveau » ne coute rien a GitHub.
		UpdateCheckInterval: envDuration("UPDATE_CHECK_INTERVAL", 15*time.Minute),

		DeliverableReminderAfter: envDuration("DELIVERABLE_REMINDER_AFTER", 72*time.Hour),
		BackupStaleAfter:         envDuration("BACKUP_STALE_AFTER", 36*time.Hour),
		AuditRetention:           envDuration("AUDIT_RETENTION", 365*24*time.Hour),
		Inbound: InboundEnv{
			Address:       strings.TrimSpace(os.Getenv("INBOUND_ADDRESS")),
			IMAPHost:      strings.TrimSpace(os.Getenv("INBOUND_IMAP_HOST")),
			IMAPPort:      envInt("INBOUND_IMAP_PORT", 993),
			IMAPSecurity:  env("INBOUND_IMAP_SECURITY", "tls"),
			IMAPUsername:  os.Getenv("INBOUND_IMAP_USERNAME"),
			IMAPPassword:  os.Getenv("INBOUND_IMAP_PASSWORD"),
			IMAPFolder:    env("INBOUND_IMAP_FOLDER", "INBOX"),
			WebhookSecret: os.Getenv("INBOUND_WEBHOOK_SECRET"),
		},
		GitWebhookSecret: os.Getenv("GIT_WEBHOOK_SECRET"),

		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     envInt("SMTP_PORT", 587),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),
		SMTPSecurity: env("SMTP_SECURITY", "starttls"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL est obligatoire")
	}
	if cfg.Inbound.IMAPSecurity != "tls" && cfg.Inbound.IMAPSecurity != "none" {
		return Config{}, fmt.Errorf("INBOUND_IMAP_SECURITY attend tls ou none (actuel : %q)", cfg.Inbound.IMAPSecurity)
	}

	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET doit faire au moins 32 octets (actuel : %d)", len(cfg.JWTSecret))
	}

	switch cfg.Env {
	case EnvDevelopment, EnvStaging, EnvProduction:
	default:
		return Config{}, fmt.Errorf("APP_ENV invalide : %q", cfg.Env)
	}

	if cfg.SMTPHost != "" && cfg.SMTPFrom == "" {
		return Config{}, fmt.Errorf("SMTP_FROM est obligatoire quand SMTP_HOST est renseigne")
	}

	sp, err := spaces.Parse(
		os.Getenv("AUTH_URL"), os.Getenv("TEAM_URL"), os.Getenv("ADMIN_URL"), os.Getenv("CLIENT_URL"),
		cfg.CookieDomain,
	)
	if err != nil {
		return Config{}, err
	}
	cfg.Spaces = sp
	if sp.Enabled() {
		// Le cookie de session doit valoir sur les quatre domaines.
		cfg.CookieDomain = sp.CookieDomain
	}

	if cfg.UpdateCheckInterval < 5*time.Minute {
		return Config{}, fmt.Errorf("UPDATE_CHECK_INTERVAL vaut au moins 5m (actuel : %s)", cfg.UpdateCheckInterval)
	}

	if cfg.MaxUploadMiB < 1 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_MIB doit valoir au moins 1 (actuel : %d)", cfg.MaxUploadMiB)
	}

	// En local, personne ne route vers une autre instance : attendre ne
	// ferait que retarder chaque arret du rechargement a chaud.
	if cfg.IsDevelopment() && os.Getenv("DRAIN_DELAY") == "" {
		cfg.DrainDelay = 0
	}

	return cfg, nil
}

// UpdaterConfig est la configuration de l'updater, le conteneur qui met
// l'application a jour. Elle ne porte que ce dont il a besoin : la base, ou il
// lit les demandes, et le socket Docker.
type UpdaterConfig struct {
	DatabaseURL string
	LogLevel    string
	// Socket du moteur Docker, monte dans le conteneur de l'updater.
	DockerSocket string
	// Service Compose a mettre a jour.
	Service string
	// Identifiant du conteneur de l'updater : Docker le donne comme nom
	// d'hote, et il sert a retrouver le projet Compose.
	Self string
}

// LoadUpdater lit la configuration de l'updater.
func LoadUpdater() (UpdaterConfig, error) {
	cfg := UpdaterConfig{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		LogLevel:     env("LOG_LEVEL", "info"),
		DockerSocket: env("DOCKER_SOCKET", "/var/run/docker.sock"),
		Service:      env("UPDATER_SERVICE", "server"),
		Self:         os.Getenv("HOSTNAME"),
	}

	if cfg.DatabaseURL == "" {
		return UpdaterConfig{}, fmt.Errorf("DATABASE_URL est obligatoire")
	}
	if cfg.Self == "" {
		return UpdaterConfig{}, fmt.Errorf("HOSTNAME est vide : l'updater doit tourner dans un conteneur Docker")
	}

	return cfg, nil
}

// IsDevelopment indique si l'API tourne en local (logs verbeux, CORS permissif).
// URLFor rend l'adresse publique d'un espace : son domaine en mode
// multi-domaines, PUBLIC_BASE_URL sinon. Sert aux liens des e-mails.
func (c Config) URLFor(space string) string {
	if c.Spaces.Enabled() {
		return c.Spaces.URL(space)
	}
	return c.PublicBaseURL
}

func (c Config) IsDevelopment() bool { return c.Env == EnvDevelopment }

// IsProduction indique si l'API tourne en production (cookies Secure, logs JSON).
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// Addr retourne l'adresse d'ecoute du serveur HTTP.
func (c Config) Addr() string { return fmt.Sprintf(":%d", c.Port) }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envList(key, fallback string) []string {
	raw := env(key, fallback)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// BackupConfig est la configuration de la commande backup, le conteneur qui
// sauvegarde la base et les fichiers.
type BackupConfig struct {
	DatabaseURL string
	LogLevel    string
	// Pieces jointes de l'application, montees en lecture seule.
	FilesDir string
	// Ou les archives sont posees.
	Dir string
	// Heure locale de la sauvegarde quotidienne, « HH:MM ».
	At string
	// Retention : jours, semaines, mois.
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int

	// Destination S3, facultative.
	S3Endpoint  string
	S3Bucket    string
	S3Region    string
	S3AccessKey string
	S3SecretKey string
	S3Prefix    string
}

// LoadBackup lit la configuration de la commande backup.
func LoadBackup() (BackupConfig, error) {
	cfg := BackupConfig{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    env("LOG_LEVEL", "info"),
		FilesDir:    env("FILES_DIR", "./data/files"),
		Dir:         env("BACKUP_DIR", "./data/backups"),
		At:          env("BACKUP_AT", "03:00"),
		KeepDaily:   envInt("BACKUP_KEEP_DAILY", 7),
		KeepWeekly:  envInt("BACKUP_KEEP_WEEKLY", 4),
		KeepMonthly: envInt("BACKUP_KEEP_MONTHLY", 3),

		S3Endpoint:  os.Getenv("BACKUP_S3_ENDPOINT"),
		S3Bucket:    os.Getenv("BACKUP_S3_BUCKET"),
		S3Region:    env("BACKUP_S3_REGION", "fr-par"),
		S3AccessKey: os.Getenv("BACKUP_S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("BACKUP_S3_SECRET_KEY"),
		S3Prefix:    env("BACKUP_S3_PREFIX", "piilot/"),
	}

	if cfg.DatabaseURL == "" {
		return BackupConfig{}, fmt.Errorf("DATABASE_URL est obligatoire")
	}
	if _, err := time.Parse("15:04", cfg.At); err != nil {
		return BackupConfig{}, fmt.Errorf("BACKUP_AT doit etre une heure « HH:MM » (actuel : %q)", cfg.At)
	}
	if (cfg.S3Endpoint != "" || cfg.S3Bucket != "") && (cfg.S3Endpoint == "" || cfg.S3Bucket == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "") {
		return BackupConfig{}, fmt.Errorf("S3 : BACKUP_S3_ENDPOINT, BACKUP_S3_BUCKET, BACKUP_S3_ACCESS_KEY et BACKUP_S3_SECRET_KEY vont ensemble")
	}

	return cfg, nil
}

// InboundEnv est l'e-mail entrant fixe par l'environnement.
type InboundEnv struct {
	Address       string
	IMAPHost      string
	IMAPPort      int
	IMAPSecurity  string
	IMAPUsername  string
	IMAPPassword  string
	IMAPFolder    string
	WebhookSecret string
}
