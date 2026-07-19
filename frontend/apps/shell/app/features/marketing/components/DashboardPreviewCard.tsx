import { motion } from "framer-motion";

interface DashboardPreviewCardProps {
  /** When true, render the final state with no animation (reduced-motion path). */
  reducedMotion?: boolean;
}

/** The three category bars shown in the dashboard preview, using category tokens. */
const CATEGORY_BARS = [
  { label: "Essentials", token: "bg-essentials", width: "72%" },
  { label: "Desires", token: "bg-desires", width: "45%" },
  { label: "Savings", token: "bg-savings", width: "30%" },
] as const;

/**
 * Scene 3 of the hero: a stylized dashboard preview showing a monthly total and
 * the three category bars filling left to right. Marketing-only, no real data.
 * With reducedMotion the bars render at their final width and the numbers show
 * their end value, which doubles as the static end state and the tested markup.
 */
export function DashboardPreviewCard({ reducedMotion }: DashboardPreviewCardProps) {
  return (
    <div className="flex h-full w-full flex-col gap-4 rounded-xl bg-card p-6 text-card-foreground ring-1 ring-foreground/10">
      <div className="flex items-baseline justify-between">
        <span className="text-sm font-medium text-muted-foreground">
          This month
        </span>
        <motion.span
          className="font-heading text-2xl font-bold"
          initial={reducedMotion ? false : { opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={reducedMotion ? undefined : { duration: 0.5, delay: 0.2 }}
        >
          $1,240
        </motion.span>
      </div>

      <div className="flex flex-col gap-3">
        {CATEGORY_BARS.map((bar, index) => (
          <div key={bar.label} className="flex flex-col gap-1">
            <span className="text-xs font-medium text-muted-foreground">
              {bar.label}
            </span>
            <div className="h-2.5 w-full overflow-hidden rounded-full bg-muted">
              <motion.div
                className={`h-full rounded-full ${bar.token}`}
                style={{ width: bar.width, transformOrigin: "left" }}
                initial={reducedMotion ? false : { scaleX: 0 }}
                animate={{ scaleX: 1 }}
                transition={
                  reducedMotion
                    ? undefined
                    : {
                        duration: 1.1,
                        delay: 0.3 + index * 0.15,
                        repeat: Infinity,
                        repeatType: "reverse",
                        repeatDelay: 1.6,
                      }
                }
              />
            </div>
          </div>
        ))}
      </div>

      <p className="mt-auto text-xs font-medium text-muted-foreground">
        Expense logged
      </p>
    </div>
  );
}
