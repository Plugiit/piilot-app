import { createFileRoute, Outlet } from '@tanstack/react-router'

import { PageFrame } from '@/components/layout/page-frame'

export const Route = createFileRoute('/_app/compte')({
  component: AccountLayout,
})

/**
 * Chassis des reglages du compte.
 *
 * Repris du fichier de design « Account setting » : un en-tete, puis la page
 * choisie. La colonne de navigation de la maquette n'y est pas — le panneau du
 * module la porte deja, et la dedoubler donnerait deux menus pour le meme
 * choix.
 *
 * Le chassis ne pose que ce qui vaut pour toutes les pages. La carte de profil
 * n'en fait pas partie : elle accompagne les champs d'identite, elle n'a rien
 * a dire sur la securite ni sur les integrations.
 */
function AccountLayout() {
  return (
    <PageFrame title="Mon compte">
      <div className="flex flex-col gap-5 p-6">
        <header className="flex w-full flex-col gap-0.5">
          <h1 className="text-[20px] leading-[1.4] font-medium text-[#1b1b1b]">Mon compte</h1>
          <p className="text-[12px] leading-[1.5] text-[#73757c]">
            Vos informations, votre photo et votre mot de passe.
          </p>
        </header>

        <div className="flex w-full flex-col gap-4">
          <Outlet />
        </div>
      </div>
    </PageFrame>
  )
}
