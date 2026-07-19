import { FeatureColumn } from "./FeatureColumn";
import type { DualModeContent } from "../types";

/**
 * Dual-mode split: an <h2> heading above the two feature columns, side by side
 * on md+ and stacked on mobile. A pure function of its content slice; the
 * column count is data-driven so a dropped content entry surfaces in tests.
 */
export function DualModeSection({ heading, columns }: DualModeContent) {
  return (
    <section className="mx-auto max-w-7xl px-4 py-16 md:py-24">
      <h2 className="font-heading text-2xl font-bold tracking-tight">
        {heading}
      </h2>
      <div className="mt-10 grid grid-cols-1 gap-6 md:grid-cols-2">
        {columns.map((column) => (
          <FeatureColumn key={column.title} {...column} />
        ))}
      </div>
    </section>
  );
}
