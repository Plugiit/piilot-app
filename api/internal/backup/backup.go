package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Noms dans l'archive.
const (
	dumpName  = "db.dump"
	filesName = "files.tar.gz"
	prefix    = "piilot-"
	suffix    = ".tar"
	// Format du nom : piilot-20261007-030000.tar — lisible, triable, et la
	// date se relit sans metadonnees.
	stamp = "20060102-150405"
)

// Runner fait et restaure les sauvegardes.
type Runner struct {
	// Base a sauvegarder, au format URL de pg_dump.
	DatabaseURL string
	// Pieces jointes de l'application.
	FilesDir string
	// Ou les archives sont posees sur le disque.
	Dir string
	// Ce qu'on garde, sur le disque comme sur S3.
	Keep Retention
	// Destination distante, facultative.
	Remote *S3
	Log    *slog.Logger
}

// Result decrit une sauvegarde faite.
type Result struct {
	Path string
	Size int64
	// Ou elle est : le dossier local, et le seau le cas echeant.
	Location string
}

// Run fait une sauvegarde complete : dump de la base, archive des fichiers,
// une seule archive sur le disque, copie sur S3 si configure, puis menage
// selon la retention.
func (r *Runner) Run(ctx context.Context, now time.Time) (Result, error) {
	if err := os.MkdirAll(r.Dir, 0o750); err != nil {
		return Result{}, fmt.Errorf("dossier des sauvegardes : %w", err)
	}
	work, err := os.MkdirTemp(r.Dir, ".en-cours-")
	if err != nil {
		return Result{}, fmt.Errorf("dossier de travail : %w", err)
	}
	defer os.RemoveAll(work)

	// Base.
	dump := filepath.Join(work, dumpName)
	cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-privileges", "--file="+dump, "--dbname="+r.DatabaseURL)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Result{}, fmt.Errorf("pg_dump : %w : %s", err, strings.TrimSpace(string(out)))
	}

	// Fichiers. Un dossier absent n'est pas une erreur : une instance neuve
	// n'a pas encore de piece jointe.
	files := filepath.Join(work, filesName)
	if _, err := os.Stat(r.FilesDir); err == nil {
		if err := tarDir(r.FilesDir, files, true); err != nil {
			return Result{}, fmt.Errorf("archive des fichiers : %w", err)
		}
	} else {
		if err := os.MkdirAll(filepath.Join(work, "vide"), 0o750); err != nil {
			return Result{}, err
		}
		if err := tarDir(filepath.Join(work, "vide"), files, true); err != nil {
			return Result{}, err
		}
	}

	// Les deux dans une archive : une seule chose a copier, a garder, a
	// restaurer.
	bundle := filepath.Join(work, "bundle")
	if err := os.MkdirAll(bundle, 0o750); err != nil {
		return Result{}, err
	}
	for _, name := range []string{dumpName, filesName} {
		if err := os.Rename(filepath.Join(work, name), filepath.Join(bundle, name)); err != nil {
			return Result{}, err
		}
	}
	name := prefix + now.Format(stamp) + suffix
	final := filepath.Join(r.Dir, name)
	if err := tarDir(bundle, filepath.Join(work, name), false); err != nil {
		return Result{}, fmt.Errorf("archive finale : %w", err)
	}
	if err := os.Rename(filepath.Join(work, name), final); err != nil {
		return Result{}, err
	}
	info, err := os.Stat(final)
	if err != nil {
		return Result{}, err
	}

	result := Result{Path: final, Size: info.Size(), Location: r.Dir}

	if r.Remote.Configured() {
		key := r.Remote.Prefix + name
		if err := r.Remote.Put(ctx, key, final); err != nil {
			return result, fmt.Errorf("copie vers S3 : %w", err)
		}
		result.Location = r.Dir + " + s3://" + r.Remote.Bucket + "/" + key
	}

	if err := r.prune(ctx, now); err != nil {
		// La sauvegarde est faite ; le menage rate n'est qu'un avertissement,
		// le prochain passage le refera.
		r.log().Warn("menage des anciennes sauvegardes", "error", err)
	}

	return result, nil
}

// Restore remet la base et les fichiers dans l'etat d'une archive. La base
// cible doit exister ; son contenu est remplace. Les fichiers sont ajoutes a
// ceux presents — une restauration ne detruit rien qu'elle ne remplace.
func (r *Runner) Restore(ctx context.Context, archive string) error {
	work, err := os.MkdirTemp("", "piilot-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	if err := untar(archive, work, false); err != nil {
		return fmt.Errorf("lecture de l'archive : %w", err)
	}
	dump := filepath.Join(work, dumpName)
	if _, err := os.Stat(dump); err != nil {
		return fmt.Errorf("l'archive ne contient pas %s", dumpName)
	}

	// --clean --if-exists : les objets existants sont remplaces, une base deja
	// migree accepte la restauration. --no-owner : le role de la base cible
	// n'est pas forcement celui d'origine.
	cmd := exec.CommandContext(ctx, "pg_restore",
		"--clean", "--if-exists", "--no-owner", "--no-privileges", "--exit-on-error",
		"--dbname="+r.DatabaseURL, dump)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pg_restore : %w : %s", err, strings.TrimSpace(string(out)))
	}

	files := filepath.Join(work, filesName)
	if _, err := os.Stat(files); err == nil && r.FilesDir != "" {
		if err := os.MkdirAll(r.FilesDir, 0o750); err != nil {
			return err
		}
		if err := untar(files, r.FilesDir, true); err != nil {
			return fmt.Errorf("fichiers : %w", err)
		}
	}

	return nil
}

// Local liste les archives presentes sur le disque, par date.
func (r *Runner) Local() (map[string]time.Time, error) {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]time.Time{}, nil
		}
		return nil, err
	}
	out := map[string]time.Time{}
	for _, e := range entries {
		if at, ok := dateOf(e.Name()); ok {
			out[e.Name()] = at
		}
	}
	return out, nil
}

// Latest rend l'archive la plus recente du disque, vide s'il n'y en a pas.
func (r *Runner) Latest() (string, error) {
	local, err := r.Local()
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(local))
	for n := range local {
		names = append(names, n)
	}
	if len(names) == 0 {
		return "", nil
	}
	sort.Slice(names, func(i, j int) bool { return local[names[i]].After(local[names[j]]) })
	return filepath.Join(r.Dir, names[0]), nil
}

// prune applique la retention sur le disque et sur S3.
func (r *Runner) prune(ctx context.Context, now time.Time) error {
	local, err := r.Local()
	if err != nil {
		return err
	}
	keep := r.Keep.Keep(local, now)
	for name := range local {
		if keep[name] {
			continue
		}
		if err := os.Remove(filepath.Join(r.Dir, name)); err != nil {
			return err
		}
		r.log().Info("sauvegarde effacee", "name", name)
	}

	if !r.Remote.Configured() {
		return nil
	}
	remote, err := r.Remote.List(ctx)
	if err != nil {
		return err
	}
	dated := map[string]time.Time{}
	for key := range remote {
		if at, ok := dateOf(strings.TrimPrefix(key, r.Remote.Prefix)); ok {
			dated[key] = at
		}
	}
	keep = r.Keep.Keep(dated, now)
	for key := range dated {
		if keep[key] {
			continue
		}
		if err := r.Remote.Delete(ctx, key); err != nil {
			return err
		}
		r.log().Info("sauvegarde S3 effacee", "key", key)
	}

	return nil
}

// dateOf relit la date d'un nom d'archive ; faux pour tout autre fichier.
func dateOf(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return time.Time{}, false
	}
	at, err := time.ParseInLocation(stamp, strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix), time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

func (r *Runner) log() *slog.Logger {
	if r.Log == nil {
		return slog.Default()
	}
	return r.Log
}
