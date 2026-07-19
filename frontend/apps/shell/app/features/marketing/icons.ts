import {
  Receipt,
  ChartPie,
  Target,
  Wallet,
  ChartLine,
  type LucideIcon,
} from "lucide-react";
import type { LandingIcon } from "./types";

/**
 * Maps each serializable LandingIcon key to its lucide-react component. This is
 * the single place the icon dependency is resolved, so content.ts stays plain
 * data. (lucide v1 exposes the pie/line chart glyphs as ChartPie/ChartLine.)
 */
export const landingIcons: Record<LandingIcon, LucideIcon> = {
  receipt: Receipt,
  pieChart: ChartPie,
  target: Target,
  wallet: Wallet,
  lineChart: ChartLine,
};
