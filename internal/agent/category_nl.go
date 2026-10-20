package agent

import (
	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/service"
)

func isCategoryToken(tok string) bool {
	return service.IsNamedCategoryToken(tok)
}

// CategoryFromUserText maps an explicit category mention. Item names stay as comment queries.
func CategoryFromUserText(text string) *domain.Category {
	return service.NamedCategory(text)
}
