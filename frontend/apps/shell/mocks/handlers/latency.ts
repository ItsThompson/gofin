import { delay } from "msw";

/** Simulates realistic network latency. */
export async function simulateLatency(): Promise<void> {
  await delay(100 + Math.random() * 200);
}
