import { describe, it, expect } from "vitest";
import {
  LOOP_DURATION,
  CROSSFADE_TIMES,
  LOG_OPACITY_KEYFRAMES,
  DASH_OPACITY_KEYFRAMES,
  barFillKeyframes,
  totalRiseKeyframes,
  reverseCycleRepeatDelay,
  loopCycleRepeatDelay,
} from "../components/heroTimeline";

/** Seconds a keyframe fraction represents on the full loop. */
const seconds = (fraction: number): number => fraction * LOOP_DURATION;
/** Duration in seconds of the phase between two adjacent CROSSFADE_TIMES. */
const phaseSeconds = (from: number, to: number): number =>
  seconds(CROSSFADE_TIMES[to] - CROSSFADE_TIMES[from]);

describe("heroTimeline crossfade", () => {
  it("holds the log scene, then the dashboard, for their configured durations", () => {
    expect(LOOP_DURATION).toBeCloseTo(7.2);
    expect(phaseSeconds(0, 1)).toBeCloseTo(2.4); // log-expense hold
    expect(phaseSeconds(2, 3)).toBeCloseTo(3); // dashboard dwell
  });

  it("uses symmetric 0.9s fades in both directions", () => {
    const fadeIn = phaseSeconds(1, 2);
    const fadeOut = phaseSeconds(3, 4);
    expect(fadeIn).toBeCloseTo(0.9);
    expect(fadeOut).toBeCloseTo(0.9);
    expect(fadeIn).toBeCloseTo(fadeOut);
  });

  it("spans the full loop with strictly increasing fractions", () => {
    expect(CROSSFADE_TIMES[0]).toBe(0);
    expect(CROSSFADE_TIMES[CROSSFADE_TIMES.length - 1]).toBe(1);
    for (let i = 1; i < CROSSFADE_TIMES.length; i++) {
      expect(CROSSFADE_TIMES[i]).toBeGreaterThan(CROSSFADE_TIMES[i - 1]);
    }
  });

  it("pairs one opacity keyframe with each crossfade time", () => {
    expect(LOG_OPACITY_KEYFRAMES).toHaveLength(CROSSFADE_TIMES.length);
    expect(DASH_OPACITY_KEYFRAMES).toHaveLength(CROSSFADE_TIMES.length);
    // The two layers are opposites: when the log is visible the dashboard is not.
    LOG_OPACITY_KEYFRAMES.forEach((logOpacity, i) => {
      expect(logOpacity + DASH_OPACITY_KEYFRAMES[i]).toBe(1);
    });
  });
});

describe("heroTimeline bar fill", () => {
  const revealFraction = CROSSFADE_TIMES[2];
  const dwellEndFraction = CROSSFADE_TIMES[3];

  it.each([0, 1, 2])("fills bar %i within the dashboard dwell", (index) => {
    const { values, times } = barFillKeyframes(index);

    expect(values).toEqual([0, 0, 1, 1]);

    const [start, fillStart, fillEnd, end] = times;
    expect(start).toBe(0);
    expect(end).toBe(1);
    // Empty until the dashboard is fully revealed, filled before the dwell ends.
    expect(fillStart).toBeGreaterThanOrEqual(revealFraction);
    expect(fillEnd).toBeGreaterThan(fillStart);
    expect(fillEnd).toBeLessThan(dwellEndFraction);
  });

  it("staggers later bars to start filling after earlier ones", () => {
    const first = barFillKeyframes(0).times[1];
    const second = barFillKeyframes(1).times[1];
    const third = barFillKeyframes(2).times[1];
    expect(second).toBeGreaterThan(first);
    expect(third).toBeGreaterThan(second);
  });
});

describe("heroTimeline total rise", () => {
  it("rises into place within the dashboard dwell and holds", () => {
    const { opacity, y, times } = totalRiseKeyframes;

    expect(opacity).toEqual([0, 0, 1, 1]);
    expect(y).toEqual([6, 6, 0, 0]);

    const [start, riseStart, riseEnd, end] = times;
    expect(start).toBe(0);
    expect(end).toBe(1);
    expect(riseStart).toBeCloseTo(CROSSFADE_TIMES[2]); // begins as the card is revealed
    expect(riseEnd).toBeGreaterThan(riseStart);
    expect(riseEnd).toBeLessThan(CROSSFADE_TIMES[3]); // settled before the dwell ends
  });
});

describe("heroTimeline phase-lock repeat delays", () => {
  it.each([0.4, 0.6, 1.1])(
    "makes a reverse there-and-back cycle of duration %d span exactly one loop",
    (duration) => {
      const fullCycle = 2 * (duration + reverseCycleRepeatDelay(duration));
      expect(fullCycle).toBeCloseTo(LOOP_DURATION);
    },
  );

  it.each([0.6, 0.4, 2])(
    "makes a single looping iteration of duration %d span exactly one loop",
    (duration) => {
      const iteration = duration + loopCycleRepeatDelay(duration);
      expect(iteration).toBeCloseTo(LOOP_DURATION);
    },
  );
});
