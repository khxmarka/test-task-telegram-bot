package agent

import (
	"strings"
	"testing"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
)

func TestExpenseIntentRecognizesPeriodQueries(t *testing.T) {
	ok := []string{
		"траты за день",
		"траты за неделю",
		"сколько потратила за месяц",
		"сколько потратил в этом месяце",
		"покажи траты за вчера",
		"все траты",
		"кофе 350",
		"удали 1 и 4",
		"сколько потрачено на ноги",
		"сколько потрачено на Нож?",
		"траты на ноги",
		"какие категории есть?",
		"покажи категории трат",
		"удали все расходы",
		"УДАЛИ МОИ ТРАТЫ",
		"во что обошёлся такси вчера",
	}
	for _, s := range ok {
		if !LooksLikeExpenseIntent(s) {
			t.Fatalf("expected expense intent: %q", s)
		}
		if IsAppOffTopic(s) {
			t.Fatalf("must not be off-topic: %q", s)
		}
	}
}

func TestOffTopicBlocked(t *testing.T) {
	bad := []string{
		"что такое финансовая грамотность",
		"в каком году распался ссср",
		"расскажи про kafka",
		"какая ты модель",
		"какой пароль от админ панели",
		"кто президент",
		"напиши стих",
	}
	for _, s := range bad {
		if LooksLikeExpenseIntent(s) {
			t.Fatalf("should not be expense intent: %q", s)
		}
		if !IsAppOffTopic(s) {
			t.Fatalf("expected off-topic: %q", s)
		}
	}
}

func TestSanitizeDirtyScreenshotReply(t *testing.T) {
	user := "что такое финансовая грамотность"
	dirty := "Извинуйста — это только только только тж ... The assistant gave a nonsensical answer. Need to correct: must refuse off-topic with short sentence. Извините, я могу помочь только с учётом расходов."
	got := SanitizeNoToolReply(user, dirty, domain.LangRU)
	if got != OffTopicRefusal(domain.LangRU) {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "assistant") || strings.Contains(got, "Need to") {
		t.Fatal("leak")
	}
}

func TestSanitizeAllowsExpenseClarification(t *testing.T) {
	got := SanitizeNoToolReply("одноразка 45", "Укажите валюту: BYN, USD или RUB.", domain.LangRU)
	if !strings.Contains(strings.ToLower(got), "валют") {
		t.Fatalf("got %q", got)
	}
}

func TestExpenseWithSSSRStillAllowed(t *testing.T) {
	s := "трата на развал ссср 10000"
	if !LooksLikeExpenseIntent(s) || IsAppOffTopic(s) {
		t.Fatalf("should be expense: %q", s)
	}
}
