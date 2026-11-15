// Package csvio lit et ecrit les fichiers CSV de l'import et de l'export.
//
// Le format d'ecriture est celui qu'Excel en francais ouvre sans assistant :
// BOM UTF-8, point-virgule, fins de ligne CRLF. La lecture est plus tolerante
// (separateur et encodage detectes) parce que les fichiers viennent de tous
// les tableurs et de tous les logiciels.
package csvio

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// BOM est la marque d'ordre des octets UTF-8, en tete de chaque fichier ecrit.
const BOM = "\xEF\xBB\xBF"

// Delimiter est le separateur des fichiers ecrits.
const Delimiter = ';'

// formulaTriggers : premiers caracteres qu'un tableur prend pour une formule.
const formulaTriggers = "=+-@\t\r"

// Cell neutralise une cellule qu'un tableur prendrait pour une formule.
//
// Une note commencant par « = » s'executerait a l'ouverture du fichier : une
// apostrophe en tete la fait lire comme du texte. Les notes et les noms sont
// saisis par des personnes, et le fichier est ouvert par d'autres. La lecture
// (Reader) retire cette apostrophe : l'aller-retour ne perd rien.
func Cell(value string) string {
	if value != "" && strings.ContainsRune(formulaTriggers, rune(value[0])) {
		return "'" + value
	}

	return value
}

// FormatDecimal ecrit un nombre a la virgule (« 1,50 »), pour qu'une somme se
// fasse directement dans le tableur.
func FormatDecimal(value float64, precision int) string {
	return strings.Replace(strconv.FormatFloat(value, 'f', precision, 64), ".", ",", 1)
}

// Writer ecrit un fichier CSV : BOM au premier enregistrement, `;`, CRLF.
// Il n'applique pas Cell de lui-meme : un nombre negatif n'est pas une
// formule, c'est a l'appelant de passer ses textes libres par Cell.
type Writer struct {
	out     io.Writer
	cw      *csv.Writer
	started bool
}

// NewWriter cree un Writer. Rien n'est ecrit avant le premier Write.
func NewWriter(out io.Writer) *Writer {
	cw := csv.NewWriter(out)
	cw.Comma = Delimiter
	cw.UseCRLF = true

	return &Writer{out: out, cw: cw}
}

// Write ecrit un enregistrement.
func (w *Writer) Write(record []string) error {
	if !w.started {
		w.started = true

		if _, err := io.WriteString(w.out, BOM); err != nil {
			return fmt.Errorf("ecriture du bom : %w", err)
		}
	}

	if err := w.cw.Write(record); err != nil {
		return fmt.Errorf("ecriture du csv : %w", err)
	}

	return nil
}

// Flush vide le tampon et rend la premiere erreur d'ecriture survenue.
func (w *Writer) Flush() error {
	w.cw.Flush()

	if err := w.cw.Error(); err != nil {
		return fmt.Errorf("ecriture du csv : %w", err)
	}

	return nil
}
