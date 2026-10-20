package service

import (
	"context"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
)

func (s *ExpenseService) GetByCategory(ctx context.Context, userID int64, f domain.ExpenseFilter) ([]domain.CategorySummary, error) {
	f.UserID = &userID
	return s.expenses.ByCategory(ctx, f)
}

func (s *ExpenseService) GetFamilyByCategory(ctx context.Context, f domain.ExpenseFilter) ([]domain.CategorySummary, error) {
	return s.expenses.ByCategory(ctx, f)
}
