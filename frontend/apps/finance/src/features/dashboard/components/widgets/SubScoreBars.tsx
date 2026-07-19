import type { HealthComponent, HealthComponentKey } from "../../../../types";
import { SubScoreBar } from "./SubScoreBar";

interface SubScoreBarsProps {
  components: HealthComponent[];
  /** Key of the insight driver, highlighted in the list. */
  driverKey: HealthComponentKey | "";
}

/**
 * The contributing sub-score bars. When savings is dropped (two components) it
 * appends a note so the missing bar is not read as a bug.
 */
export function SubScoreBars({ components, driverKey }: SubScoreBarsProps) {
  const savingsDropped = !components.some(
    (component) => component.key === "savings_achievement",
  );

  return (
    <div className="space-y-1.5">
      {components.map((component) => (
        <SubScoreBar
          key={component.key}
          component={component}
          isDriver={component.key === driverKey}
        />
      ))}
      {savingsDropped && (
        <p className="px-2 text-xs text-muted-foreground">
          Savings isn&apos;t budgeted this month.
        </p>
      )}
    </div>
  );
}
