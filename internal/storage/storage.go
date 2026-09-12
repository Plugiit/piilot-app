// Package storage ecrit et relit les pieces jointes des projets.
//
// Un paquet a part plutot que du code disperse dans les handlers : c'est ici
// que se trouve la regle qui ferme la traversee de repertoire, et elle doit
// tenir en un seul endroit qu'on puisse lire d'un coup.
//
// L'implementation ecrit sur le disque local. Elle suffit a une agence dont les
// pieces jointes sont des briefs et des maquettes ; le jour ou il faudra
// plusieurs instances derriere un repartiteur, c'est cette interface qu'un
// stockage objet europeen viendra remplir, sans que le reste du code bouge.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store range des octets sous une cle et sait les rendre.
type Store interface {
	Save(r io.Reader, limit int64) (key string, size int64, err error)
	Open(key string) (io.ReadCloser, error)
	Remove(key string) error
}

// ErrTooLarge est rendu quand le contenu depasse la limite passee a Save.
var ErrTooLarge = fmt.Errorf("fichier trop volumineux")

// Local ecrit dans un repertoire du disque.
type Local struct{ root string }

// NewLocal cree le repertoire s'il manque et rend le magasin qui l'exploite.
func NewLocal(root string) (*Local, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("creation du repertoire de stockage : %w", err)
	}

	return &Local{root: root}, nil
}

// path resout une cle en chemin absolu.
//
// La cle est tiree au sort par Save et n'est jamais un nom fourni par
// l'utilisateur ; ce controle est la ceinture par-dessus les bretelles, pour
// que la garantie ne repose pas seulement sur la discipline des appelants.
func (l *Local) path(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("cle vide")
	}

	full := filepath.Join(l.root, filepath.Base(key))
	if filepath.Dir(full) != filepath.Clean(l.root) {
		return "", fmt.Errorf("cle invalide : %q", key)
	}

	return full, nil
}

// Save ecrit le flux sous une cle tiree au sort et rend sa taille.
//
// La lecture est bornee a `limit` octets : sans cela, un client pourrait
// remplir le disque en annoncant une taille et en envoyant davantage. Le
// depassement efface le fichier partiel plutot que de le laisser trainer.
func (l *Local) Save(r io.Reader, limit int64) (key string, size int64, err error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", 0, fmt.Errorf("tirage de la cle : %w", err)
	}
	key = hex.EncodeToString(buf)

	full, err := l.path(key)
	if err != nil {
		return "", 0, err
	}

	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", 0, fmt.Errorf("ouverture du fichier : %w", err)
	}
	defer func() { _ = f.Close() }()

	// limit+1 pour distinguer « pile a la limite » de « au-dela ».
	size, err = io.Copy(f, io.LimitReader(r, limit+1))
	if err != nil {
		_ = os.Remove(full)
		return "", 0, fmt.Errorf("ecriture du fichier : %w", err)
	}

	if size > limit {
		_ = os.Remove(full)
		return "", 0, ErrTooLarge
	}

	if size == 0 {
		_ = os.Remove(full)
		return "", 0, fmt.Errorf("fichier vide")
	}

	return key, size, nil
}

// Open rend le contenu range sous cette cle.
func (l *Local) Open(key string) (io.ReadCloser, error) {
	full, err := l.path(key)
	if err != nil {
		return nil, err
	}

	return os.Open(full)
}

// Remove efface le fichier. L'absence n'est pas une erreur : la ligne de base a
// pu partir sans que le fichier suive, et l'inverse ne doit pas bloquer.
func (l *Local) Remove(key string) error {
	full, err := l.path(key)
	if err != nil {
		return err
	}

	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
