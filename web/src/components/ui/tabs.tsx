"use client"

import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { motion } from "framer-motion"
import { cn } from "@/lib/utils"
import { Tabs as TabsPrimitive } from "radix-ui"

import { useSlideTransition } from "@/lib/motion"

const TabsContext = React.createContext<{
  current: string | undefined
  orientation: "horizontal" | "vertical"
}>({ current: undefined, orientation: "horizontal" })

const TabsListContext = React.createContext<{ variant: "default" | "line" }>({
  variant: "default",
})

function Tabs({
  className,
  orientation = "horizontal",
  value,
  defaultValue,
  onValueChange,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Root>) {
  // Radix garde la valeur pour lui quand le composant n'est pas controle ; le
  // marqueur, lui, doit savoir quel onglet est actif. On suit donc la valeur
  // en double, et `value` l'emporte des que l'appelant la fournit.
  const [uncontrolled, setUncontrolled] = React.useState(defaultValue)

  return (
    <TabsContext.Provider value={{ current: value ?? uncontrolled, orientation }}>
      <TabsPrimitive.Root
        data-slot="tabs"
        data-orientation={orientation}
        value={value}
        defaultValue={defaultValue}
        onValueChange={(next) => {
          setUncontrolled(next)
          onValueChange?.(next)
        }}
        className={cn(
          "group/tabs flex gap-2 data-horizontal:flex-col",
          className
        )}
        {...props}
      />
    </TabsContext.Provider>
  )
}

const tabsListVariants = cva(
  "group/tabs-list relative inline-flex w-fit items-center justify-center rounded-lg p-[3px] text-muted-foreground group-data-horizontal/tabs:h-8 group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col data-[variant=line]:rounded-none",
  {
    variants: {
      variant: {
        default: "bg-muted",
        line: "gap-1 bg-transparent",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

/**
 * Barre d'onglets, marqueur compris.
 *
 * Le marqueur est pose ici et non dans l'onglet actif, et sa position est
 * mesuree par rapport a la barre. C'est ce qui lui evite de s'animer quand
 * c'est la barre entiere qui bouge : un bloc qui s'ouvre au-dessus la pousse
 * vers le bas, et le marqueur descend avec elle sans rien interpoler, puisque
 * sa position dans la barre n'a pas change. Seul un changement d'onglet le
 * fait glisser — sur l'axe des onglets, et sur lui seul.
 */
function TabsList({
  className,
  variant = "default",
  indicatorClassName,
  children,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.List> &
  VariantProps<typeof tabsListVariants> & {
    /** Couleur du marqueur, que la barre seule peut porter. */
    indicatorClassName?: string
  }) {
  // `VariantProps` admet null ; le contexte, lui, doit trancher.
  const resolved = variant ?? "default"

  const { current, orientation } = React.useContext(TabsContext)
  const transition = useSlideTransition()
  const list = React.useRef<HTMLDivElement>(null)
  const [box, setBox] = React.useState<{ start: number; size: number } | null>(null)

  React.useLayoutEffect(() => {
    const node = list.current
    if (node === null) return

    function measure() {
      const active = node?.querySelector<HTMLElement>(
        '[data-slot="tabs-trigger"][data-state="active"]'
      )

      if (active == null) {
        setBox(null)

        return
      }

      setBox(
        orientation === "vertical"
          ? { start: active.offsetTop, size: active.offsetHeight }
          : { start: active.offsetLeft, size: active.offsetWidth }
      )
    }

    measure()

    // La barre peut changer de largeur sans changer d'onglet : une fenetre
    // retaillee, une police chargee apres coup.
    const observer = new ResizeObserver(measure)
    observer.observe(node)

    return () => observer.disconnect()
  }, [current, orientation, resolved])

  return (
    <TabsListContext.Provider value={{ variant: resolved }}>
      <TabsPrimitive.List
        ref={list}
        data-slot="tabs-list"
        data-variant={resolved}
        className={cn(tabsListVariants({ variant: resolved }), className)}
        {...props}
      >
        {box !== null && (
          <motion.span
            aria-hidden
            data-slot="tabs-indicator"
            initial={false}
            animate={
              orientation === "vertical"
                ? { y: box.start, height: box.size }
                : { x: box.start, width: box.size }
            }
            transition={transition}
            className={cn(
              "pointer-events-none absolute",
              orientation === "vertical"
                ? resolved === "line"
                  ? "top-0 right-[-1px] w-0.5 bg-foreground"
                  : "inset-x-[3px] top-0 rounded-md bg-background shadow-sm"
                : resolved === "line"
                  ? "bottom-[-1px] left-0 h-0.5 bg-foreground"
                  : "inset-y-[3px] left-0 rounded-md bg-background shadow-sm",
              indicatorClassName
            )}
          />
        )}
        {children}
      </TabsPrimitive.List>
    </TabsListContext.Provider>
  )
}

function TabsTrigger({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Trigger>) {
  return (
    <TabsPrimitive.Trigger
      data-slot="tabs-trigger"
      className={cn(
        // `relative` sans z-index : place dans le flux apres le marqueur, le
        // declencheur passe au-dessus de lui sans qu'on ait a les empiler.
        "relative inline-flex h-[calc(100%-1px)] flex-1 items-center justify-center gap-1.5 rounded-md border border-transparent px-1.5 py-0.5 text-sm font-medium whitespace-nowrap text-foreground/60 transition-colors group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 has-data-[icon=inline-end]:pr-1 has-data-[icon=inline-start]:pl-1 dark:text-muted-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        "data-active:text-foreground dark:data-active:text-foreground",
        className
      )}
      {...props}
    />
  )
}

function TabsContent({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Content>) {
  return (
    <TabsPrimitive.Content
      data-slot="tabs-content"
      className={cn("flex-1 text-sm outline-none", className)}
      {...props}
    />
  )
}

export { Tabs, TabsList, TabsTrigger, TabsContent, tabsListVariants }
