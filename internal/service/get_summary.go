package service

import (
	"context"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
)

// GetSummary sums the user's expenses; f.UserID is always overwritten with the trusted userID.
func (s *ExpenseService) GetSummary(ctx context.Context, userID int64, f domain.ExpenseFilter) (*domain.PeriodSummary, error) {
	f.UserID = &userID
	return s.summary(ctx, f)
}

func (s *ExpenseService) GetFamilySummary(ctx context.Context, f domain.ExpenseFilter) (*domain.PeriodSummary, error) {
	return s.summary(ctx, f)
}

func (s *ExpenseService) summary(ctx context.Context, f domain.ExpenseFilter) (*domain.PeriodSummary, error) {
	total, count, err := s.expenses.Summary(ctx, f)
	if err != nil {
		return nil, err
	}
	out := &domain.PeriodSummary{Total: total, Count: count}
	if f.FromDate != nil {
		out.From = f.FromDate.Format("2006-01-02")
	}
	if f.ToDate != nil {
		out.To = f.ToDate.Format("2006-01-02")
	}
	return out, nil
}
