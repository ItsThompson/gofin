import { motion } from "framer-motion";
import { LogExpenseCard } from "./LogExpenseCard";
import { DashboardPreviewCard } from "./DashboardPreviewCard";
import { usePrefersReducedMotion } from "../hooks/usePrefersReducedMotion";
import type { HeroAnimationProps } from "../types";

/** Full loop duration (seconds) for the log -> dashboard crossfade. */
const LOOP_DURATION = 6;
/** Shared keyframe times for the crossfade timeline. */
const CROSSFADE_TIMES = [0, 0.4, 0.55, 0.92, 1];

/**
 * The hero visual: a looping, animated scene that shows logging an expense and
 * then the dashboard updating. Two cards are stacked and crossfaded on an
 * infinite timeline. Marketing-only: it holds no real data and imports no
 * finance components. Under prefers-reduced-motion it renders the static
 * dashboard end state and starts no loop. The whole scene is announced to
 * assistive tech as one labelled image.
 */
export function HeroAnimation({ alt }: HeroAnimationProps) {
  const prefersReducedMotion = usePrefersReducedMotion();

  if (prefersReducedMotion) {
    return (
      <div
        role="img"
        aria-label={alt}
        className="relative aspect-[4/3] w-full"
      >
        <DashboardPreviewCard reducedMotion />
      </div>
    );
  }

  return (
    <div role="img" aria-label={alt} className="relative aspect-[4/3] w-full">
      <motion.div
        className="absolute inset-0"
        animate={{ opacity: [1, 1, 0, 0, 1] }}
        transition={{
          duration: LOOP_DURATION,
          times: CROSSFADE_TIMES,
          repeat: Infinity,
          ease: "easeInOut",
        }}
      >
        <LogExpenseCard />
      </motion.div>
      <motion.div
        className="absolute inset-0"
        animate={{ opacity: [0, 0, 1, 1, 0] }}
        transition={{
          duration: LOOP_DURATION,
          times: CROSSFADE_TIMES,
          repeat: Infinity,
          ease: "easeInOut",
        }}
      >
        <DashboardPreviewCard />
      </motion.div>
    </div>
  );
}
