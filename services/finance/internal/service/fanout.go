package service

import (
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/serverkit"
)

// dashboardFanoutLimit is the uniform cap on concurrent upstream reads shared by
// every fan-out path in this service, sized for the pgxpool that repository reads
// draw from so a wide fan-out cannot starve the pool.
const dashboardFanoutLimit = 5

// guardFanout wraps an errgroup task so a panic inside it becomes an error the
// enclosing g.Wait() already handles.
//
// recover() does not cross goroutines and errgroup deliberately does not recover,
// so the recoveries serverkit installs cannot see a panic here: they wrap the
// handler goroutine, and every task below runs on its own. Unguarded, one nil
// dereference in a fan-out read takes the whole finance process down, while the
// same dereference on the handler goroutine would return a 500.
//
// task names the work in the record and in the synthesized error. Neither
// reaches the client: the REST respondError path and every gRPC handler map an
// unclassified error to a generic 500 or codes.Internal.
func (s *FinanceService) guardFanout(task, userID string, run func() error) func() error {
	return func() (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				serverkit.LogRecoveredPanic(s.logger, "recovered panic in finance fan-out", recovered,
					slog.String("task", task),
					slog.String("user_id", userID),
				)
				err = fmt.Errorf("%s failed unexpectedly", task)
			}
		}()

		return run()
	}
}
