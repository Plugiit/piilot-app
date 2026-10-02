package security

// Jetons a usage unique des liens envoyes par e-mail : invitations et
// reinitialisations de mot de passe.
//
// Meme construction que les jetons de rafraichissement — 256 bits d'alea, seule
// l'empreinte SHA-256 est stockee — et pour les memes raisons : une fuite de la
// table ne permet pas de suivre un lien a la place de son destinataire, et un
// tirage de cette taille n'a pas besoin d'un hachage lent.

// NewOneTimeToken tire un jeton et rend sa forme claire (placee dans le lien,
// jamais stockee) et son empreinte (stockee).
func NewOneTimeToken() (raw string, hash []byte, err error) {
	return NewRefreshToken()
}

// HashOneTimeToken calcule l'empreinte d'un jeton recu dans un lien.
func HashOneTimeToken(raw string) []byte {
	return HashRefreshToken(raw)
}
