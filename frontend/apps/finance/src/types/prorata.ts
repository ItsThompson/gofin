/** Pro-rata schedule record from the finance service. */
export interface ProRataSchedule {
  id: string;
  userId: string;
  proRataGroup: string;
  name: string;
  /** Installment amount in minor units (cents). */
  amount: number;
  currency: string;
  expenseType: "essentials" | "desires" | "savings";
  tagId: string;
  targetYear: number;
  targetMonth: number;
  installmentIndex: number;
  installmentTotal: number;
  status: "pending" | "applied";
  createdAt: string;
  appliedAt: string | null;
}

/** Request body for POST /api/finance/prorata. */
export interface CreateProRataRequest {
  name: string;
  /** Total amount in minor units (cents). */
  totalAmount: number;
  currency: string;
  expenseType: "essentials" | "desires" | "savings";
  tagId: string;
  /** ISO date string (YYYY-MM-DD). */
  expenseDate: string;
  /** Number of months to spread over (minimum 2). */
  months: number;
}

/** Response from POST /api/finance/prorata. */
export interface ProRataResponse {
  expense: {
    id: string;
    name: string;
    amount: number;
    currency: string;
    expenseType: string;
    tagId: string;
    expenseDate: string;
    periodYear: number;
    periodMonth: number;
    isProRata: boolean;
    proRataGroup: string;
    proRataIndex: number;
    proRataTotal: number;
    createdAt: string;
  };
  schedules: ProRataSchedule[];
}

/** Response from GET /api/finance/prorata/upcoming. */
export interface UpcomingProRataResponse {
  schedules: ProRataSchedule[];
}
