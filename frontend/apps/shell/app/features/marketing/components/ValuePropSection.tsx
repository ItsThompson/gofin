import { Card } from "@gofin/ui/components/card";
import type { ValuePropContent } from "../types";

/**
 * Value-proposition band: an accent Card carrying the large quote beside a
 * supporting body paragraph and a smaller muted footnote. Two columns on md+
 * (quote left, supporting copy right), stacked with the quote first below md.
 * The accent uses the brand `primary` token, not a hardcoded color.
 *
 * The band has no heading of its own, so the <section> is named by its quote
 * (aria-labelledby) to expose it to assistive tech as an identifiable region.
 */
export function ValuePropSection({ quote, body, footnote }: ValuePropContent) {
  return (
    <section
      aria-labelledby="value-prop-quote"
      className="mx-auto flex max-w-7xl flex-col items-stretch gap-8 px-4 py-16 md:flex-row md:items-center md:py-24"
    >
      <Card className="flex-1 bg-primary p-8 text-primary-foreground md:p-10">
        <p
          id="value-prop-quote"
          className="font-heading text-2xl font-semibold tracking-tight md:text-3xl"
        >
          {quote}
        </p>
      </Card>
      <div className="flex flex-1 flex-col gap-4">
        <p className="text-lg text-foreground">{body}</p>
        <p className="text-sm text-muted-foreground">{footnote}</p>
      </div>
    </section>
  );
}
