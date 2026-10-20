package service

import (
	"context"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/repository"
)

type UserService struct {
	users repository.UserRepository
}

func NewUserService(users repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) UpsertFromTelegram(ctx context.Context, telegramID int64, displayName string, username *string, lang domain.Language) (*domain.User, error) {
	if lang == "" {
		lang = domain.LangRU
	}
	return s.users.UpsertByTelegram(ctx, telegramID, displayName, username, lang)
}

func (s *UserService) SetLanguage(ctx context.Context, userID int64, lang domain.Language) error {
	return s.users.UpdateLanguage(ctx, userID, lang)
}
