package service

// dashboardFanoutLimit is the uniform cap on concurrent upstream reads shared by
// every fan-out path in this service, sized for the pgxpool that repository reads
// draw from so a wide fan-out cannot starve the pool.
const dashboardFanoutLimit = 5
