package service

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/shared/validator"
)

// ExpenseRevision identifies the current ledger generation for one user.
type ExpenseRevision struct {
	Epoch    string
	Revision uint64
}

type expenseRevisionOwner struct {
	mu        sync.Mutex
	epoch     string
	next      uint64
	revisions map[string]uint64
}

func newExpenseRevisionOwner() *expenseRevisionOwner {
	return &expenseRevisionOwner{
		epoch:     uuid.NewString(),
		revisions: make(map[string]uint64),
	}
}

func (o *expenseRevisionOwner) observe(userID string) ExpenseRevision {
	o.mu.Lock()
	defer o.mu.Unlock()

	if revision, ok := o.revisions[userID]; ok {
		return ExpenseRevision{Epoch: o.epoch, Revision: revision}
	}
	return ExpenseRevision{Epoch: o.epoch, Revision: o.advanceLocked(userID)}
}

func (o *expenseRevisionOwner) advance(userID string) ExpenseRevision {
	o.mu.Lock()
	defer o.mu.Unlock()

	return ExpenseRevision{Epoch: o.epoch, Revision: o.advanceLocked(userID)}
}

func (o *expenseRevisionOwner) retire(userID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	delete(o.revisions, userID)
}

func (o *expenseRevisionOwner) advanceLocked(userID string) uint64 {
	if o.next == ^uint64(0) {
		o.epoch = uuid.NewString()
		o.next = 0
		o.revisions = make(map[string]uint64)
	}
	o.next++
	o.revisions[userID] = o.next
	return o.next
}

func (s *ExpenseService) invalidateUser(userID string) {
	s.readCaches.purgeUser(userID)
	s.revisionOwner.advance(userID)
}

func (s *ExpenseService) retireUser(userID string) {
	s.readCaches.purgeUser(userID)
	s.revisionOwner.advance(userID)
	s.revisionOwner.retire(userID)
}

// GetExpenseRevision returns the current process epoch and user revision for
// internal freshness validation. It only reads process-local state.
func (s *ExpenseService) GetExpenseRevision(_ context.Context, userID string) (*ExpenseRevision, error) {
	v := validator.New()
	v.Check(userID != "", "userId", "user_id is required")
	if v.HasErrors() {
		return nil, apierr.Validation("validation failed", v.Errors())
	}

	revision := s.revisionOwner.observe(userID)
	return &revision, nil
}
