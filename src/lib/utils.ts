import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Concatene des classes Tailwind en resolvant les conflits : la derniere
 * classe d'une meme famille gagne, ce qui rend les surcharges par prop fiables.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
