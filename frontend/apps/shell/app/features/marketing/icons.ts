import {
  Receipt,
  ChartPie,
  Target,
  Wallet,
  ChartLine,
  Gauge,
  CalendarClock,
  House,
  Sparkles,
  PiggyBank,
  type LucideIcon,
} from "lucide-react";
import type { LandingIcon } from "./types";

/**
 * Maps each serializable LandingIcon key to its lucide-react component. This is
 * the single place the icon dependency is resolved, so content.ts stays plain
 * data. (lucide v1 renamed some glyphs: pie/line charts are ChartPie/ChartLine
 * and the home glyph is House; the string keys stay stable.)
 */
export const landingIcons: Record<LandingIcon, LucideIcon> = {
  receipt: Receipt,
  pieChart: ChartPie,
  target: Target,
  wallet: Wallet,
  lineChart: ChartLine,
  gauge: Gauge,
  calendarClock: CalendarClock,
  house: House,
  sparkles: Sparkles,
  piggyBank: PiggyBank,
};
