package agent

import "github.com/testTelegramApp/expenses-bot/internal/domain"

const (
	ToolCreateExpense  = "create_expense"
	ToolListExpenses   = "list_expenses"
	ToolUpdateExpense  = "update_expense"
	ToolDeleteExpense  = "delete_expense"
	ToolDeleteExpenses = "delete_expenses"
	ToolGetSummary     = "get_summary"
	ToolGetByCategory  = "get_by_category"
	ToolListCategories = "list_categories"
	ToolReply          = "reply"
	ToolRefuseOffTopic = "refuse_off_topic"
)

var Whitelist = map[string]struct{}{
	ToolCreateExpense:  {},
	ToolListExpenses:   {},
	ToolUpdateExpense:  {},
	ToolDeleteExpense:  {},
	ToolDeleteExpenses: {},
	ToolGetSummary:     {},
	ToolGetByCategory:  {},
	ToolListCategories: {},
	ToolReply:          {},
	ToolRefuseOffTopic: {},
}

var periodEnum = []string{"today", "yesterday", "week", "last_week", "month", "last_month", "year", "all"}

// filterProps are shared by every read / bulk tool so the model can combine them freely.
func filterProps() map[string]any {
	return map[string]any{
		"category": map[string]any{"type": "string", "enum": categoryEnums(), "description": "Only this category"},
		"query": map[string]any{"type": "string", "description": "Word(s) to search in expense comments, e.g. 'ноги', 'нож', 'лего'. Use when the user names a thing, not a category."},
		"period":   map[string]any{"type": "string", "enum": periodEnum},
		"from":     map[string]any{"type": "string", "description": "YYYY-MM-DD, inclusive"},
		"to":       map[string]any{"type": "string", "description": "YYYY-MM-DD, inclusive"},
	}
}

func withProps(base map[string]any, extra map[string]any) map[string]any {
	for k, v := range extra {
		base[k] = v
	}
	return base
}

func obj(props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props}
}

func ToolDefinitions() []ToolDef {
	return []ToolDef{
		fn(ToolCreateExpense, "Record a new expense. Default currency BYN; plain 'рубли/руб' = BYN, RUB only for explicitly Russian rubles. Always pick the best category yourself.", obj(map[string]any{
			"amount":   map[string]any{"type": "number", "description": "Amount as entered by user"},
			"currency": map[string]any{"type": "string", "enum": []string{"BYN", "USD", "RUB"}},
			"category": map[string]any{"type": "string", "enum": categoryEnums()},
			"spent_at": map[string]any{"type": "string", "description": "YYYY-MM-DD; omit for today"},
			"comment":  map[string]any{"type": "string", "description": "What was bought, short"},
			"raw_text": map[string]any{"type": "string", "description": "Original user text"},
		})),
		fn(ToolListExpenses, "List the user's expenses. Combine filters: category, query (comment search), period/from/to.", obj(withProps(filterProps(), map[string]any{
			"limit":  map[string]any{"type": "integer"},
			"offset": map[string]any{"type": "integer"},
		}))),
		fn(ToolGetSummary, "Total amount and count of the user's expenses. Combine filters: category, query (comment search), period/from/to.", obj(filterProps())),
		fn(ToolGetByCategory, "Totals grouped by category for the user's expenses (optionally filtered).", obj(filterProps())),
		fn(ToolUpdateExpense, "Update ONE expense of the user (by id, last, or search query).", obj(map[string]any{
			"expense_id": map[string]any{"type": "integer"},
			"use_last":   map[string]any{"type": "boolean"},
			"query":      map[string]any{"type": "string", "description": "Search text if id unknown"},
			"amount":     map[string]any{"type": "number"},
			"currency":   map[string]any{"type": "string", "enum": []string{"BYN", "USD", "RUB"}},
			"category":   map[string]any{"type": "string", "enum": categoryEnums()},
			"spent_at":   map[string]any{"type": "string"},
			"comment":    map[string]any{"type": "string"},
		})),
		fn(ToolDeleteExpense, "Delete ONE expense of the user (by id, last, or search query).", obj(map[string]any{
			"expense_id": map[string]any{"type": "integer"},
			"use_last":   map[string]any{"type": "boolean"},
			"query":      map[string]any{"type": "string"},
		})),
		fn(ToolDeleteExpenses, "Delete SEVERAL of the current user's own expenses by filter (all=true for all of them). Server asks the user to confirm first; never touches other users.", obj(withProps(filterProps(), map[string]any{
			"all": map[string]any{"type": "boolean", "description": "true = all of the user's expenses (filters ignored)"},
		}))),
		fn(ToolListCategories, "Show the available expense categories.", obj(map[string]any{})),
		fn(ToolReply, "Short text reply about using this bot: clarifying question, greeting, help. Never for off-topic answers.", obj(map[string]any{
			"text": map[string]any{"type": "string", "description": "1-2 sentences in the user's language"},
		})),
		fn(ToolRefuseOffTopic, "Use when the message is not about the user's expenses in this bot.", obj(map[string]any{})),
	}
}

func fn(name, desc string, params map[string]any) ToolDef {
	return ToolDef{
		Type: "function",
		Function: ToolFunctionSchema{
			Name:        name,
			Description: desc,
			Parameters:  params,
		},
	}
}

func categoryEnums() []string {
	out := make([]string, len(domain.OrderedCategories))
	for i, c := range domain.OrderedCategories {
		out[i] = string(c)
	}
	return out
}
