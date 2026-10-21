package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

type mockExpenseRepo struct {
	mu     sync.Mutex
	nextID int64
	items  map[int64]*domain.Expense
}

func newMockExpenseRepo() *mockExpenseRepo {
	return &mockExpenseRepo{nextID: 1, items: map[int64]*domain.Expense{}}
}

func (m *mockExpenseRepo) Create(ctx context.Context, e *domain.Expense) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.ID = m.nextID
	m.nextID++
	now := time.Now().UTC()
	e.CreatedAt = now
	e.UpdatedAt = now
	cp := *e
	m.items[e.ID] = &cp
	return nil
}

func (m *mockExpenseRepo) GetByID(ctx context.Context, id int64) (*domain.Expense, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[id]
	if !ok {
		return nil, errorsx.ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (m *mockExpenseRepo) Update(ctx context.Context, e *domain.Expense) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.items[e.ID]
	if !ok || cur.UserID != e.UserID {
		return errorsx.ErrNotFound
	}
	cp := *e
	cp.UpdatedAt = time.Now().UTC()
	m.items[e.ID] = &cp
	return nil
}

func (m *mockExpenseRepo) Delete(ctx context.Context, id, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[id]
	if !ok || e.UserID != userID {
		return errorsx.ErrNotFound
	}
	delete(m.items, id)
	return nil
}

func (m *mockExpenseRepo) DeleteByID(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[id]; !ok {
		return errorsx.ErrNotFound
	}
	delete(m.items, id)
	return nil
}

func (m *mockExpenseRepo) List(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error) {
	all := m.filter(f)
	start := f.Offset
	if start > len(all) {
		return nil, nil
	}
	end := start + f.Limit
	if end > len(all) || f.Limit <= 0 {
		end = len(all)
	}
	return all[start:end], nil
}

func (m *mockExpenseRepo) Count(ctx context.Context, f domain.ExpenseFilter) (int64, error) {
	return int64(len(m.filter(f))), nil
}

func (m *mockExpenseRepo) filter(f domain.ExpenseFilter) []domain.Expense {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Expense
	for _, e := range m.items {
		if f.UserID != nil && e.UserID != *f.UserID {
			continue
		}
		if f.Category != nil && e.Category != *f.Category {
			continue
		}
		if f.FromDate != nil && e.SpentAt.Before(*f.FromDate) {
			continue
		}
		if f.ToDate != nil && e.SpentAt.After(*f.ToDate) {
			continue
		}
		if f.Query != "" && !strings.Contains(strings.ToLower(e.Comment), strings.ToLower(f.Query)) {
			continue
		}
		out = append(out, *e)
	}
	return out
}

func (m *mockExpenseRepo) GetLastByUser(ctx context.Context, userID int64) (*domain.Expense, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best *domain.Expense
	for _, e := range m.items {
		if e.UserID != userID {
			continue
		}
		if best == nil || e.CreatedAt.After(best.CreatedAt) {
			cp := *e
			best = &cp
		}
	}
	if best == nil {
		return nil, errorsx.ErrNotFound
	}
	return best, nil
}

func (m *mockExpenseRepo) Summary(ctx context.Context, f domain.ExpenseFilter) (int64, int64, error) {
	items := m.filter(f)
	var total int64
	for _, e := range items {
		total += e.AmountBYNKopecks
	}
	return total, int64(len(items)), nil
}

func (m *mockExpenseRepo) ByCategory(ctx context.Context, f domain.ExpenseFilter) ([]domain.CategorySummary, error) {
	items := m.filter(f)
	agg := map[domain.Category]*domain.CategorySummary{}
	for _, e := range items {
		s, ok := agg[e.Category]
		if !ok {
			s = &domain.CategorySummary{Category: e.Category}
			agg[e.Category] = s
		}
		s.Total += e.AmountBYNKopecks
		s.Count++
	}
	var out []domain.CategorySummary
	for _, s := range agg {
		out = append(out, *s)
	}
	return out, nil
}

func (m *mockExpenseRepo) ListFamily(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error) {
	return m.filter(f), nil
}

type mockUserRepo struct {
	users map[int64]*domain.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: map[int64]*domain.User{}}
}

func (m *mockUserRepo) UpsertByTelegram(ctx context.Context, telegramID int64, displayName string, username *string, lang domain.Language) (*domain.User, error) {
	for _, u := range m.users {
		if u.TelegramID == telegramID {
			u.DisplayName = displayName
			u.Username = username
			cp := *u
			return &cp, nil
		}
	}
	id := int64(len(m.users) + 1)
	u := &domain.User{
		ID: id, TelegramID: telegramID, DisplayName: displayName, Username: username,
		IsFamilyHead: len(m.users) == 0, Language: lang, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	m.users[id] = u
	cp := *u
	return &cp, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, errorsx.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *mockUserRepo) UpdateLanguage(ctx context.Context, id int64, lang domain.Language) error {
	u, ok := m.users[id]
	if !ok {
		return errorsx.ErrNotFound
	}
	u.Language = lang
	return nil
}

func (m *mockUserRepo) ListMembers(ctx context.Context) ([]domain.User, error) {
	var out []domain.User
	for _, u := range m.users {
		out = append(out, *u)
	}
	return out, nil
}
