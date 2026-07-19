import { StepCard } from "./StepCard";
import type { HowItWorksContent } from "../types";

/**
 * "How it works" section: an <h2> heading and a responsive grid of StepCards
 * (three columns on md+, one column below). Pure function of its content slice;
 * renders exactly one card per step in the content model.
 */
export function HowItWorksSection({ heading, steps }: HowItWorksContent) {
  return (
    <section className="mx-auto max-w-7xl px-4 py-16 md:py-24">
      <h2 className="font-heading text-2xl font-bold tracking-tight md:text-3xl">
        {heading}
      </h2>
      <div className="mt-10 grid grid-cols-1 gap-6 md:grid-cols-3">
        {steps.map((step) => (
          <StepCard key={step.ordinal} {...step} />
        ))}
      </div>
    </section>
  );
}
