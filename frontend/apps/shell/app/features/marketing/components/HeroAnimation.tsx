import { motion } from "framer-motion";
import { LogExpenseCard } from "./LogExpenseCard";
import { DashboardPreviewCard } from "./DashboardPreviewCard";
import {
  LOOP_DURATION,
  CROSSFADE_TIMES,
  LOG_OPACITY_KEYFRAMES,
  DASH_OPACITY_KEYFRAMES,
} from "./heroTimeline";
import { usePrefersReducedMotion } from "../hooks/usePrefersReducedMotion";
import type { HeroAnimationProps } from "../types";

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
        animate={{ opacity: LOG_OPACITY_KEYFRAMES }}
        transition={{
          duration: LOOP_DURATION,
          times: CROSSFADE_TIMES,
          repeat: Infinity,
          ease: "linear",
        }}
      >
        <LogExpenseCard />
      </motion.div>
      <motion.div
        className="absolute inset-0"
        animate={{ opacity: DASH_OPACITY_KEYFRAMES }}
        transition={{
          duration: LOOP_DURATION,
          times: CROSSFADE_TIMES,
          repeat: Infinity,
          ease: "linear",
        }}
      >
        <DashboardPreviewCard />
      </motion.div>
    </div>
  );
}
