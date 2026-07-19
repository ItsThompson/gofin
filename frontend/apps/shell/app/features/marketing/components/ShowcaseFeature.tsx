import { Card, CardContent } from "@gofin/ui/components/card";
import { landingIcons } from "../icons";
import type { ShowcaseFeatureContent } from "../types";

/**
 * One feature-showcase entry: a Card with a token-circled icon, an <h3> title,
 * and a body paragraph. Text + icon only (no mock UI), to avoid visual overload
 * next to the animated hero. The icon is decorative (aria-hidden); icon
 * resolution mirrors FeatureColumn.
 */
export function ShowcaseFeature({ icon, title, body }: ShowcaseFeatureContent) {
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
