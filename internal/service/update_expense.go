package service

import (
	"context"
	"fmt"
	"time"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

type UpdateExpenseInput struct {
	ID               int64
	UserID           int64
	AmountBYNKopecks *int64
	Category         *domain.Category
	SpentAt          *time.Time
	Comment          *string
}

func (s *ExpenseService) Update(ctx context.Context, in UpdateExpenseInput) (*domain.Expense, error) {
	if in.ID <= 0 || in.UserID <= 0 {
		return nil, fmt.Errorf("%w: id and user_id required", errorsx.ErrValidation)
	}
	e, err := s.expenses.GetByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if e.UserID != in.UserID {
		return nil, errorsx.ErrForbidden
	}
	if in.AmountBYNKopecks != nil {
		if *in.AmountBYNKopecks <= 0 {
			return nil, fmt.Errorf("%w: amount must be > 0", errorsx.ErrValidation)
		}
		e.AmountBYNKopecks = *in.AmountBYNKopecks
	}
	if in.Category != nil {
		if !in.Category.Valid() {
			return nil, fmt.Errorf("%w: invalid category", errorsx.ErrValidation)
		}
		e.Category = *in.Category
	}
	if in.SpentAt != nil {
		d := *in.SpentAt
		e.SpentAt = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	}
	if in.Comment != nil {
		e.Comment = *in.Comment
	}
	if err := s.expenses.Update(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *ExpenseService) GetLast(ctx context.Context, userID int64) (*domain.Expense, error) {
	return s.expenses.GetLastByUser(ctx, userID)
}

func (s *ExpenseService) GetByIDForUser(ctx context.Context, id, userID int64) (*domain.Expense, error) {
	e, err := s.expenses.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if e.UserID != userID {
		return nil, errorsx.ErrForbidden
	}
	return e, nil
}
