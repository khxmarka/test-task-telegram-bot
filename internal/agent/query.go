package agent

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/service"
)

// Words we strip when guessing a comment-search query from the user phrase.
// Intent glue, not content: leftover tokens are what the person named (ноги, нож, лего).
var queryFiller = map[string]struct{}{
	"сколько": {}, "ушло": {}, "ушли": {}, "потратил": {}, "потратила": {}, "потратили": {},
	"потрачено": {}, "потрачен": {}, "истратил": {}, "истратила": {}, "истрачено": {},
	"купил": {}, "купила": {}, "купили": {}, "заплатил": {}, "заплатила": {}, "оплатил": {},
	"обошелся": {}, "обошлась": {}, "обошлось": {}, "стоило": {}, "стоит": {}, "стоимость": {},
	"трат": {}, "трата": {}, "траты": {}, "трату": {}, "тратами": {},
	"расход": {}, "расхода": {}, "расходы": {}, "расходов": {},
	"expense": {}, "expenses": {}, "spend": {}, "spent": {}, "spending": {}, "cost": {}, "costs": {},
	"покажи": {}, "покажите": {}, "показать": {}, "список": {}, "итог": {}, "итого": {}, "сводка": {},
	"show": {}, "list": {}, "summary": {}, "total": {}, "how": {}, "much": {}, "did": {}, "was": {},
	"мои": {}, "мой": {}, "моя": {}, "мое": {}, "наши": {}, "наш": {}, "своя": {}, "свои": {}, "свое": {},
	"my": {}, "our": {}, "the": {}, "a": {}, "an": {}, "of": {}, "for": {}, "on": {}, "in": {}, "at": {},
	"за": {}, "на": {}, "в": {}, "во": {}, "по": {}, "про": {}, "из": {}, "с": {}, "со": {}, "к": {},
	"от": {}, "до": {}, "о": {}, "об": {}, "и": {}, "или": {}, "а": {}, "же": {}, "ли": {}, "бы": {},
	"все": {}, "всего": {}, "всей": {}, "всем": {}, "всех": {}, "all": {},
	"категор": {}, "категория": {}, "категории": {}, "категорию": {}, "категориям": {}, "category": {}, "categories": {},
	"сегодня": {}, "вчера": {}, "позавчера": {}, "завтра": {}, "неделю": {}, "неделя": {}, "неделе": {}, "недели": {},
	"месяц": {}, "месяца": {}, "месяце": {}, "год": {}, "года": {}, "году": {}, "время": {},
	"этой": {}, "этом": {}, "этот": {}, "эта": {}, "эти": {}, "прошлой": {}, "прошлом": {}, "прошлую": {},
	"прошлый": {}, "прошлые": {}, "last": {}, "this": {}, "today": {}, "yesterday": {}, "week": {},
	"month": {}, "year": {}, "morning": {}, "evening": {},
	"удали": {}, "удалить": {}, "удаляй": {}, "убери": {}, "сотри": {}, "очисти": {}, "очистить": {},
	"delete": {}, "remove": {}, "clear": {}, "wipe": {},
	"записей": {}, "запись": {}, "записи": {}, "entry": {}, "entries": {},
	"пожалуйста": {}, "please": {}, "мне": {}, "меня": {}, "я": {}, "мы": {}, "i": {}, "we": {}, "me": {},
	"what": {}, "which": {}, "есть": {}, "какие": {}, "какой": {}, "какая": {}, "какое": {},
	"что": {}, "чем": {}, "куда": {}, "когда": {}, "где": {}, "зачем": {}, "почему": {},
	"who": {}, "where": {}, "when": {}, "why": {},
}

var bulkDeleteFiller = map[string]struct{}{
	"удали": {}, "удалить": {}, "удаляй": {}, "убери": {}, "сотри": {}, "очисти": {}, "очистить": {},
	"delete": {}, "remove": {}, "clear": {}, "wipe": {},
	"все": {}, "всех": {}, "всей": {}, "all": {},
	"мои": {}, "мой": {}, "моя": {}, "мое": {}, "свои": {}, "свое": {}, "my": {},
	"трат": {}, "трата": {}, "траты": {}, "трату": {}, "тратами": {},
	"расход": {}, "расхода": {}, "расходы": {}, "расходов": {},
	"expense": {}, "expenses": {}, "записей": {}, "запись": {}, "записи": {},
	"пожалуйста": {}, "please": {},
}

func tokenize(text string) []string {
	t := foldYo(text)
	return strings.FieldsFunc(t, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func dropTokens(text string, extra map[string]struct{}) []string {
	var keep []string
	for _, tok := range tokenize(text) {
		if _, ok := queryFiller[tok]; ok {
			continue
		}
		if extra != nil {
			if _, ok := extra[tok]; ok {
				continue
			}
		}
		if isCategoryToken(tok) {
			continue
		}
		keep = append(keep, tok)
	}
	return keep
}

// ExtractCommentQuery leftover after stripping intent glue: «сколько потрачено на ноги» → «ноги».
func ExtractCommentQuery(text string) string {
	keep := dropTokens(text, nil)
	return strings.TrimSpace(strings.Join(keep, " "))
}

func hasDeleteVerb(t string) bool {
	return hasAny(t, "удал", "delete", "убери", "сотри", "очист", "wipe", "remove")
}

// LooksLikeBulkDeleteOwn: wipe MY expenses (no leftover thing-name, no numeric ids).
func LooksLikeBulkDeleteOwn(text string) bool {
	if len(ParseDeleteIDs(text)) > 0 {
		return false
	}
	t := foldYo(text)
	if !hasDeleteVerb(t) {
		return false
	}
	if hasAny(t, "базу", "бд", "database", "table", "пользовател", "чуж") {
		return false
	}
	leftover := dropTokens(text, bulkDeleteFiller)
	return len(leftover) == 0
}

// LooksLikeCategoryCatalog: «какие категории есть», not «траты по категориям».
func LooksLikeCategoryCatalog(text string) bool {
	t := foldYo(text)
	if !hasAny(t, "категор", "categor") {
		return false
	}
	if hasAny(t, "сколько", "потрач", "ушло", "по категориям", "по категории", "в категории") {
		return false
	}
	return hasAny(t, "как", "спис", "есть", "покаж", "what", "which", "доступ", "перечисл", "назов")
}

func filterFromArgs(args map[string]any, loc *time.Location) domain.ExpenseFilter {
	f := domain.ExpenseFilter{}
	if c, ok := args["category"].(string); ok && strings.TrimSpace(c) != "" {
		if cat, err := service.ParseCategory(c); err == nil {
			f.Category = &cat
		}
	}
	if q, ok := args["query"].(string); ok {
		f.Query = strings.TrimSpace(q)
	}
	from, to := PeriodFromArgs(args, loc)
	f.FromDate, f.ToDate = from, to
	return f
}

// enrichFilterArgs fills category / query / period when the model omitted them.
func enrichFilterArgs(args map[string]any, userText string) {
	if args == nil {
		return
	}
	if cat, _ := args["category"].(string); strings.TrimSpace(cat) == "" {
		if hint := CategoryFromUserText(userText); hint != nil {
			args["category"] = string(*hint)
		}
	}
	// Do not invent a comment query when the model already scoped by category —
	// otherwise «еду» sticks onto category=stores and the summary returns 0.
	if cat, _ := args["category"].(string); strings.TrimSpace(cat) == "" {
		if q, _ := args["query"].(string); strings.TrimSpace(q) == "" {
			if leftover := ExtractCommentQuery(userText); leftover != "" {
				args["query"] = leftover
			}
		}
	}
	if p, _ := args["period"].(string); strings.TrimSpace(p) == "" {
		if _, hasFrom := args["from"]; !hasFrom {
			if hint := PeriodHintFromText(userText); hint != "" {
				args["period"] = hint
			}
		}
	}
}

func enrichCallsFromText(calls []ToolCall, userText string) {
	for i := range calls {
		name := calls[i].Function.Name
		switch name {
		case ToolGetSummary, ToolGetByCategory, ToolListExpenses, ToolDeleteExpenses:
		default:
			continue
		}
		var args map[string]any
		if err := json.Unmarshal([]byte(StripNullArgs(calls[i].Function.Arguments)), &args); err != nil {
			args = map[string]any{}
		}
		if args == nil {
			args = map[string]any{}
		}
		if name == ToolDeleteExpenses {
			if all, _ := args["all"].(bool); all {
				b, _ := json.Marshal(args)
				calls[i].Function.Arguments = string(b)
				continue
			}
			if LooksLikeBulkDeleteOwn(userText) {
				args["all"] = true
			}
		}
		enrichFilterArgs(args, userText)
		b, _ := json.Marshal(args)
		calls[i].Function.Arguments = string(b)
	}
}
