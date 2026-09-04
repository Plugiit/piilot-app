/**
 * Marque-place d'un module dont la route existe mais pas le contenu.
 *
 * Dit ce qu'il en est plutot que de laisser une page blanche : rien n'est
 * casse, le module n'est simplement pas ecrit.
 */
export function EmptyModule({ name }: { name: string }) {
  return (
    <div className="flex h-full items-center justify-center p-6">
      <p className="text-sm text-[#777]">Le module {name} n'est pas encore développé.</p>
    </div>
  )
}
