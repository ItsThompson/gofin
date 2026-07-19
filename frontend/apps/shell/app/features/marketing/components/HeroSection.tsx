import { Link } from "react-router";
import { Button } from "@gofin/ui/components/button";
import { HeroVisual } from "./HeroVisual";
import type { HeroContent } from "../types";

/**
 * Hero: two-column on md+ (copy left, visual right), stacked with copy first
 * below md. Renders the single page <h1>, the subheading, the primary CTA as a
 * real link, the CTA microcopy, and the placeholder HeroVisual.
 */
export function HeroSection({
  heading,
  subheading,
  primaryCta,
  ctaFootnote,
  visualAlt,
}: HeroContent) {
  return (
    <section className="mx-auto grid max-w-7xl grid-cols-1 items-center gap-10 px-4 py-16 md:grid-cols-2 md:py-24">
      <div className="flex flex-col gap-6">
        <h1 className="font-heading text-4xl font-bold tracking-tight md:text-5xl">
          {heading}
        </h1>
        <p className="text-lg text-muted-foreground">{subheading}</p>
        <div className="flex flex-col gap-2">
          <div>
            <Button asChild size="lg">
              <Link to={primaryCta.href}>{primaryCta.label}</Link>
            </Button>
          </div>
          <p className="text-sm text-muted-foreground">{ctaFootnote}</p>
        </div>
      </div>
      <HeroVisual alt={visualAlt} />
    </section>
  );
}
