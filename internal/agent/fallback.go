package agent

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
)

var amountRe = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*([^\d\s]+)?`)

var deleteIDsRe = regexp.MustCompile(`#?\s*(\d+)`)

// Whole message must be "удали [траты] 1, 4 и #7 [записи]" — anything else goes to the LLM.
var deleteIDsCmdRe = regexp.MustCompile(`(?i)^\s*(удали(ть)?|убери|сотри|delete|remove)\s+` +
	`((трат[уыа]?|запис[ьи]|расход[ыа]?|expenses?|номер[а]?)\s+)?` +
	`#?\d+(\s*(,|и|and|&)?\s*#?\d+)*` +
	`(\s+(трат[уыа]?|запис[ьи]|расход[ыа]?|expenses?))?\s*[.!]?\s*$`)

func normalizeAnswer(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, ".!?,) ")
	return strings.ReplaceAll(s, "ё", "е")
}

// IsConfirm matches a short yes-answer to a pending confirmation.
func IsConfirm(text string) bool {
	switch normalizeAnswer(text) {
	case "да", "ага", "угу", "давай", "удаляй", "подтверждаю", "да, удаляй", "да удаляй",
		"ок", "окей", "конечно", "yes", "y", "ok", "okay", "sure", "confirm":
		return true
	}
	return false
}

// IsCancel matches a short no-answer to a pending confirmation.
func IsCancel(text string) bool {
	switch normalizeAnswer(text) {
	case "нет", "не", "не надо", "отмена", "отмени", "стоп", "no", "n", "cancel", "stop":
		return true
	}
	return false
}

type ParsedCreate struct {
	Amount   float64
	Currency domain.Currency
	Comment  string
	OK       bool
}

// ParseSimpleCreate parses amount + currency. Default currency is BYN.
// Plain "руб/рубли" → BYN. Russian rubles only if explicit (RUB / рос / российск).
func ParseSimpleCreate(text string) ParsedCreate {
	m := amountRe.FindStringSubmatch(text)
	if m == nil {
		return ParsedCreate{}
	}
	num := strings.ReplaceAll(m[1], ",", ".")
	amt, err := strconv.ParseFloat(num, 64)
	if err != nil || amt <= 0 {
		return ParsedCreate{}
	}
	cur := InferCurrency(text)
	comment := strings.TrimSpace(amountRe.ReplaceAllString(text, " "))
	return ParsedCreate{Amount: amt, Currency: cur, Comment: comment, OK: true}
}

var latinRubRe = regexp.MustCompile(`(?i)(^|[^a-zа-яё])rub([^a-zа-яё]|$)`)

// InferCurrency from free text. Default BYN.
func InferCurrency(text string) domain.Currency {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "usd"), strings.Contains(lower, "$"), strings.Contains(lower, "доллар"):
		return domain.CurrencyUSD
	case strings.Contains(lower, "российск"), strings.Contains(lower, "рос руб"),
		strings.Contains(lower, "рос."), latinRubRe.MatchString(lower):
		return domain.CurrencyRUB
	case strings.Contains(lower, "byn"), strings.Contains(lower, "bun"), strings.Contains(lower, "бел"):
		return domain.CurrencyBYN
	default:
		// "рубли", "руб", "р", bare number → BYN
		return domain.CurrencyBYN
	}
}

// ParseDeleteIDs extracts ids from "удали 1 и 4", "удали #4 и #1", "удали 4 и 1 записи".
func ParseDeleteIDs(text string) []int64 {
	if !deleteIDsCmdRe.MatchString(text) {
		return nil
	}
	matches := deleteIDsRe.FindAllStringSubmatch(text, -1)
	seen := map[int64]struct{}{}
	var ids []int64
	for _, m := range matches {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		ids = append(ids, n)
	}
	return ids
}

// NormalizeCurrencyTypo maps typos / short answers to ISO-like codes.
func NormalizeCurrencyTypo(s string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "bun", "byn", "бел", "белруб", "белорус", "белорусские":
		return "BYN", true
	case "usd", "$", "доллар", "доллары", "долларов":
		return "USD", true
	case "rub", "рос", "российский", "российских", "российские", "российский рубль":
		return "RUB", true
	default:
		u := strings.ToUpper(strings.TrimSpace(s))
		if u == "BYN" || u == "USD" || u == "RUB" {
			return u, true
		}
		return "", false
	}
}
