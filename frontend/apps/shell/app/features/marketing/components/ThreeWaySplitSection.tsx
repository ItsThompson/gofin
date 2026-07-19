import { SplitBucketCard } from "./SplitBucketCard";
import type { ThreeWaySplitContent } from "../types";

/**
 * "The three-way split" section: an <h2> heading and intro above a responsive
 * grid of SplitBucketCards (three columns on md+, one below). Pure function of
 * its content slice; renders exactly one card per bucket in the content model.
 */
export function ThreeWaySplitSection({
  heading,
  intro,
  buckets,
}: ThreeWaySplitContent) {
  return (
    <section className="mx-auto max-w-7xl px-4 py-16 md:py-24">
      <h2 className="font-heading text-2xl font-bold tracking-tight md:text-3xl">
        {heading}
      </h2>
      <p className="mt-4 max-w-2xl text-lg text-muted-foreground">{intro}</p>
      <div className="mt-10 grid grid-cols-1 gap-6 md:grid-cols-3">
        {buckets.map((bucket) => (
          <SplitBucketCard key={bucket.accent} {...bucket} />
        ))}
      </div>
    </section>
  );
}
