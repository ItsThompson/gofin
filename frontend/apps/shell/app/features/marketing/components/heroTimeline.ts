/**
 * Shared timing for the looping hero scene. HeroAnimation (the log -> dashboard
 * crossfade) and DashboardPreviewCard (its category bars and monthly total) run
 * on this single period so they stay phase-locked: the dashboard's contents fill
 * exactly when the card is revealed, hold through the dwell, then reset during
 * the hidden phase where the jump is invisible.
 *
 * The timeline is defined in seconds; every framer-motion `times` array is a
 * fraction of the loop, so changing one phase keeps all keyframes in sync.
 *
 * Consumers MUST animate these tracks with `ease: "linear"`. framer-motion warps
 * a non-linear scalar ease across the whole `times` timeline (compressing holds,
 * skewing fades), so a non-linear ease would break the exact second-based timing
 * this module encodes.
 */

/** Log-expense scene fully visible. */
const LOG_HOLD_SECONDS = 2.4;
/** One crossfade, applied symmetrically in both directions. */
const FADE_SECONDS = 0.9;
/** Dashboard fully visible before the loop restarts. */
const DASH_HOLD_SECONDS = 3;
/** Time a single category bar takes to fill left to right. */
const BAR_FILL_SECONDS = 1.1;
/** Stagger between successive category bars starting to fill. */
const BAR_STAGGER_SECONDS = 0.15;
/** Time the monthly total takes to rise into place once revealed. */
const TOTAL_RISE_SECONDS = 0.5;

/** Full loop length. Every keyframe time below is a fraction of this. */
export const LOOP_DURATION =
  LOG_HOLD_SECONDS + FADE_SECONDS + DASH_HOLD_SECONDS + FADE_SECONDS;

/** Seconds elapsed at the moment the dashboard becomes fully opaque. */
const DASH_REVEALED_SECONDS = LOG_HOLD_SECONDS + FADE_SECONDS;
/** Seconds elapsed at the moment the dashboard starts fading back out. */
const DASH_DWELL_END_SECONDS = DASH_REVEALED_SECONDS + DASH_HOLD_SECONDS;

const fraction = (seconds: number): number => seconds / LOOP_DURATION;

/**
 * Keyframe times shared by both crossfading layers: loop start, log fade-out
 * start, dashboard revealed, dashboard fade-out start, loop end.
 */
export const CROSSFADE_TIMES = [
  0,
  fraction(LOG_HOLD_SECONDS),
  fraction(DASH_REVEALED_SECONDS),
  fraction(DASH_DWELL_END_SECONDS),
  1,
];

/** Log card opacity across CROSSFADE_TIMES: visible, fade out, hidden, fade in. */
export const LOG_OPACITY_KEYFRAMES = [1, 1, 0, 0, 1];
/** Dashboard card opacity across CROSSFADE_TIMES: hidden, fade in, visible, fade out. */
export const DASH_OPACITY_KEYFRAMES = [0, 0, 1, 1, 0];

/** A scaleX keyframe track for one category bar, aligned to the loop. */
export interface BarFillAnimation {
  values: number[];
  times: number[];
}

/**
 * scaleX track for the category bar at `index`: empty until the dashboard is
 * revealed, fills left to right (staggered per bar), then holds full through the
 * dwell. Snaps back to empty at the loop wrap, which lands while the card is
 * hidden.
 */
export function barFillKeyframes(index: number): BarFillAnimation {
  const startSeconds = DASH_REVEALED_SECONDS + index * BAR_STAGGER_SECONDS;
  const endSeconds = startSeconds + BAR_FILL_SECONDS;
  return {
    values: [0, 0, 1, 1],
    times: [0, fraction(startSeconds), fraction(endSeconds), 1],
  };
}

/** Opacity/y track for the monthly total, aligned to the loop. */
export interface TotalRiseAnimation {
  opacity: number[];
  y: number[];
  times: number[];
}

/**
 * The monthly total rises into place as the dashboard is revealed, holds through
 * the dwell, then resets during the hidden phase.
 */
export const totalRiseKeyframes: TotalRiseAnimation = {
  opacity: [0, 0, 1, 1],
  y: [6, 6, 0, 0],
  times: [
    0,
    fraction(DASH_REVEALED_SECONDS),
    fraction(DASH_REVEALED_SECONDS + TOTAL_RISE_SECONDS),
    1,
  ],
};

/**
 * repeatDelay that makes a `repeatType: "reverse"` animation of the given segment
 * duration complete one full there-and-back cycle per loop, keeping it
 * phase-locked to the crossfade instead of drifting on its own period.
 */
export function reverseCycleRepeatDelay(segmentDuration: number): number {
  return LOOP_DURATION / 2 - segmentDuration;
}

/**
 * repeatDelay that makes a looping (`repeatType: "loop"`) animation of the given
 * segment duration complete exactly one iteration per loop, keeping it
 * phase-locked to the crossfade.
 */
export function loopCycleRepeatDelay(segmentDuration: number): number {
  return LOOP_DURATION - segmentDuration;
}
