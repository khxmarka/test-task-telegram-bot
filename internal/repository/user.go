package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

type UserRepository interface {
	UpsertByTelegram(ctx context.Context, telegramID int64, displayName string, username *string, lang domain.Language) (*domain.User, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	UpdateLanguage(ctx context.Context, id int64, lang domain.Language) error
	ListMembers(ctx context.Context) ([]domain.User, error)
}

type userRepo struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) UpsertByTelegram(ctx context.Context, telegramID int64, displayName string, username *string, lang domain.Language) (*domain.User, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existing domain.User
	err = tx.GetContext(ctx, &existing, `SELECT * FROM users WHERE telegram_id = $1`, telegramID)
	if err == nil {
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET display_name = $2, username = $3, updated_at = NOW()
			WHERE id = $1`, existing.ID, displayName, username)
		if err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetByID(ctx, existing.ID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Serialize first-insert so two /start cannot both see headCount=0.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(872341)`); err != nil {
		return nil, fmt.Errorf("lock family head: %w", err)
	}
	var headCount int
	if err := tx.GetContext(ctx, &headCount, `SELECT COUNT(*) FROM users WHERE is_family_head = TRUE`); err != nil {
		return nil, fmt.Errorf("count heads: %w", err)
	}
	isHead := headCount == 0
	if lang == "" {
		lang = domain.LangRU
	}

	var id int64
	err = tx.QueryRowxContext(ctx, `
		INSERT INTO users (telegram_id, display_name, username, is_family_head, language, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id`,
		telegramID, displayName, username, isHead, lang,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *userRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	var u domain.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errorsx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (r *userRepo) UpdateLanguage(ctx context.Context, id int64, lang domain.Language) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE users SET language = $2, updated_at = $3 WHERE id = $1`, id, lang, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update language: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errorsx.ErrNotFound
	}
	return nil
}

func (r *userRepo) ListMembers(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	err := r.db.SelectContext(ctx, &users, `SELECT * FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}
