package service

// dashboardFanoutLimit bounds the number of concurrent upstream reads issued by
// the dashboard fan-out paths (GetSpendingTrends and GetHistoricalComparison
// today; GetAllUserData later). It is a domain truth shared across those paths,
// not a per-call literal.
//
// The bound protects the pgxpool: its default max connections is max(4, NumCPU)
// and each in-flight pg read checks out one connection, so an unbounded wide
// fan-out (e.g. the 12-month trends window) could momentarily starve the pool.
// gRPC reads to the expense service do not consume pg connections, but the same
// bound is applied uniformly for simplicity. The value sits in the 4-6 band.
const dashboardFanoutLimit = 5
