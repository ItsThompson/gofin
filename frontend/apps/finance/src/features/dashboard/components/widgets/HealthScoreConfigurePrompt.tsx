import { Link } from "react-router";
import { HeartPulse } from "lucide-react";
import { Button } from "@gofin/ui/components/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@gofin/ui/components/card";

/**
 * Shown when the period has no budget configured. Prompts the user to set a
 * budget instead of rendering an empty or misleading score.
 */
export function HealthScoreConfigurePrompt() {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-base">Financial Health</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col items-center gap-3 py-6 text-center">
        <HeartPulse className="size-8 text-muted-foreground/50" aria-hidden="true" />
        <p className="text-sm text-muted-foreground">
          Set a budget to see your health score.
        </p>
        <Button asChild variant="outline" size="sm">
          <Link to="/settings">Budget Settings</Link>
        </Button>
      </CardContent>
    </Card>
  );
}
