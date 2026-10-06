import type { User } from '@/types/api'

/**
 * Initiales d'un compte, pour la pastille qui remplace sa photo : prenom et
 * nom, ou les deux premieres lettres de l'adresse quand le compte n'a pas
 * encore de nom.
 */
export function userInitials(user: Pick<User, 'firstname' | 'lastname' | 'email'>): string {
  const letters = `${user.firstname.trim().at(0) ?? ''}${user.lastname.trim().at(0) ?? ''}`
  return (letters === '' ? user.email.slice(0, 2) : letters).toUpperCase()
}
