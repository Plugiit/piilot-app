import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { COUNTRIES, flagOf, formatNational, splitPhone, stripTrunk } from '@/lib/countries'
import { cn } from '@/lib/utils'

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
 *
 * `className` habille les deux controles. Les reglages du compte y passent
 * leur `CHAMP` ; ailleurs, le champ garde l'habillage ordinaire d'un `Input`,
 * pour ne pas detonner au milieu d'un formulaire qui n'est pas celui-la.
 *
 * Le declencheur ne force aucune hauteur : `Input` et `SelectTrigger` font tous
 * deux `h-8` par defaut, et les deux controles s'alignent d'eux-memes. Un
 * appelant qui impose sa propre hauteur — `CHAMP` et son `py-2` — doit passer
 * `data-[size=default]:h-auto` avec, faute de quoi le selecteur resterait a 32
 * pixels quand le champ voisin grandit.
 */
export function PhoneField({
  value,
  onChange,
  className,
}: {
  value: string
  onChange: (next: string) => void
  className?: string
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
          className={cn('w-[104px] shrink-0', className)}
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
        className={className}
      />
    </div>
  )
}
