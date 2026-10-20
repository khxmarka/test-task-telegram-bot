package service

import (
	"context"
	"fmt"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

type ListResult struct {
	Items  []domain.Expense `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func (s *ExpenseService) List(ctx context.Context, userID int64, f domain.ExpenseFilter) (*ListResult, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("%w: user_id required", errorsx.ErrValidation)
	}
	f.UserID = &userID
	if f.Limit <= 0 {
		f.Limit = 10
	}
	if f.Limit > 50 {
		f.Limit = 50
	}
	items, err := s.expenses.List(ctx, f)
	if err != nil {
		return nil, err
	}
	total, err := s.expenses.Count(ctx, f)
	if err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *ExpenseService) ListFamily(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error) {
	return s.expenses.ListFamily(ctx, f)
}

func (s *ExpenseService) ListMembers(ctx context.Context) ([]domain.User, error) {
	return s.users.ListMembers(ctx)
}
