import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Concatene des classes Tailwind en resolvant les conflits : la derniere
 * classe d'une meme famille gagne, ce qui rend les surcharges par prop fiables.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Extension d'un nom de fichier, en minuscules. Vide quand il n'y en a pas.
 *
 * Un point en tete ne compte pas : « .gitignore » est un nom, pas une
 * extension.
 */
export function extensionOf(filename: string): string {
  const dot = filename.lastIndexOf('.')

  return dot <= 0 ? '' : filename.slice(dot + 1).toLowerCase()
}
