import type { HeroVisualProps } from "../types";

/**
 * Placeholder product illustration for the hero. Renders three token-colored
 * bars standing in for the essentials / desires / savings split until real
 * artwork lands. Announced to assistive tech as a single labelled image.
 */
export function HeroVisual({ alt }: HeroVisualProps) {
  return (
    <div
      role="img"
      aria-label={alt}
      className="flex aspect-[4/3] w-full items-end gap-3 rounded-xl bg-muted p-6 ring-1 ring-foreground/10"
    >
      <div className="h-[70%] flex-1 rounded-lg bg-essentials" />
      <div className="h-[45%] flex-1 rounded-lg bg-desires" />
      <div className="h-[30%] flex-1 rounded-lg bg-savings" />
    </div>
  );
}
