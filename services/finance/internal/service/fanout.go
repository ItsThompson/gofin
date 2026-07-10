package service

// dashboardFanoutLimit is the uniform cap on concurrent upstream reads shared by
// every fan-out path in this service (GetSpendingTrends, GetHistoricalComparison,
// and GetAllUserData). It is a domain truth, not a per-call literal, so the paths
// share one tunable bound rather than each choosing its own.
//
// The cap is sized for the pgxpool that GetAllUserData's repository reads draw
// from: its default max connections is max(4, NumCPU) and each in-flight pg read
// checks out one connection, so a wide pg fan-out could starve the pool. The
// dashboard paths fan out gRPC GetExpensesForPeriod reads (which consume no pg
// connections); they adopt the same bound for uniformity and to keep the widest
// window (the 12-month trends fan-out) from bursting the expense service. The
// value sits in the 4-6 band.
const dashboardFanoutLimit = 5
