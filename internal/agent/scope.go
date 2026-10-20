package agent

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
)

func foldYo(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "ё", "е")
}

func hasAny(t string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// expenseSignals are whole phrases / word forms, not one shared stem.
// People say «потрачено», «ушло», «во что обошлось» — none of those share потрат-.
var expenseSignals = []string{
	"трат", "расход", "expense", "spend", "spent", "spending", "cost",
	"потратил", "потратила", "потратили", "потрачено", "потрачен", "потрачу", "потратим",
	"сколько ушло", "ушло на", "ушло за", "ушло в",
	"купил", "купила", "купили", "покупк",
	"заплатил", "заплатила", "заплатили", "оплатил", "оплатила", "оплат",
	"истратил", "истратила", "истрачено",
	"обошелся", "обошлась", "обошлось", "стоило", "стоит", "стоимость",
	"записал", "запиши", "добав", "create", "update", "удал", "delete", "убери", "сотри",
	"покажи", "список", "итог", "сводк", "summary", "list",
	"категор", "categor",
	"жилье", "квартир", "аренд", "жкх", "коммунал",
	"такси", "транспорт", "кафе", "кофе", "магазин", "продукт",
	"валют", "byn", "usd", "rub", "доллар", "рубл",
}

var hardOffTopic = []string{
	"что такое", "в каком году", "кто президент", "какая ты модель", "какой ты модели",
	"what model", "who are you", "напиши стих", "напиши код", "расскажи про kafka",
	"какой пароль", "пароль от админ", "системный промпт", "system prompt",
	"финансовая грамотность", "расскажи анекдот", "погода", "рецепт",
}

var clarifyRe = regexp.MustCompile(`(?i)` +
	`валют|currency|уточн|какую|какой\s+именно|which|укажите|укажи|` +
	`byn|usd|rub|не понял|не поняла|уточните`)

var dirtyReplyRe = regexp.MustCompile(`(?i)` +
	`need\s+to\s+correct|the\s+assistant|the\s+user\s+(asks|wants|said)|off-?topic|nonsensical|` +
	`system\s+prompt|системн\S*\s+промпт|chain\s+of\s+thought|as\s+an\s+ai|` +
	`i\s+am\s+based|я\s+основан|gpt-?\d|openai|claude|llama|groq|` +
	`\b(function|tool)_?call|\{\s*"|` +
	`только\s+только|\.\.\.\s*\.\.\.|не\s*заверш[её]н`)

const maxFreeReplyRunes = 400

func ReplyIsDirty(reply string) bool {
	return dirtyReplyRe.MatchString(reply) || utf8.RuneCountInString(reply) > maxFreeReplyRunes
}

func IsLikelyClarificationReply(reply string) bool {
	return clarifyRe.MatchString(reply) && utf8.RuneCountInString(reply) <= 280
}

// LooksLikeExpenseIntent is a generous pre-filter: meaning, not one root.
func LooksLikeExpenseIntent(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if ParseSimpleCreate(text).OK {
		return true
	}
	if LooksLikeCategoryCatalog(text) || LooksLikeBulkDeleteOwn(text) {
		return true
	}
	if len(ParseDeleteIDs(text)) > 0 {
		return true
	}
	t := foldYo(text)
	for _, p := range expenseSignals {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

func isHardOffTopic(text string) bool {
	t := foldYo(text)
	for _, p := range hardOffTopic {
		if strings.Contains(t, foldYo(p)) {
			return true
		}
	}
	return false
}

// IsAppOffTopic blocks trivia/model talk that is clearly not bookkeeping.
// Anything that can reasonably be about the user's expenses is sent to the LLM.
func IsAppOffTopic(text string) bool {
	if LooksLikeExpenseIntent(text) {
		return false
	}
	return isHardOffTopic(text)
}

// SanitizeNoToolReply: if the model skipped tools, keep a short clarification or refuse.
func SanitizeNoToolReply(userText, reply string, lang domain.Language) string {
	reply = strings.TrimSpace(reply)
	if reply == "" || ReplyIsDirty(reply) {
		return OffTopicRefusal(lang)
	}
	if IsLikelyClarificationReply(reply) {
		return reply
	}
	if LooksLikeExpenseIntent(userText) && utf8.RuneCountInString(reply) <= 280 {
		return reply
	}
	return OffTopicRefusal(lang)
}
