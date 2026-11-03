import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Field } from '@/components/settings-ui'
import { Input } from '@/components/ui/input'

describe('Field', () => {
  it('nomme le champ qu’il porte', () => {
    render(
      <Field label="Branche" hint="Le nom d’une branche">
        <Input />
      </Field>,
    )
    expect(screen.getByRole('textbox', { name: 'Branche' })).toHaveAccessibleDescription('Le nom d’une branche')
  })

  it('signale l’erreur sur le champ', () => {
    render(
      <Field label="Branche" error="Invalide">
        <Input />
      </Field>,
    )
    const input = screen.getByRole('textbox', { name: 'Branche' })
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toHaveAccessibleDescription('Invalide')
  })

  it('laisse tranquille un contenu composite', () => {
    render(
      <Field label="Sections">
        <div>
          <Input aria-label="Une section" />
        </div>
      </Field>,
    )
    expect(screen.getByRole('textbox', { name: 'Une section' })).toBeInTheDocument()
  })
})
