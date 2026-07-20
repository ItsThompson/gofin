import { renderWithRouter } from "@gofin/test-utils";
import { DashboardFeature } from "../index";
import { testUser } from "./fixtures";

export function renderDashboard(user = testUser) {
  return renderWithRouter(<DashboardFeature user={user} />, { route: "/dashboard" });
}
