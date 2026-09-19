import { zodResolver } from '@hookform/resolvers/zod'
import { ViewOffIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import dashboardUrl from '@/assets/login/dashboard.png'
import eyeLineUrl from '@/assets/login/eye-line.svg'
import glowUrl from '@/assets/login/glow.svg'
import logoUrl from '@/assets/sidebar/logo.svg'
import starUrl from '@/assets/login/star.svg'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { HttpError } from '@/lib/api'
import { homeFor, login, sessionQuery } from '@/lib/auth'

const searchSchema = z.object({
  redirect: z.string().optional(),
})

export const Route = createFileRoute('/login')({
  validateSearch: searchSchema,
  component: LoginPage,
})

const formSchema = z.object({
  email: z.email("L'adresse e-mail est invalide"),
  password: z.string().min(1, 'Le mot de passe est requis'),
})

type FormValues = z.infer<typeof formSchema>

/**
 * Adresse retenue d'une visite a l'autre.
 *
 * Seule l'adresse est gardee, jamais le mot de passe, et elle ne quitte pas le
 * poste : le serveur ne sait rien de cette case. « Se souvenir » ne prolonge
 * donc aucune session — l'API n'a pas de quoi le faire —, il evite de retaper
 * son adresse.
 *
 * Chaque acces est garde : une fenetre privee ou un navigateur qui refuse le
 * stockage leve ici, et un e-mail non repris ne vaut pas un ecran blanc.
 */
const REMEMBERED_EMAIL = 'login:email'

function readRememberedEmail(): string {
  try {
    return window.localStorage.getItem(REMEMBERED_EMAIL) ?? ''
  } catch {
    return ''
  }
}

function writeRememberedEmail(email: string | null) {
  try {
    if (email === null) window.localStorage.removeItem(REMEMBERED_EMAIL)
    else window.localStorage.setItem(REMEMBERED_EMAIL, email)
  } catch {
    // La connexion aboutit quand meme, l'adresse ne sera simplement pas reprise.
  }
}

function LoginPage() {
  const { redirect } = Route.useSearch()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [remembered] = useState(readRememberedEmail)
  const [remember, setRemember] = useState(remembered !== '')
  const [revealed, setRevealed] = useState(false)

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { email: remembered, password: '' },
  })

  const mutation = useMutation({
    mutationFn: (values: FormValues) => login(values.email, values.password),
    onSuccess: (user, values) => {
      writeRememberedEmail(remember ? values.email : null)

      // La session est ecrite directement dans le cache : la redirection ne
      // declenche pas un second aller-retour vers /auth/me.
      queryClient.setQueryData(sessionQuery.queryKey, user)

      // `redirect` porte la page demandee avant la connexion. A defaut, le
      // role decide de l'espace d'atterrissage.
      void navigate({ to: redirect ?? homeFor(user) })
    },
    onError: (error) => {
      toast.error(error instanceof HttpError ? error.message : 'Connexion impossible')
    },
  })

  return (
    <div className="grid h-full lg:grid-cols-2">
      {/* Panneau de presentation. Masque sous `lg` : ampute de sa capture il ne
          dirait plus rien, et le formulaire seul tient mieux un petit ecran. */}
      <aside className="relative hidden overflow-hidden bg-[#e8e8e9] lg:block">
        {/* Halo orange qui deborde par le bas a gauche.
            Le cercle mesure 804px mais son flou deborde : l'image fait 1292px
            et se dessine par-dessus les bords, d'ou le wrapper en retrait
            negatif. La poser directement a 804px ecraserait le flou dedans. */}
        <div
          aria-hidden
          className="pointer-events-none absolute bottom-[-217px] left-[-237px] size-[804px]"
        >
          <div className="absolute inset-[-30.35%]">
            <img src={glowUrl} alt="" className="block size-full max-w-none" />
          </div>
        </div>

        {/* L'accroche et les etoiles suivent les memes proportions que la
            capture — 60/720 a gauche, 60 et 166 sur 1024 en hauteur. Les
            laisser en pixels fixes pendant que la capture, elle, se cale sur la
            hauteur du panneau les ferait chevaucher des qu'une fenetre est
            basse : a 1024x768 la capture remonte a 205px, juste dans la ligne
            Trustpilot. Ils montent et descendent donc ensemble. */}
        <p className="font-heading absolute top-[5.8594%] left-[8.3333%] w-[83.3333%] max-w-[600px] text-[32px] leading-[1.4] font-medium text-[#1b1b1b]">
          Pilotez vos projets, vos clients et vos livrables au même endroit.
        </p>

        <div className="absolute top-[16.2109%] left-[8.3333%] flex flex-col items-start justify-center gap-3">
          <div className="flex items-center gap-1">
            {/* Cinq pastilles vertes distinctes, comme dans la maquette : une
                boucle sur un tableau de cinq entrees, pas une etoile etiree. */}
            {[0, 1, 2, 3, 4].map((index) => (
              <span
                key={index}
                className="flex items-center rounded-[2px] bg-[#0ca234] p-1"
                aria-hidden
              >
                <img src={starUrl} alt="" className="block size-4 max-w-none" />
              </span>
            ))}
          </div>

          <p className="text-[14px] leading-[1.5] font-medium whitespace-nowrap text-[#73757c]">
            Conçu pour le quotidien d'une agence.
          </p>
        </div>

        {/* La capture deborde volontairement a droite et en bas : c'est ce
            cadrage qui donne l'impression d'un ecran qui continue hors champ.
            Le liset blanc est porte par le cadre, la capture le remplit.

            Les positions sont en proportions du panneau, pas en pixels : la
            maquette est dessinee sur un panneau de 720x1024.
              largeur  1400/720  = 194.4444 %
              gauche     60/720  =   8.3333 %
              haut      273/1024 =  26.6602 %
            La hauteur vient du rapport d'image plutot que d'un second
            pourcentage : reglee sur la hauteur du panneau, elle etirerait la
            capture des que la fenetre n'a plus les proportions du fichier.

            La largeur est plafonnee a 1400px, sa valeur dans la maquette. Sans
            ce plafond elle grandit sans fin avec le panneau : sur un 1920 la
            capture montait a 1866px pour un panneau de 960, et on n'en voyait
            plus la moitie — un tableau de bord zoome au point d'etre illisible.
            Plafonnee, elle garde la taille voulue par la maquette et un grand
            ecran en montre simplement davantage, ce qui est le but.

            `min-h` rattrape le seul effet de bord du plafond : sur un ecran
            haut, une capture bornee a 996px ne touchait plus le bas du panneau
            et laissait une bande grise. 100 % - 26.6602 % de haut = 73.3398 %,
            soit la hauteur minimale pour rejoindre le bord. Le cadre s'etire
            alors sans deformer la capture, que l'`object-cover` recadre. En
            deca cette borne ne sert a rien : la hauteur naturelle depasse deja
            le bas, et la maquette reste rendue au pixel pres. */}
        <div
          aria-hidden
          className="absolute top-[26.6602%] left-[8.3333%] aspect-[1400/996] min-h-[73.3398%] w-[194.4444%] max-w-[1400px] rounded-[16px] border-8 border-white/20"
        >
          <img
            src={dashboardUrl}
            alt=""
            className="absolute inset-0 block size-full max-w-none rounded-[16px] object-cover"
          />
        </div>
      </aside>

      <main className="flex items-center justify-center bg-white px-6 py-10">
        <div className="flex w-full max-w-[476px] flex-col gap-[44px]">
          <header className="flex flex-col items-center gap-5">
            {/* Le logo porte deja son cadre arrondi : pas de pastille autour. */}
            <img src={logoUrl} alt="Plugiit" className="block size-14 max-w-none" />

            <div className="flex w-full flex-col gap-1 text-center">
              <h1 className="font-heading text-[24px] leading-[1.5] font-medium text-[#1b1b1b]">
                Poursuivez avec votre compte
              </h1>
              <p className="text-[16px] leading-[1.5] text-[#8a8c91]">
                Retrouvez vos projets, vos livrables et vos tickets.
              </p>
            </div>
          </header>

          <Form {...form}>
            <form
              className="flex flex-col gap-6"
              onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
              noValidate
            >
              <div className="flex flex-col gap-4">
                <FormField
                  control={form.control}
                  name="email"
                  render={({ field }) => (
                    <FormItem className="gap-1.5">
                      <FormLabel className="text-[16px] leading-[1.5] text-[#1b1b1b]">
                        Adresse e-mail
                      </FormLabel>
                      <FormControl>
                        <Input
                          type="email"
                          autoComplete="username"
                          autoFocus
                          placeholder="Entrez votre adresse e-mail"
                          className="h-auto rounded-[12px] border-[#e8e8e9] p-3 text-[16px] leading-[1.5] placeholder:text-[#8a8c91] md:text-[16px]"
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="password"
                  render={({ field }) => (
                    <FormItem className="gap-1.5">
                      <FormLabel className="text-[16px] leading-[1.5] text-[#1b1b1b]">
                        Mot de passe
                      </FormLabel>
                      <FormControl>
                        <div className="relative">
                          <Input
                            type={revealed ? 'text' : 'password'}
                            autoComplete="current-password"
                            placeholder="Entrez votre mot de passe"
                            className="h-auto rounded-[12px] border-[#e8e8e9] p-3 pr-11 text-[16px] leading-[1.5] placeholder:text-[#8a8c91] md:text-[16px]"
                            {...field}
                          />

                          {/* `type="button"` : sans lui, devoiler le mot de
                              passe enverrait le formulaire.

                              L'oeil au repos est l'asset de la maquette. L'etat
                              devoile prend l'oeil barre de HugeIcons : une
                              bascule demande deux etats, Figma n'en exporte
                              qu'un. */}
                          <button
                            type="button"
                            onClick={() => setRevealed((shown) => !shown)}
                            aria-label={
                              revealed ? 'Masquer le mot de passe' : 'Afficher le mot de passe'
                            }
                            aria-pressed={revealed}
                            className="absolute top-1/2 right-3 flex size-5 -translate-y-1/2 cursor-pointer items-center justify-center rounded-[6px] text-[#8a8c91] transition-colors hover:text-[#1b1b1b]"
                          >
                            {revealed ? (
                              <HugeiconsIcon icon={ViewOffIcon} size={20} strokeWidth={1.6} />
                            ) : (
                              <img src={eyeLineUrl} alt="" className="block size-5 max-w-none" />
                            )}
                          </button>
                        </div>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <div className="flex items-center justify-between gap-2">
                  <div className="flex min-w-0 flex-1 items-center gap-2">
                    <Checkbox
                      id="remember"
                      checked={remember}
                      onCheckedChange={(next) => setRemember(next === true)}
                      className="size-5 rounded-[6px] border-[#e8e8e9]"
                    />
                    <Label
                      htmlFor="remember"
                      className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]"
                    >
                      Se souvenir
                    </Label>
                  </div>

                  {/* Aucun ecran de reinitialisation n'existe encore cote API :
                      le lien est pose comme dans la maquette et ne mene nulle
                      part tant que la route n'est pas ecrite. */}
                  <a
                    href="#"
                    className="text-brand shrink-0 text-right text-[16px] leading-[1.5] whitespace-nowrap hover:underline"
                  >
                    Mot de passe oublié ?
                  </a>
                </div>
              </div>

              <Button
                type="submit"
                className="w-full py-[13px] text-[16px] leading-[1.5]"
                disabled={mutation.isPending}
              >
                {mutation.isPending ? 'Connexion…' : 'Se connecter'}
              </Button>
            </form>
          </Form>
        </div>
      </main>
    </div>
  )
}
