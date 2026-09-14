import {
  FemaleSymbolIcon,
  MaleSymbolIcon,
  NonBinaryIcon,
  UserIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useRef } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { CHAMP, Card, Field, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Avatars } from '@/features/projects/ui'
import {
  sessionQuery,
  useRemoveAvatar,
  useUpdateProfile,
  useUploadAvatar,
} from '@/lib/auth'
import {
  COUNTRIES,
  flagOf,
  formatNational,
  splitPhone,
  stripTrunk,
} from '@/lib/countries'
import { cn } from '@/lib/utils'
import type { User } from '@/types/api'

import { reportError } from './-shared'

export const Route = createFileRoute('/_app/compte/')({
  component: PersonalPage,
})

/**
 * Etat civil et coordonnees.
 *
 * Les deux sections ont leur propre formulaire et leur propre enregistrement :
 * corriger une faute dans son adresse ne doit pas obliger a renvoyer son nom.
 */
function PersonalPage() {
  const { data: user } = useQuery(sessionQuery)

  if (user === undefined) return null

  return (
    <div className="flex flex-col-reverse items-start gap-5 lg:flex-row">
      <div className="flex min-w-0 flex-1 flex-col gap-4">
        <IdentityForm key={`${user.firstname}-${user.lastname}-${user.gender}`} user={user} />
        <ContactForm key={user.email} user={user} />
      </div>

      <ProfileCard user={user} />
    </div>
  )
}

/**
 * Le sexe est facultatif, et la chaine vide en est une valeur.
 *
 * « Ne pas se prononcer » doit rester possible : c'est pourquoi la liste porte
 * une quatrieme entree plutot qu'un champ qu'on ne pourrait plus vider une
 * fois rempli.
 *
 * Cette entree ne peut pas porter la chaine vide dans la liste deroulante :
 * Radix la reserve a « rien de choisi », et un item qui la porterait serait
 * refuse. D'ou un jeton interne, traduit dans les deux sens au bord du champ.
 */
const UNSET = 'none'

const GENDERS = [
  { value: UNSET, label: 'Non précisé', icon: UserIcon },
  { value: 'female', label: 'Femme', icon: FemaleSymbolIcon },
  { value: 'male', label: 'Homme', icon: MaleSymbolIcon },
  { value: 'nonbinary', label: 'Non-binaire', icon: NonBinaryIcon },
] as const

const identitySchema = z.object({
  firstname: z.string().trim().min(1, 'Le prénom est requis'),
  lastname: z.string().trim().min(1, 'Le nom est requis'),
  gender: z.enum(['', 'female', 'male', 'nonbinary']),
})

type Gender = z.infer<typeof identitySchema>['gender']

function IdentityForm({ user }: { user: User }) {
  const update = useUpdateProfile()

  const form = useForm<z.infer<typeof identitySchema>>({
    resolver: zodResolver(identitySchema),
    defaultValues: {
      firstname: user.firstname,
      lastname: user.lastname,
      gender: user.gender,
    },
  })

  const gender = form.watch('gender')

  return (
    <form
      onSubmit={form.handleSubmit((values) =>
        update.mutate(values, {
          onSuccess: () => {
            toast.success('Informations enregistrées')
            form.reset(values)
          },
          onError: reportError,
        }),
      )}
    >
      <Card title="Informations personnelles">
        <Field label="Prénom" error={form.formState.errors.firstname?.message}>
          <Input className={CHAMP} {...form.register('firstname')} />
        </Field>

        <Field label="Nom" error={form.formState.errors.lastname?.message}>
          <Input className={CHAMP} {...form.register('lastname')} />
        </Field>

        <Field label="Sexe" error={form.formState.errors.gender?.message}>
          <Select
            value={gender === '' ? UNSET : gender}
            onValueChange={(value) =>
              form.setValue('gender', value === UNSET ? '' : (value as Gender), {
                shouldDirty: true,
              })
            }
          >
            {/* `data-[size=default]:h-auto` : le composant fixe sa hauteur sous
                ce variant, et une classe simple ne l'emporterait pas. */}
            <SelectTrigger className={cn(CHAMP, 'data-[size=default]:h-auto')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {/* L'icone est posee dans l'item, donc dans le texte que Radix
                  recopie au declencheur : le choix courant la garde une fois
                  le menu referme. */}
              {GENDERS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  <HugeiconsIcon
                    icon={option.icon}
                    size={16}
                    strokeWidth={1.8}
                    className="text-[#73757c]"
                  />
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>

        <SaveBar
          dirty={form.formState.isDirty}
          pending={update.isPending}
          onCancel={() => form.reset()}
        />
      </Card>
    </form>
  )
}

const contactSchema = z.object({
  email: z.string().trim().email('Adresse invalide'),
  phone: z.string().trim(),
  address: z.string().trim(),
  postal_code: z.string().trim().regex(/^\d*$/, 'Chiffres uniquement'),
  city: z.string().trim(),
  country: z.string().trim(),
})

function ContactForm({ user }: { user: User }) {
  const update = useUpdateProfile()

  const form = useForm<z.infer<typeof contactSchema>>({
    resolver: zodResolver(contactSchema),
    defaultValues: {
      email: user.email,
      phone: user.phone,
      address: user.address,
      postal_code: user.postal_code,
      city: user.city,
      country: user.country,
    },
  })

  const phone = form.watch('phone')
  const postalCode = form.watch('postal_code')

  return (
    <form
      onSubmit={form.handleSubmit((values) =>
        update.mutate(values, {
          onSuccess: () => {
            toast.success('Coordonnées enregistrées')
            form.reset(values)
          },
          onError: reportError,
        }),
      )}
    >
      <Card title="Contact">
        <Field
          label="Adresse e-mail"
          hint="Elle sert aussi à vous connecter."
          error={form.formState.errors.email?.message}
        >
          <Input type="email" className={CHAMP} {...form.register('email')} />
        </Field>

        <Field label="Téléphone" error={form.formState.errors.phone?.message}>
          <PhoneField
            value={phone}
            onChange={(next) => form.setValue('phone', next, { shouldDirty: true })}
          />
        </Field>

        <Field label="Adresse" error={form.formState.errors.address?.message}>
          <Input autoComplete="street-address" className={CHAMP} {...form.register('address')} />
        </Field>

        {/* Code postal et ville sur une ligne : on les lit et on les saisit
            ensemble, et le code postal n'a pas besoin de toute la largeur. */}
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="sm:w-[140px]">
            <Field label="Code postal" error={form.formState.errors.postal_code?.message}>
              {/* Champ controle plutot que `register` : le filtrage se fait a
                  la frappe, ce qui couvre aussi le collage. Une regle de
                  validation seule laisserait taper des lettres pour les
                  refuser ensuite. */}
              <Input
                inputMode="numeric"
                autoComplete="postal-code"
                maxLength={10}
                value={postalCode}
                onChange={(event) =>
                  form.setValue('postal_code', event.target.value.replace(/\D/g, ''), {
                    shouldDirty: true,
                  })
                }
                className={CHAMP}
              />
            </Field>
          </div>

          <div className="min-w-0 flex-1">
            <Field label="Ville" error={form.formState.errors.city?.message}>
              <Input
                autoComplete="address-level2"
                className={CHAMP}
                {...form.register('city')}
              />
            </Field>
          </div>
        </div>

        <Field label="Pays" error={form.formState.errors.country?.message}>
          <Input autoComplete="country-name" className={CHAMP} {...form.register('country')} />
        </Field>

        <SaveBar
          dirty={form.formState.isDirty}
          pending={update.isPending}
          onCancel={() => form.reset()}
        />
      </Card>
    </form>
  )
}

/**
 * Numero de telephone, precede de son indicatif.
 *
 * Les deux se rangent dans un seul champ en base : un numero se lit et se
 * compose d'un bloc, et separer l'indicatif aurait demande une colonne de plus
 * pour une valeur qui ne se lit jamais seule. Le decoupage se fait ici, a
 * l'affichage.
 *
 * Le declencheur ne montre que le drapeau et l'indicatif : le nom du pays
 * tiendrait la moitie de la ligne, et il se lit dans le menu au moment ou l'on
 * choisit.
 */
function PhoneField({
  value,
  onChange,
}: {
  value: string
  onChange: (next: string) => void
}) {
  const { country, national } = splitPhone(value)

  return (
    <div className="flex items-start gap-2">
      <Select
        value={country.code}
        onValueChange={(code) => {
          const next = COUNTRIES.find((item) => item.code === code)

          // Le numero ne bouge pas, seul son indicatif change : le prefixe
          // national du pays qu'on quitte a deja ete retire a la saisie.
          if (next !== undefined) onChange(national === '' ? '' : next.dial + national)
        }}
      >
        <SelectTrigger
          aria-label="Indicatif du pays"
          className={cn(CHAMP, 'w-[104px] shrink-0 data-[size=default]:h-auto')}
        >
          <SelectValue>
            <span className="flex items-center gap-1.5">
              <span aria-hidden>{flagOf(country.code)}</span>
              {country.dial}
            </span>
          </SelectValue>
        </SelectTrigger>

        <SelectContent className="w-[280px]">
          {COUNTRIES.map((item) => (
            <SelectItem key={item.code} value={item.code}>
              <span aria-hidden>{flagOf(item.code)}</span>
              <span className="min-w-0 flex-1 truncate">{item.name}</span>
              <span className="text-[#73757c]">{item.dial}</span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Le champ ne garde que des chiffres, retire le prefixe national et
          les espace selon le pays. La saisie se lit donc formatee pendant
          qu'on tape, et c'est la forme internationale compacte qui part au
          serveur — celle qui se compose partout. */}
      <Input
        type="tel"
        inputMode="tel"
        autoComplete="tel-national"
        placeholder={formatNational(country, '612345678')}
        value={formatNational(country, national)}
        onChange={(event) => {
          const digits = stripTrunk(country, event.target.value.replace(/\D/g, ''))

          onChange(digits === '' ? '' : country.dial + digits)
        }}
        className={CHAMP}
      />
    </div>
  )
}

/**
 * Photo, identite, et les deux actions qui portent sur la photo.
 *
 * Meme habillage que les sections du formulaire, et collee en haut quand la
 * page defile : c'est le repere qui dit de quel compte on regle les champs,
 * il n'a pas a disparaitre des qu'on descend.
 *
 * `top-5` et non `top-0` : une fois collee, la carte doit garder l'ecart
 * qu'elle a avec la colonne d'a cote — celui du `gap-5` des deux colonnes —
 * faute de quoi elle vient buter contre le haut du cadre.
 */
function ProfileCard({ user }: { user: User }) {
  const upload = useUploadAvatar()
  const remove = useRemoveAvatar()
  const input = useRef<HTMLInputElement>(null)
  const name = `${user.firstname} ${user.lastname}`.trim() || 'Sans nom'

  return (
    <div className="w-full shrink-0 lg:sticky lg:top-5 lg:w-[260px]">
      <Card title="Photo de profil">
        <div className="flex w-full flex-col items-center gap-3 py-2">
          {/* `Avatars` sert deja les equipes et les affectations : la photo quand
              il y en a une, les initiales sinon, sur la meme pastille. Le compte
              connecte ne porte pas ses initiales — le serveur ne les calcule que
              pour les personnes affichees dans les listes. */}
          <Avatars
            people={[
              {
                id: user.id,
                firstname: user.firstname,
                lastname: user.lastname,
                initials: `${user.firstname.at(0) ?? ''}${user.lastname.at(0) ?? ''}`.toUpperCase(),
                avatar_url: user.avatar_url,
              },
            ]}
            max={1}
            size={80}
          />

          <div className="flex w-full flex-col text-center">
            <p className="truncate text-[14px] leading-[1.5] font-medium text-[#1b1b1b]">{name}</p>
            <p className="truncate text-[12px] leading-[1.5] text-[#73757c]">{user.email}</p>
          </div>

          <div className="flex items-center gap-3">
            {user.avatar_url != null && user.avatar_url !== '' && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={remove.isPending}
                onClick={() =>
                  remove.mutate(undefined, {
                    onSuccess: () => toast.success('Photo retirée'),
                    onError: reportError,
                  })
                }
                className="text-[12px] text-[#ff4345] hover:text-[#ff4345]"
              >
                Retirer
              </Button>
            )}

            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={upload.isPending}
              onClick={() => input.current?.click()}
              className="text-[12px]"
            >
              {upload.isPending ? 'Envoi…' : 'Changer la photo'}
            </Button>
          </div>

          <input
            ref={input}
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
            hidden
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ''

              if (file === undefined) return

              upload.mutate(file, {
                onSuccess: () => toast.success('Photo mise à jour'),
                onError: reportError,
              })
            }}
          />
        </div>
      </Card>
    </div>
  )
}
