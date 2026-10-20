package agent

import (
	"fmt"
	"strings"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/service"
)

func scoreExpense(e domain.Expense, query string) int {
	q := strings.ToLower(query)
	score := 0
	if strings.Contains(strings.ToLower(e.Comment), q) || (q != "" && strings.Contains(q, strings.ToLower(e.Comment)) && e.Comment != "") {
		score += 3
	}
	for _, w := range strings.Fields(q) {
		if strings.Contains(strings.ToLower(e.Comment), w) {
			score += 2
		}
		if strings.Contains(string(e.Category), w) {
			score += 2
		}
		if strings.Contains(service.FormatBYN(e.AmountBYNKopecks), w) {
			score += 1
		}
	}
	// Category keyword hints in query.
	hint := service.InferCategory(query)
	if hint != domain.CategoryOther && hint == e.Category {
		score += 4
	}
	return score
}

func FindCandidates(items []domain.Expense, query string, limit int) []domain.Expense {
	type scored struct {
		e domain.Expense
		s int
	}
	var all []scored
	for _, e := range items {
		s := scoreExpense(e, query)
		if s > 0 {
			all = append(all, scored{e: e, s: s})
		}
	}
	// simple sort
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].s > all[i].s {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if limit <= 0 {
		limit = 5
	}
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]domain.Expense, len(all))
	for i, s := range all {
		out[i] = s.e
	}
	return out
}

func FormatCandidates(items []domain.Expense, lang domain.Language) string {
	if len(items) == 0 {
		if lang == domain.LangEN {
			return "No matching expenses (0)."
		}
		return "Подходящих трат нет (0)."
	}
	var b strings.Builder
	if lang == domain.LangEN {
		b.WriteString("Which one?\n")
	} else {
		b.WriteString("Какую именно?\n")
	}
	for _, e := range items {
		fmt.Fprintf(&b, "#%d %s %s %s %s\n",
			e.ID, e.SpentAt.Format("2006-01-02"), service.FormatBYN(e.AmountBYNKopecks), e.Category, e.Comment)
	}
	return strings.TrimSpace(b.String())
}
