import { ShowcaseFeature } from "./ShowcaseFeature";
import type { FeatureShowcaseContent } from "../types";

/**
 * "Feature showcase" section: an <h2> heading above a responsive grid of
 * ShowcaseFeatures (three columns on md+, one below). Pure function of its
 * content slice; the feature count is data-driven so a dropped entry surfaces
 * in tests.
 */
export function FeatureShowcaseSection({
  heading,
  features,
}: FeatureShowcaseContent) {
  return (
    <section className="mx-auto max-w-7xl px-4 py-16 md:py-24">
      <h2 className="font-heading text-2xl font-bold tracking-tight md:text-3xl">
        {heading}
      </h2>
      <div className="mt-10 grid grid-cols-1 gap-6 md:grid-cols-3">
        {features.map((feature) => (
          <ShowcaseFeature key={feature.title} {...feature} />
        ))}
      </div>
    </section>
  );
}
