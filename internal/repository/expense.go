package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jmoiron/sqlx"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

type ExpenseRepository interface {
	Create(ctx context.Context, e *domain.Expense) error
	GetByID(ctx context.Context, id int64) (*domain.Expense, error)
	Update(ctx context.Context, e *domain.Expense) error
	Delete(ctx context.Context, id, userID int64) error
	DeleteByID(ctx context.Context, id int64) error
	List(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error)
	Count(ctx context.Context, f domain.ExpenseFilter) (int64, error)
	GetLastByUser(ctx context.Context, userID int64) (*domain.Expense, error)
	Summary(ctx context.Context, f domain.ExpenseFilter) (total, count int64, err error)
	ByCategory(ctx context.Context, f domain.ExpenseFilter) ([]domain.CategorySummary, error)
	ListFamily(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error)
}

type expenseRepo struct {
	db *sqlx.DB
}

func NewExpenseRepository(db *sqlx.DB) ExpenseRepository {
	return &expenseRepo{db: db}
}

func (r *expenseRepo) Create(ctx context.Context, e *domain.Expense) error {
	return r.db.QueryRowxContext(ctx, `
		INSERT INTO expenses (user_id, amount_byn_kopecks, category, spent_at, comment, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, created_at, updated_at`,
		e.UserID, e.AmountBYNKopecks, e.Category, e.SpentAt, e.Comment,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

func (r *expenseRepo) GetByID(ctx context.Context, id int64) (*domain.Expense, error) {
	var e domain.Expense
	err := r.db.GetContext(ctx, &e, `SELECT * FROM expenses WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errorsx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get expense: %w", err)
	}
	return &e, nil
}

func (r *expenseRepo) Update(ctx context.Context, e *domain.Expense) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE expenses
		SET amount_byn_kopecks = $2, category = $3, spent_at = $4, comment = $5, updated_at = NOW()
		WHERE id = $1 AND user_id = $6`,
		e.ID, e.AmountBYNKopecks, e.Category, e.SpentAt, e.Comment, e.UserID,
	)
	if err != nil {
		return fmt.Errorf("update expense: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errorsx.ErrNotFound
	}
	return nil
}

func (r *expenseRepo) Delete(ctx context.Context, id, userID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM expenses WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete expense: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errorsx.ErrNotFound
	}
	return nil
}

func (r *expenseRepo) DeleteByID(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM expenses WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete expense by id: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errorsx.ErrNotFound
	}
	return nil
}

func (r *expenseRepo) GetLastByUser(ctx context.Context, userID int64) (*domain.Expense, error) {
	var e domain.Expense
	err := r.db.GetContext(ctx, &e, `
		SELECT * FROM expenses WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errorsx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("last expense: %w", err)
	}
	return &e, nil
}

// prefixTSQuery turns "ход ноги" into "ход:* & ноги:*" for to_tsquery('russian', …):
// Postgres stems every word (ноги → ног), so «нога», «ногу», «ноги» all match.
func prefixTSQuery(q string) string {
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		words = append(words, w+":*")
	}
	return strings.Join(words, " & ")
}

// buildWhere renders filter conditions; col prefixes column names (e.g. "e.") for joins.
func buildWhere(f domain.ExpenseFilter, col string) (string, []any, int) {
	var parts []string
	var args []any
	n := 1
	add := func(cond string, v any) {
		parts = append(parts, fmt.Sprintf(cond, n))
		args = append(args, v)
		n++
	}
	if f.UserID != nil {
		add(col+"user_id = $%d", *f.UserID)
	}
	if f.Category != nil {
		add(col+"category = $%d", *f.Category)
	}
	if f.FromDate != nil {
		add(col+"spent_at >= $%d", f.FromDate.Format("2006-01-02"))
	}
	if f.ToDate != nil {
		add(col+"spent_at <= $%d", f.ToDate.Format("2006-01-02"))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		cond := fmt.Sprintf("%scomment ILIKE $%d", col, n)
		args = append(args, "%"+q+"%")
		n++
		if ts := prefixTSQuery(q); ts != "" {
			cond = fmt.Sprintf("(%s OR to_tsvector('russian', %scomment) @@ to_tsquery('russian', $%d))", cond, col, n)
			args = append(args, ts)
			n++
		}
		parts = append(parts, cond)
	}
	where := ""
	if len(parts) > 0 {
		where = "WHERE " + strings.Join(parts, " AND ")
	}
	return where, args, n
}

func (r *expenseRepo) List(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error) {
	where, args, n := buildWhere(f, "")
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	q := fmt.Sprintf(`
		SELECT * FROM expenses %s
		ORDER BY spent_at DESC, created_at DESC
		LIMIT $%d OFFSET $%d`, where, n, n+1)
	args = append(args, limit, offset)
	var items []domain.Expense
	if err := r.db.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	return items, nil
}

func (r *expenseRepo) Count(ctx context.Context, f domain.ExpenseFilter) (int64, error) {
	where, args, _ := buildWhere(f, "")
	q := fmt.Sprintf(`SELECT COUNT(*) FROM expenses %s`, where)
	var count int64
	if err := r.db.GetContext(ctx, &count, q, args...); err != nil {
		return 0, fmt.Errorf("count expenses: %w", err)
	}
	return count, nil
}

func (r *expenseRepo) Summary(ctx context.Context, f domain.ExpenseFilter) (int64, int64, error) {
	where, args, _ := buildWhere(f, "")
	q := fmt.Sprintf(`SELECT COALESCE(SUM(amount_byn_kopecks),0), COUNT(*) FROM expenses %s`, where)
	var total, count int64
	if err := r.db.QueryRowxContext(ctx, q, args...).Scan(&total, &count); err != nil {
		return 0, 0, fmt.Errorf("summary: %w", err)
	}
	return total, count, nil
}

func (r *expenseRepo) ByCategory(ctx context.Context, f domain.ExpenseFilter) ([]domain.CategorySummary, error) {
	where, args, _ := buildWhere(f, "")
	q := fmt.Sprintf(`
		SELECT category, COALESCE(SUM(amount_byn_kopecks),0) AS total, COUNT(*) AS count
		FROM expenses %s
		GROUP BY category
		ORDER BY total DESC`, where)
	var rows []domain.CategorySummary
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("by category: %w", err)
	}
	return rows, nil
}

func (r *expenseRepo) ListFamily(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error) {
	where, args, _ := buildWhere(f, "e.")
	q := fmt.Sprintf(`
		SELECT e.*, u.display_name AS user_display_name
		FROM expenses e
		JOIN users u ON u.id = e.user_id
		%s
		ORDER BY e.spent_at DESC, e.created_at DESC`, where)
	var items []domain.Expense
	if err := r.db.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, fmt.Errorf("list family: %w", err)
	}
	return items, nil
}
