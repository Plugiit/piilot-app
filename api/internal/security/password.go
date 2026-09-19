package security

import (
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// PasswordCost regle le cout bcrypt. 12 represente environ 250 ms de calcul sur
// le materiel courant : assez lent pour rendre une attaque par dictionnaire
// couteuse, assez rapide pour ne pas peser sur un login legitime.
//
// Augmenter ce cout n'invalide pas les mots de passe existants : bcrypt stocke
// le cout dans l'empreinte, et VerifyPassword relit celui de l'empreinte, pas
// cette constante.
const PasswordCost = 12

// MinPasswordLength : plancher volontairement bas. La longueur est une garantie
// bien plus solide que les regles de composition (une majuscule, un chiffre...),
// qui poussent surtout a des mots de passe previsibles.
const MinPasswordLength = 12

// dummyHash sert a egaliser le temps de reponse quand le compte n'existe pas.
// Sans lui, un login sur un email inconnu repondrait sans passer par bcrypt,
// et l'ecart de duree suffirait a enumerer les comptes existants.
//
// Calcule a la premiere utilisation plutot qu'au demarrage : le cout de 250 ms
// ne pese ni sur le boot du conteneur ni sur les tests qui n'y touchent pas.
var dummyHash = sync.OnceValue(func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("mot de passe factice, jamais valide"), PasswordCost)
	if err != nil {
		// GenerateFromPassword n'echoue que sur un cout hors bornes, ce qui est
		// exclu par la constante ci-dessus.
		panic(fmt.Sprintf("bcrypt indisponible : %v", err))
	}
	return h
})

// HashPassword produit l'empreinte a stocker dans users.password_hash.
func HashPassword(plain string) (string, error) {
	if len(plain) < MinPasswordLength {
		return "", fmt.Errorf("mot de passe trop court : %d caracteres, minimum %d", len(plain), MinPasswordLength)
	}

	// bcrypt tronque silencieusement au-dela de 72 octets : deux mots de passe
	// partageant leurs 72 premiers octets seraient interchangeables. Mieux vaut
	// refuser que d'accepter une equivalence invisible.
	if len(plain) > 72 {
		return "", fmt.Errorf("mot de passe trop long : %d octets, maximum 72", len(plain))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plain), PasswordCost)
	if err != nil {
		return "", fmt.Errorf("hachage du mot de passe : %w", err)
	}

	return string(hash), nil
}

// VerifyPassword compare un mot de passe en clair a son empreinte stockee.
//
// Retourne un booleen et non une erreur : l'appelant n'a aucune decision a
// prendre selon la raison de l'echec, et distinguer « empreinte illisible » de
// « mot de passe faux » dans une reponse HTTP renseignerait l'attaquant.
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// WasteComparison consomme le meme temps qu'une verification reelle, sans rien
// valider. A appeler quand le compte est introuvable, pour que la duree de la
// reponse ne trahisse pas l'existence de l'email.
func WasteComparison(plain string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(plain))
}
