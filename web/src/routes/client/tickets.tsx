import { createFileRoute } from '@tanstack/react-router'

/**
 * Support. Le depot et le suivi des tickets arrivent avec la 0.9.0 ; d'ici la,
 * la page dit comment joindre l'agence plutot que d'afficher un ecran vide.
 */
export const Route = createFileRoute('/client/tickets')({
  component: ClientTicketsPage,
})

function ClientTicketsPage() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-heading text-[26px] leading-tight font-medium text-[#1b1b1b]">Support</h1>
      <p className="rounded-[14px] border border-[#e8e8e9] bg-white p-5 text-[15px] leading-[1.6] text-[#4b4b4f]">
        Le suivi de vos demandes arrive bientôt dans cet espace. En attendant, écrivez à votre interlocuteur
        habituel à l’agence : il vous répondra comme aujourd’hui.
      </p>
    </div>
  )
}
