import { Card } from "@gofin/ui/components/card";
import { landingIcons } from "../icons";
import type { SplitAccent, SplitBucketContent } from "../types";

/**
 * Category-accent classes for each bucket. The full class strings are listed
 * literally so Tailwind's JIT keeps them; do not build them from the accent
 * value at runtime.
 */
const ACCENT_CLASSES: Record<SplitAccent, string> = {
  essentials: "bg-essentials/10 text-essentials",
  desires: "bg-desires/10 text-desires",
  savings: "bg-savings/10 text-savings",
};

/**
 * One bucket in the three-way split: a Card with an accent-tinted icon circle,
 * an <h3> title, and a body paragraph. The icon is decorative (aria-hidden);
 * its meaning is carried by the adjacent text. Icon resolution mirrors StepCard.
 */
export function SplitBucketCard({ accent, icon, title, body }: SplitBucketContent) {
  const Icon = landingIcons[icon];

  return (
    <Card className="gap-3 p-6">
      <div
        className={`flex size-12 items-center justify-center rounded-full ${ACCENT_CLASSES[accent]}`}
      >
        <Icon aria-hidden={true} className="size-6" />
      </div>
      <h3 className="font-heading text-lg font-medium">{title}</h3>
      <p className="text-sm text-muted-foreground">{body}</p>
    </Card>
  );
}
