package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

// bulkDeletePage is how many own rows we load per round-trip.
// Confirm always names the real Count; leftover is never left silently.
var bulkDeletePage = 500

func (s *ExpenseService) Delete(ctx context.Context, id, userID int64) error {
	if id <= 0 || userID <= 0 {
		return fmt.Errorf("%w: id and user_id required", errorsx.ErrValidation)
	}
	e, err := s.expenses.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if e.UserID != userID {
		return errorsx.ErrForbidden
	}
	return s.expenses.Delete(ctx, id, userID)
}

// FindForBulkDelete returns one page of the user's own expenses matching f.
func (s *ExpenseService) FindForBulkDelete(ctx context.Context, userID int64, f domain.ExpenseFilter) ([]domain.Expense, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("%w: user_id required", errorsx.ErrValidation)
	}
	f.UserID = &userID
	if bulkDeletePage < 1 {
		f.Limit = 500
	} else {
		f.Limit = bulkDeletePage
	}
	f.Offset = 0
	return s.expenses.List(ctx, f)
}

// DeleteMany deletes the given ids one by one, each scoped to userID.
func (s *ExpenseService) DeleteMany(ctx context.Context, userID int64, ids []int64) (deleted int, err error) {
	for _, id := range ids {
		if err := s.Delete(ctx, id, userID); err != nil {
			if errors.Is(err, errorsx.ErrNotFound) {
				continue
			}
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

// DeleteMatching pages through the user's own rows matching f until none remain.
func (s *ExpenseService) DeleteMatching(ctx context.Context, userID int64, f domain.ExpenseFilter) (deleted int, err error) {
	if userID <= 0 {
		return 0, fmt.Errorf("%w: user_id required", errorsx.ErrValidation)
	}
	f.UserID = &userID
	page := bulkDeletePage
	if page < 1 {
		page = 500
	}
	for round := 0; round < 40; round++ {
		f.Limit = page
		f.Offset = 0
		items, err := s.expenses.List(ctx, f)
		if err != nil {
			return deleted, err
		}
		if len(items) == 0 {
			return deleted, nil
		}
		ids := make([]int64, len(items))
		for i, e := range items {
			ids[i] = e.ID
		}
		n, err := s.DeleteMany(ctx, userID, ids)
		deleted += n
		if err != nil {
			return deleted, err
		}
		if n == 0 {
			return deleted, nil
		}
	}
	return deleted, fmt.Errorf("%w: bulk delete stopped after too many pages", errorsx.ErrValidation)
}
