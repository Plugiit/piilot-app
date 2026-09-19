/**
 * Habillage clair des infobulles des graphiques.
 *
 * Celle du projet est sombre par defaut ; sur une carte blanche elle reprend
 * les bordures de la carte, sinon deux traitements sans rapport se repondraient
 * au meme endroit. La fleche est un carre pivote que shadcn peint en
 * `bg-foreground` : sans la reprendre, elle resterait noire sous une bulle
 * blanche. Elle se vise en descendant et non en enfant direct — Radix
 * l'enveloppe d'un span, son `ResizeObserver` mesurant mal les SVG.
 *
 * Vit dans `lib` et non dans un fichier de composant : un module qui exporte
 * autre chose que des composants perd le rechargement a chaud.
 */
export const LIGHT_TOOLTIP =
  'border border-[#ebebeb] bg-white text-[#171717] shadow-[0_4px_12px_rgb(16_24_40/0.08)] [&_svg]:bg-white [&_svg]:fill-white'
