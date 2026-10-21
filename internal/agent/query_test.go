package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestExtractCommentQuery(t *testing.T) {
	tests := map[string]string{
		"сколько потрачено на ноги":             "ноги",
		"сколько потрачено на Нож?":             "нож",
		"траты на ход ноги":                     "ход ноги",
		"сколько ушло на жилье вчера":           "",
		"траты на такси":                        "",
		"сколько потратил на еду в этом месяце": "",
		"сколько ушло на food":                  "",
	}
	for in, want := range tests {
		got := ExtractCommentQuery(in)
		if got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestEnrichCallsFromTextFoodMonth(t *testing.T) {
	calls := []ToolCall{{
		Function: ToolCallFunction{
			Name:      ToolGetSummary,
			Arguments: `{"category":"stores","period":"month"}`,
		},
	}}
	enrichCallsFromText(calls, "сколько потратил на еду в этом месяце")
	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
		t.Fatal(err)
	}
	if q, _ := args["query"].(string); strings.TrimSpace(q) != "" {
		t.Fatalf("expected empty query, got %q args=%v", q, args)
	}
	if args["category"] != "stores" {
		t.Fatalf("category=%v", args["category"])
	}
}

func TestLooksLikeBulkDeleteOwn(t *testing.T) {
	ok := []string{"удали все", "удали все расходы", "УДАЛИ МОИ ТРАТЫ", "delete all expenses please"}
	for _, s := range ok {
		if !LooksLikeBulkDeleteOwn(s) {
			t.Fatalf("expected bulk-own: %q", s)
		}
	}
	no := []string{"удали 1 и 4", "удали все на ноги", "кофе 350", "удали всю базу"}
	for _, s := range no {
		if LooksLikeBulkDeleteOwn(s) {
			t.Fatalf("did not expect bulk-own: %q", s)
		}
	}
}

func TestLooksLikeCategoryCatalog(t *testing.T) {
	if !LooksLikeCategoryCatalog("какие категории есть?") {
		t.Fatal("catalog")
	}
	if !LooksLikeCategoryCatalog("покажи категории трат") {
		t.Fatal("show categories")
	}
	if LooksLikeCategoryCatalog("сколько ушло на жилье") {
		t.Fatal("summary is not a catalog question")
	}
}

func TestPeriodHintAndYesterday(t *testing.T) {
	if PeriodHintFromText("сколько ушло на жилье вчера") != "yesterday" {
		t.Fatal("yesterday hint")
	}
	from, to := ResolvePeriod("yesterday", time.UTC)
	if from == nil || to == nil || !from.Equal(*to) {
		t.Fatalf("yesterday range %+v %+v", from, to)
	}
}
