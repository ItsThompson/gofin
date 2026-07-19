import { FaqItem } from "./FaqItem";
import type { FaqContent } from "../types";

/**
 * FAQ section: an <h2> heading above a divided list of FaqItems, one per entry
 * in content.faq.items. Purely data-driven: adding or removing a question is a
 * content.ts edit and requires no change here. Static list, no accordion (§09).
 */
export function FaqSection({ heading, items }: FaqContent) {
  return (
    <section className="mx-auto max-w-3xl px-4 py-16 md:py-24">
      <h2 className="font-heading text-2xl font-bold tracking-tight md:text-3xl">
        {heading}
      </h2>
      <div className="mt-8">
        {items.map((item) => (
          <FaqItem key={item.question} {...item} />
        ))}
      </div>
    </section>
  );
}
