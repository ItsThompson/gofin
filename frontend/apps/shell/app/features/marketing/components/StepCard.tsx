import { Card } from "@gofin/ui/components/card";
import { landingIcons } from "../icons";
import type { StepContent } from "../types";

/**
 * One "How it works" step: a zero-padded ordinal eyebrow and its icon (in a
 * muted circle) on top, then an <h3> title and a body paragraph. The icon is
 * decorative (aria-hidden): its meaning is carried by the adjacent text. The
 * icon is resolved from the content model's string key via landingIcons.
 */
export function StepCard({ ordinal, icon, title, body }: StepContent) {
  const Icon = landingIcons[icon];

  return (
    <Card className="gap-3 p-6">
      <div className="flex items-center gap-3">
        <div className="flex size-10 items-center justify-center rounded-full bg-muted">
          <Icon aria-hidden={true} className="size-5" />
        </div>
        <span className="text-sm font-medium text-muted-foreground">
          {ordinal}
        </span>
      </div>
      <h3 className="font-heading text-lg font-medium">{title}</h3>
      <p className="text-sm text-muted-foreground">{body}</p>
    </Card>
  );
}
