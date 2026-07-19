import type { HealthComponent } from "../../../../types";
import { COMPONENT_LABEL } from "./healthScoreDisplay";

interface SubScoreBarProps {
  component: HealthComponent;
  /** Highlights the row when this component is the insight driver. */
  isDriver: boolean;
}

/** One sub-score row: label, score/max, a progress bar, and the detail line. */
export function SubScoreBar({ component, isDriver }: SubScoreBarProps) {
  const label = COMPONENT_LABEL[component.key];
  const fillPercent =
    component.max > 0
      ? Math.min(100, Math.round((component.score / component.max) * 100))
      : 0;

  return (
    <div
      className={`rounded-lg p-2 ${isDriver ? "bg-muted/50 ring-1 ring-primary/40" : ""}`}
      data-testid={`sub-score-${component.key}`}
    >
      <div className="flex items-center justify-between text-sm">
        <span className="font-medium">{label}</span>
        <span className="text-xs font-semibold text-muted-foreground">
          {component.score}/{component.max}
        </span>
      </div>
      <div className="mt-1 h-2 w-full overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-all"
          style={{ width: `${fillPercent}%` }}
          role="progressbar"
          aria-valuenow={component.score}
          aria-valuemin={0}
          aria-valuemax={component.max}
          aria-label={`${label} sub-score`}
        />
      </div>
      <p className="mt-1 text-xs text-muted-foreground">{component.detail}</p>
    </div>
  );
}
