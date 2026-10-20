package agent

import (
	"testing"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
)

func TestCategoryFromUserText(t *testing.T) {
	c := CategoryFromUserText("сколько я потратил на жилье за все время")
	if c == nil || *c != domain.CategoryHousing {
		t.Fatalf("got %v", c)
	}
	c = CategoryFromUserText("траты на такси")
	if c == nil || *c != domain.CategoryTransport {
		t.Fatalf("got %v", c)
	}
	if CategoryFromUserText("сколько потратил за месяц") != nil {
		t.Fatal("no specific category")
	}
	if CategoryFromUserText("жила в минске") != nil {
		t.Fatal("жила must not match housing")
	}
	if CategoryFromUserText("траты на ноги") != nil {
		t.Fatal("item names are comments, not categories")
	}
}
