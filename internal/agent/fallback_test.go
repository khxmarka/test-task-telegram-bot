package agent

import (
	"strings"
	"testing"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
)

func TestParseSimpleCreateTable(t *testing.T) {
	tests := []struct {
		in  string
		ok  bool
		amt float64
		cur domain.Currency
	}{
		{"кофе 350", true, 350, domain.CurrencyBYN},
		{"3 USD lunch", true, 3, domain.CurrencyUSD},
		{"10 рублей", true, 10, domain.CurrencyBYN},
		{"такси 10 руб", true, 10, domain.CurrencyBYN},
		{"10 RUB такси", true, 10, domain.CurrencyRUB},
		{"10 российских рублей", true, 10, domain.CurrencyRUB},
		{"2.5 BYN", true, 2.5, domain.CurrencyBYN},
		{"привет", false, 0, ""},
	}
	for _, tt := range tests {
		p := ParseSimpleCreate(tt.in)
		if p.OK != tt.ok {
			t.Fatalf("%s: ok=%v want %v", tt.in, p.OK, tt.ok)
		}
		if !tt.ok {
			continue
		}
		if p.Amount != tt.amt || p.Currency != tt.cur {
			t.Fatalf("%s: got %+v want amt=%v cur=%v", tt.in, p, tt.amt, tt.cur)
		}
	}
}

func TestParseDeleteIDs(t *testing.T) {
	ids := ParseDeleteIDs("удали 4 и 1 записи")
	if len(ids) != 2 || ids[0] != 4 || ids[1] != 1 {
		t.Fatalf("got %v", ids)
	}
	ids = ParseDeleteIDs("удали #4 и #1")
	if len(ids) != 2 {
		t.Fatalf("got %v", ids)
	}
	if ParseDeleteIDs("покажи траты") != nil {
		t.Fatal("expected nil")
	}
}

func TestStripNullArgs(t *testing.T) {
	got := StripNullArgs(`{"amount":45,"comment":null,"raw_text":null,"currency":"BYN"}`)
	if strings.Contains(got, "null") {
		t.Fatalf("still has null: %s", got)
	}
	if !strings.Contains(got, `"amount":45`) || !strings.Contains(got, `"currency":"BYN"`) {
		t.Fatalf("got %s", got)
	}
}

func TestNormalizeCurrencyTypo(t *testing.T) {
	code, ok := NormalizeCurrencyTypo("bun")
	if !ok || code != "BYN" {
		t.Fatalf("%s %v", code, ok)
	}
}

func TestIsConfirmIsNotCreateWording(t *testing.T) {
	if !IsConfirm("да") || !IsConfirm("yes") {
		t.Fatal("yes-words must confirm")
	}
	for _, s := range []string{"запиши", "записывай", "запиши кофе 350", "добавь"} {
		if IsConfirm(s) {
			t.Fatalf("%q must not confirm a wipe", s)
		}
	}
}

func TestOffTopic(t *testing.T) {
	if !IsAppOffTopic("в каком году распался ссср") {
		t.Fatal("expected off-topic")
	}
	if !IsAppOffTopic("какая ты модель") {
		t.Fatal("expected off-topic")
	}
	if !IsAppOffTopic("что такое финансовая грамотность") {
		t.Fatal("expected off-topic")
	}
	if IsAppOffTopic("трата на развал ссср 10000") {
		t.Fatal("expense with amount should pass")
	}
	if IsAppOffTopic("кофе 350") {
		t.Fatal("expense")
	}
	if IsAppOffTopic("сколько потратила за неделю") {
		t.Fatal("period summary is in-scope")
	}
}
