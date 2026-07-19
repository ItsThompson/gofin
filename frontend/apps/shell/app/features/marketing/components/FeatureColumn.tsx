import { Card, CardContent } from "@gofin/ui/components/card";
import { landingIcons } from "../icons";
import type { FeatureColumnContent } from "../types";

/**
 * One dual-mode column: a Card with a token-circled icon, an <h3> title, and a
 * body paragraph. The icon is decorative (aria-hidden); its meaning is carried
 * by the adjacent title and body text. Icon resolution mirrors StepCard.
 */
export function FeatureColumn({ icon, title, body }: FeatureColumnContent) {
  const Icon = landingIcons[icon];
  return (
    <Card>
      <CardContent className="flex flex-col gap-4">
        <div className="flex size-12 items-center justify-center rounded-full bg-muted">
          <Icon aria-hidden className="size-6" />
        </div>
        <h3 className="font-heading text-lg font-medium">{title}</h3>
        <p className="text-muted-foreground">{body}</p>
      </CardContent>
    </Card>
  );
}
