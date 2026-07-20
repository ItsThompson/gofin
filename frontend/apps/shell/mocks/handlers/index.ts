import { authHandlers } from "./auth";
import { adminHandlers } from "./admin";
import { periodsHandlers } from "./periods";
import { defaultsHandlers } from "./defaults";
import { tagsHandlers } from "./tags";
import { prorataHandlers } from "./prorata";
import { dashboardHandlers } from "./dashboard";
import { healthScoreHandlers } from "./health-score";
import { trendsHandlers } from "./trends";
import { expensesHandlers } from "./expenses";
import { healthHandlers } from "./health";

/**
 * Composed MSW handler list. Order is preserved from the original single-file
 * registration: auth, admin, periods, defaults, tags, pro-rata, dashboard
 * aggregations, health-score, trends, expenses, then the health check.
 */
export const handlers = [
  ...authHandlers,
  ...adminHandlers,
  ...periodsHandlers,
  ...defaultsHandlers,
  ...tagsHandlers,
  ...prorataHandlers,
  ...dashboardHandlers,
  ...healthScoreHandlers,
  ...trendsHandlers,
  ...expensesHandlers,
  ...healthHandlers,
];
