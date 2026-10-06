import { motion } from 'framer-motion'
import { useLayoutEffect, useRef, useState, type ReactNode } from 'react'

import { useSlideTransition } from '@/lib/motion'

/**
 * Enveloppe dont la hauteur suit son contenu, en ressort : une fenetre grandit
 * ou retrecit quand son contenu change au lieu de sauter.
 */
export function AutoHeight({ children }: { children: ReactNode }) {
  const inner = useRef<HTMLDivElement>(null)
  const [height, setHeight] = useState<number | 'auto'>('auto')
  const transition = useSlideTransition()

  useLayoutEffect(() => {
    const node = inner.current
    if (node === null) return

    const observer = new ResizeObserver(([entry]) => {
      if (entry !== undefined) setHeight(entry.contentRect.height)
    })
    observer.observe(node)

    return () => observer.disconnect()
  }, [])

  return (
    <motion.div animate={{ height }} transition={transition} className="-mx-1 overflow-hidden px-1">
      <div ref={inner}>{children}</div>
    </motion.div>
  )
}
