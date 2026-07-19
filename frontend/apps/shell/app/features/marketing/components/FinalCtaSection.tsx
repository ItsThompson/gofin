import { Link } from "react-router";
import { Button } from "@gofin/ui/components/button";
import { Card } from "@gofin/ui/components/card";
import type { FinalCtaContent } from "../types";

/**
 * Closing call-to-action band: a centered Card holding the section <h2> and the
 * primary CTA. The CTA reuses the hero's pattern (default variant, size="lg",
 * asChild around a real Link) so the two primary actions are visually identical.
 */
export function FinalCtaSection({ heading, primaryCta }: FinalCtaContent) {
  return (
    <section className="mx-auto max-w-7xl px-4 py-16 md:py-24">
      <Card className="items-center gap-6 px-6 py-12 text-center md:py-16">
        <h2 className="font-heading text-2xl font-bold tracking-tight md:text-3xl">
          {heading}
        </h2>
        <Button asChild size="lg">
          <Link to={primaryCta.href}>{primaryCta.label}</Link>
        </Button>
      </Card>
    </section>
  );
}
