package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/testTelegramApp/expenses-bot/internal/config"
	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/service"
)

type pendingBulk struct {
	userID int64
	ids    []int64               // explicit ids (e.g. «удали 1 и 4»)
	filter *domain.ExpenseFilter // own rows matching filter; nil = use ids
	count  int64
	total  int64
}

type Agent struct {
	queue     *Queue
	expenses  *service.ExpenseService
	users     *service.UserService
	history   *HistoryStore
	rates     config.CurrencyConfig
	llmCfg    config.LLMConfig
	loc       *time.Location
	log       *zap.Logger
	pendingMu sync.Mutex
	pending   map[int64]*pendingBulk
}

func New(
	llm LLMClient,
	expenses *service.ExpenseService,
	users *service.UserService,
	rates config.CurrencyConfig,
	llmCfg config.LLMConfig,
	loc *time.Location,
	log *zap.Logger,
) *Agent {
	return &Agent{
		queue:    NewQueue(llm, 32),
		expenses: expenses,
		users:    users,
		history:  NewHistoryStore(),
		rates:    rates,
		llmCfg:   llmCfg,
		loc:      loc,
		log:      log,
		pending:  map[int64]*pendingBulk{},
	}
}

func (a *Agent) Close() {
	a.queue.Close()
}

type HandleResult struct {
	Reply string
}

func (a *Agent) HandleMessage(ctx context.Context, chatID int64, text string, tc TrustedContext) HandleResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return HandleResult{Reply: a.msg(tc.Language, "Напишите трату или вопрос.", "Send an expense or a question.")}
	}

	if reply, handled := a.handlePending(ctx, chatID, text, tc); handled {
		return HandleResult{Reply: reply}
	}

	if RejectInjection(text) {
		return HandleResult{Reply: a.msg(tc.Language,
			"Не могу выполнить такой запрос.",
			"I can't do that.")}
	}

	lower := strings.ToLower(text)
	if lower == "english" || lower == "en" || strings.Contains(lower, "switch to english") {
		_ = a.users.SetLanguage(ctx, tc.UserID, domain.LangEN)
		tc.Language = domain.LangEN
		return HandleResult{Reply: "Language set to English."}
	}
	if lower == "русский" || lower == "ru" || strings.Contains(lower, "переключи на русский") {
		_ = a.users.SetLanguage(ctx, tc.UserID, domain.LangRU)
		tc.Language = domain.LangRU
		return HandleResult{Reply: "Язык: русский."}
	}

	if LooksLikeCategoryCatalog(text) {
		reply := a.execListCategories(tc)
		a.remember(chatID, text, reply)
		return HandleResult{Reply: reply}
	}

	// Off-topic stays pre-LLM: bot must not chat about anything outside family expenses.
	if IsAppOffTopic(text) {
		return HandleResult{Reply: OffTopicRefusal(tc.Language)}
	}

	// Explicit numeric ids («удали 1 и 2») — deterministic confirm flow, no LLM needed.
	if ids := ParseDeleteIDs(text); len(ids) > 0 {
		reply := a.deleteByIDs(ctx, chatID, ids, tc)
		a.remember(chatID, text, reply)
		return HandleResult{Reply: reply}
	}

	start := time.Now()
	reply, err := a.runWithLLM(ctx, chatID, text, tc)
	latency := time.Since(start)
	if err != nil {
		a.log.Warn("llm_failed",
			zap.Int64("user_id", tc.UserID),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
		if degraded := a.degradedCreate(ctx, text, tc); degraded != "" {
			return HandleResult{Reply: degraded}
		}
		return HandleResult{Reply: a.msg(tc.Language,
			"Сейчас не могу обработать запрос. Попробуйте позже или напишите сумму и валюту, например: 10 BYN кофе.",
			"I can't process that right now. Try later or send amount + currency, e.g. 10 BYN coffee.")}
	}
	a.log.Info("llm_ok", zap.Int64("user_id", tc.UserID), zap.Duration("latency", latency))
	return HandleResult{Reply: reply}
}

func (a *Agent) remember(chatID int64, user, assistant string) {
	a.history.Append(chatID,
		Message{Role: "user", Content: user},
		Message{Role: "assistant", Content: assistant},
	)
}

func (a *Agent) handlePending(ctx context.Context, chatID int64, text string, tc TrustedContext) (string, bool) {
	p := a.peekPending(chatID, tc.UserID)
	if p == nil {
		return "", false
	}
	if IsConfirm(text) {
		a.clearPending(chatID, tc.UserID)
		n, err := a.commitPending(ctx, p, tc)
		if err != nil {
			return a.msg(tc.Language, "Не удалось удалить: "+err.Error(), "Failed to delete: "+err.Error()), true
		}
		return a.msg(tc.Language,
			fmt.Sprintf("Удалил %d ваших трат (%s).", n, service.FormatBYN(p.total)),
			fmt.Sprintf("Deleted %d of your expenses (%s).", n, service.FormatBYN(p.total))), true
	}
	if IsCancel(text) {
		a.clearPending(chatID, tc.UserID)
		return a.msg(tc.Language, "Отменил удаление.", "Cancelled the delete."), true
	}
	a.clearPending(chatID, tc.UserID)
	return "", false
}

func (a *Agent) peekPending(chatID, userID int64) *pendingBulk {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	p := a.pending[chatID]
	if p == nil || p.userID != userID {
		return nil
	}
	cp := *p
	return &cp
}

func (a *Agent) setPending(chatID int64, p *pendingBulk) {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	a.pending[chatID] = p
}

func (a *Agent) clearPending(chatID, userID int64) {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	if p := a.pending[chatID]; p != nil && p.userID == userID {
		delete(a.pending, chatID)
	}
}

func (a *Agent) commitPending(ctx context.Context, p *pendingBulk, tc TrustedContext) (int, error) {
	if p.filter != nil {
		return a.expenses.DeleteMatching(ctx, tc.UserID, *p.filter)
	}
	return a.expenses.DeleteMany(ctx, tc.UserID, p.ids)
}

func (a *Agent) confirmDeletePrompt(tc TrustedContext, count int64, total int64) string {
	return a.msg(tc.Language,
		fmt.Sprintf("Удалить %d ваших трат на сумму %s? Чужие записи не затрагиваются. Напишите «да» или «нет».", count, service.FormatBYN(total)),
		fmt.Sprintf("Delete %d of your expenses totaling %s? Other users are not touched. Reply yes or no.", count, service.FormatBYN(total)))
}

func (a *Agent) deleteByIDs(ctx context.Context, chatID int64, ids []int64, tc TrustedContext) string {
	var found []int64
	var total int64
	var parts []string
	for _, id := range ids {
		e, err := a.expenses.GetByIDForUser(ctx, id, tc.UserID)
		if err != nil {
			parts = append(parts, a.msg(tc.Language,
				fmt.Sprintf("Трата #%d не найдена.", id),
				fmt.Sprintf("Expense #%d not found.", id)))
			continue
		}
		found = append(found, e.ID)
		total += e.AmountBYNKopecks
	}
	if len(found) == 0 {
		if len(parts) == 0 {
			return a.msg(tc.Language, "Трат нет (0). Нечего удалять.", "No expenses (0). Nothing to delete.")
		}
		return strings.Join(parts, "\n")
	}
	if len(found) == 1 && len(ids) == 1 {
		if err := a.expenses.Delete(ctx, found[0], tc.UserID); err != nil {
			return a.msg(tc.Language, "Не удалось удалить.", "Failed to delete.")
		}
		return a.msg(tc.Language,
			fmt.Sprintf("Удалил #%d (%s).", found[0], service.FormatBYN(total)),
			fmt.Sprintf("Deleted #%d (%s).", found[0], service.FormatBYN(total)))
	}
	a.setPending(chatID, &pendingBulk{userID: tc.UserID, ids: found, count: int64(len(found)), total: total})
	reply := a.confirmDeletePrompt(tc, int64(len(found)), total)
	if len(parts) > 0 {
		return strings.Join(parts, "\n") + "\n" + reply
	}
	return reply
}

func (a *Agent) runWithLLM(ctx context.Context, chatID int64, text string, tc TrustedContext) (string, error) {
	window := a.llmCfg.HistoryWindow
	if window <= 0 {
		window = 12
	}
	msgs := []Message{{Role: "system", Content: SystemPrompt(tc.Language, time.Now().In(a.loc))}}
	msgs = append(msgs, a.history.Recent(chatID, window)...)
	msgs = append(msgs, Message{Role: "user", Content: text})

	req := ChatRequest{
		Model:       a.llmCfg.Model,
		Messages:    msgs,
		Tools:       ToolDefinitions(),
		ToolChoice:  "required",
		Temperature: a.llmCfg.Temperature,
		MaxTokens:   a.llmCfg.MaxTokens,
	}
	if strings.Contains(a.llmCfg.Model, "oss") {
		req.ReasoningEffort = a.llmCfg.ReasoningEffort
	}

	resp, err := a.queue.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty llm response")
	}
	msg := resp.Choices[0].Message

	for i := range msg.ToolCalls {
		msg.ToolCalls[i].Function.Arguments = StripNullArgs(msg.ToolCalls[i].Function.Arguments)
	}

	if len(msg.ToolCalls) == 0 {
		content := SanitizeNoToolReply(text, msg.Content, tc.Language)
		a.remember(chatID, text, content)
		return content, nil
	}

	enrichCallsFromText(msg.ToolCalls, text)

	var parts []string
	for _, call := range msg.ToolCalls {
		validated, err := ValidateToolCall(call.Function.Name, call.Function.Arguments, tc)
		var out string
		if err != nil {
			a.log.Warn("tool_rejected", zap.Int64("user_id", tc.UserID), zap.String("tool", call.Function.Name), zap.Error(err))
			out = a.msg(tc.Language, "Запрос отклонён.", "Request rejected.")
		} else {
			a.log.Info("tool_call", zap.Int64("user_id", tc.UserID), zap.String("tool", validated.Name))
			out, err = a.execute(ctx, chatID, validated, tc)
			if err != nil {
				out = a.msg(tc.Language, "Ошибка: "+err.Error(), "Error: "+err.Error())
			}
		}
		parts = append(parts, out)
	}

	reply := strings.TrimSpace(strings.Join(parts, "\n"))
	if reply == "" {
		reply = a.msg(tc.Language, "Готово.", "Done.")
	}
	a.remember(chatID, text, reply)
	return reply, nil
}

func (a *Agent) degradedCreate(ctx context.Context, text string, tc TrustedContext) string {
	p := ParseSimpleCreate(text)
	if !p.OK {
		return ""
	}
	kopecks, err := service.ToBYNKopecks(p.Amount, p.Currency, a.rates)
	if err != nil {
		return ""
	}
	cat := service.InferCategory(text)
	e, err := a.expenses.Create(ctx, service.CreateExpenseInput{
		UserID: tc.UserID, AmountBYNKopecks: kopecks, Category: cat,
		Comment: p.Comment, SpentAt: time.Now().In(a.loc),
	})
	if err != nil {
		return ""
	}
	return a.msg(tc.Language,
		fmt.Sprintf("Добавлено (без ИИ): %s, %s.", service.FormatBYN(e.AmountBYNKopecks), e.Category),
		fmt.Sprintf("Added (no AI): %s, %s.", service.FormatBYN(e.AmountBYNKopecks), e.Category),
	)
}

func (a *Agent) execute(ctx context.Context, chatID int64, call *ValidatedCall, tc TrustedContext) (string, error) {
	switch call.Name {
	case ToolCreateExpense:
		return a.execCreate(ctx, call.Args, tc)
	case ToolListExpenses:
		return a.execList(ctx, call.Args, tc)
	case ToolUpdateExpense:
		return a.execUpdate(ctx, call.Args, tc)
	case ToolDeleteExpense:
		return a.execDelete(ctx, call.Args, tc)
	case ToolDeleteExpenses:
		return a.execDeleteExpenses(ctx, chatID, call.Args, tc)
	case ToolGetSummary:
		return a.execSummary(ctx, call.Args, tc)
	case ToolGetByCategory:
		return a.execByCategory(ctx, call.Args, tc)
	case ToolListCategories:
		return a.execListCategories(tc), nil
	case ToolReply:
		if s, _ := call.Args["text"].(string); strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s), nil
		}
		return a.msg(tc.Language, "Напишите трату или вопрос.", "Send an expense or a question."), nil
	case ToolRefuseOffTopic:
		return OffTopicRefusal(tc.Language), nil
	default:
		return "", fmt.Errorf("unknown tool")
	}
}

func (a *Agent) execCreate(ctx context.Context, args map[string]any, tc TrustedContext) (string, error) {
	var amount float64
	var currency domain.Currency = domain.CurrencyBYN
	comment, _ := args["comment"].(string)
	raw, _ := args["raw_text"].(string)

	if raw != "" {
		p := ParseSimpleCreate(raw)
		if p.OK {
			amount = p.Amount
			currency = p.Currency
			if comment == "" {
				comment = p.Comment
			}
		} else if amt := toFloat(args["amount"]); amt > 0 {
			amount = amt
		} else {
			return "", fmt.Errorf("could not parse amount")
		}
	} else {
		amount = toFloat(args["amount"])
	}
	if amount <= 0 {
		return "", fmt.Errorf("amount must be > 0")
	}

	if c, ok := args["currency"].(string); ok && c != "" {
		if code, ok := NormalizeCurrencyTypo(c); ok {
			c = code
		}
		parsed, err := service.ParseCurrency(c)
		if err != nil {
			return "", err
		}
		currency = parsed
	}
	if raw != "" {
		inferred := InferCurrency(raw)
		if inferred == domain.CurrencyBYN && currency == domain.CurrencyRUB {
			lower := strings.ToLower(raw)
			if !strings.Contains(lower, "российск") && !strings.Contains(lower, "рос ") && !latinRubRe.MatchString(lower) {
				currency = domain.CurrencyBYN
			}
		} else if inferred != domain.CurrencyBYN {
			currency = inferred
		}
	}

	kopecks, err := service.ToBYNKopecks(amount, currency, a.rates)
	if err != nil {
		return "", err
	}

	cat := domain.CategoryOther
	if c, ok := args["category"].(string); ok && c != "" {
		cat, err = service.ParseCategory(c)
		if err != nil {
			return "", err
		}
	} else {
		cat = service.InferCategory(raw + " " + comment)
	}

	var spent time.Time
	if s, ok := args["spent_at"].(string); ok {
		if d := ParseDate(s, a.loc); d != nil {
			spent = *d
		}
	}
	if spent.IsZero() && raw != "" {
		if strings.Contains(foldYo(raw), "вчера") || strings.Contains(strings.ToLower(raw), "yesterday") {
			spent = time.Now().In(a.loc).AddDate(0, 0, -1)
		}
	}

	e, err := a.expenses.Create(ctx, service.CreateExpenseInput{
		UserID: tc.UserID, AmountBYNKopecks: kopecks, Category: cat, SpentAt: spent, Comment: comment,
	})
	if err != nil {
		return "", err
	}
	return a.msg(tc.Language,
		fmt.Sprintf("Записал: %s · %s · %s%s", service.FormatBYN(e.AmountBYNKopecks), e.Category, e.SpentAt.Format("2006-01-02"), commentSuffix(e.Comment)),
		fmt.Sprintf("Saved: %s · %s · %s%s", service.FormatBYN(e.AmountBYNKopecks), e.Category, e.SpentAt.Format("2006-01-02"), commentSuffix(e.Comment)),
	), nil
}

func (a *Agent) execList(ctx context.Context, args map[string]any, tc TrustedContext) (string, error) {
	f := filterFromArgs(args, a.loc)
	if v := toInt(args["limit"]); v > 0 {
		f.Limit = int(v)
	} else {
		f.Limit = 10
	}
	if _, ok := args["offset"]; ok {
		f.Offset = int(toInt(args["offset"]))
	}

	res, err := a.expenses.List(ctx, tc.UserID, f)
	if err != nil {
		return "", err
	}
	if len(res.Items) == 0 {
		return a.formatEmpty(tc.Language, f), nil
	}
	var b strings.Builder
	for _, e := range res.Items {
		fmt.Fprintf(&b, "#%d %s %s %s%s\n", e.ID, e.SpentAt.Format("2006-01-02"), service.FormatBYN(e.AmountBYNKopecks), e.Category, commentSuffix(e.Comment))
	}
	fmt.Fprintf(&b, a.msg(tc.Language, "Всего: %d (показано %d)", "Total: %d (showing %d)"), res.Total, len(res.Items))
	return strings.TrimSpace(b.String()), nil
}

func (a *Agent) resolveTarget(ctx context.Context, args map[string]any, tc TrustedContext) (*domain.Expense, string, error) {
	if id := toInt(args["expense_id"]); id > 0 {
		e, err := a.expenses.GetByIDForUser(ctx, id, tc.UserID)
		return e, "", err
	}
	if useLast, _ := args["use_last"].(bool); useLast {
		e, err := a.expenses.GetLast(ctx, tc.UserID)
		return e, "", err
	}
	query, _ := args["query"].(string)
	if ids := ParseDeleteIDs(query); len(ids) == 1 {
		e, err := a.expenses.GetByIDForUser(ctx, ids[0], tc.UserID)
		return e, "", err
	}
	list, err := a.expenses.List(ctx, tc.UserID, domain.ExpenseFilter{Limit: 50, Query: strings.TrimSpace(query)})
	if err != nil {
		return nil, "", err
	}
	cands := FindCandidates(list.Items, query, 5)
	if len(cands) == 0 && query != "" {
		// Fallback: search all comments, not only the filtered page.
		all, err := a.expenses.List(ctx, tc.UserID, domain.ExpenseFilter{Limit: 50})
		if err != nil {
			return nil, "", err
		}
		cands = FindCandidates(all.Items, query, 5)
	}
	if len(cands) == 0 {
		return nil, FormatCandidates(nil, tc.Language), nil
	}
	if len(cands) == 1 {
		return &cands[0], "", nil
	}
	return nil, FormatCandidates(cands, tc.Language), nil
}

func (a *Agent) execUpdate(ctx context.Context, args map[string]any, tc TrustedContext) (string, error) {
	e, clarify, err := a.resolveTarget(ctx, args, tc)
	if err != nil {
		return "", err
	}
	if clarify != "" {
		return clarify, nil
	}
	in := service.UpdateExpenseInput{ID: e.ID, UserID: tc.UserID}
	if amt := toFloat(args["amount"]); amt > 0 {
		cur := domain.CurrencyBYN
		if c, ok := args["currency"].(string); ok && c != "" {
			if code, ok := NormalizeCurrencyTypo(c); ok {
				c = code
			}
			cur, _ = service.ParseCurrency(c)
		} else if q, ok := args["query"].(string); ok {
			// Only infer currency from query when the model omitted it — never override amount.
			cur = InferCurrency(q)
		}
		k, err := service.ToBYNKopecks(amt, cur, a.rates)
		if err != nil {
			return "", err
		}
		in.AmountBYNKopecks = &k
	}
	if c, ok := args["category"].(string); ok && c != "" {
		cat, err := service.ParseCategory(c)
		if err != nil {
			return "", err
		}
		in.Category = &cat
	}
	if s, ok := args["spent_at"].(string); ok {
		if d := ParseDate(s, a.loc); d != nil {
			in.SpentAt = d
		}
	}
	if c, ok := args["comment"].(string); ok {
		in.Comment = &c
	}
	updated, err := a.expenses.Update(ctx, in)
	if err != nil {
		return "", err
	}
	return a.msg(tc.Language,
		fmt.Sprintf("Обновил #%d: %s.", updated.ID, service.FormatBYN(updated.AmountBYNKopecks)),
		fmt.Sprintf("Updated #%d: %s.", updated.ID, service.FormatBYN(updated.AmountBYNKopecks)),
	), nil
}

func (a *Agent) execDelete(ctx context.Context, args map[string]any, tc TrustedContext) (string, error) {
	e, clarify, err := a.resolveTarget(ctx, args, tc)
	if err != nil {
		return "", err
	}
	if clarify != "" {
		return clarify, nil
	}
	if err := a.expenses.Delete(ctx, e.ID, tc.UserID); err != nil {
		return "", err
	}
	return a.msg(tc.Language,
		fmt.Sprintf("Удалил #%d (%s).", e.ID, service.FormatBYN(e.AmountBYNKopecks)),
		fmt.Sprintf("Deleted #%d (%s).", e.ID, service.FormatBYN(e.AmountBYNKopecks)),
	), nil
}

func (a *Agent) execDeleteExpenses(ctx context.Context, chatID int64, args map[string]any, tc TrustedContext) (string, error) {
	all, _ := args["all"].(bool)
	var f domain.ExpenseFilter
	if !all {
		f = filterFromArgs(args, a.loc)
	}
	sum, err := a.expenses.GetSummary(ctx, tc.UserID, f)
	if err != nil {
		return "", err
	}
	if sum.Count == 0 {
		if all {
			return a.msg(tc.Language, "Трат нет (0). Нечего удалять.", "No expenses (0). Nothing to delete."), nil
		}
		return a.formatEmpty(tc.Language, f) + a.msg(tc.Language, " Нечего удалять.", " Nothing to delete."), nil
	}
	fp := f
	a.setPending(chatID, &pendingBulk{
		userID: tc.UserID,
		filter: &fp,
		count:  sum.Count,
		total:  sum.Total,
	})
	return a.confirmDeletePrompt(tc, sum.Count, sum.Total), nil
}

func (a *Agent) execSummary(ctx context.Context, args map[string]any, tc TrustedContext) (string, error) {
	f := filterFromArgs(args, a.loc)
	sum, err := a.expenses.GetSummary(ctx, tc.UserID, f)
	if err != nil {
		return "", err
	}
	if sum.Count == 0 {
		return a.formatEmpty(tc.Language, f), nil
	}
	scope := a.formatScope(tc.Language, f)
	return a.msg(tc.Language,
		fmt.Sprintf("%s: %s, записей: %d.", scope, service.FormatBYN(sum.Total), sum.Count),
		fmt.Sprintf("%s: %s, entries: %d.", scope, service.FormatBYN(sum.Total), sum.Count),
	), nil
}

func (a *Agent) execByCategory(ctx context.Context, args map[string]any, tc TrustedContext) (string, error) {
	f := filterFromArgs(args, a.loc)
	rows, err := a.expenses.GetByCategory(ctx, tc.UserID, f)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return a.formatEmpty(tc.Language, f), nil
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s: %s (%d)\n", r.Category.Label(tc.Language), service.FormatBYN(r.Total), r.Count)
	}
	return strings.TrimSpace(b.String()), nil
}

func (a *Agent) execListCategories(tc TrustedContext) string {
	var b strings.Builder
	if tc.Language == domain.LangEN {
		b.WriteString("Expense categories:\n")
	} else {
		b.WriteString("Категории трат:\n")
	}
	for _, c := range domain.OrderedCategories {
		fmt.Fprintf(&b, "• %s — %s\n", c, c.Label(tc.Language))
	}
	return strings.TrimSpace(b.String())
}

func (a *Agent) formatEmpty(lang domain.Language, f domain.ExpenseFilter) string {
	if q := strings.TrimSpace(f.Query); q != "" {
		return a.msg(lang, fmt.Sprintf("По «%s» трат нет (0).", q), fmt.Sprintf("No expenses matching %q (0).", q))
	}
	if f.Category != nil {
		lab := f.Category.Label(lang)
		return a.msg(lang, fmt.Sprintf("По категории «%s» трат нет (0).", lab), fmt.Sprintf("No expenses in %s (0).", lab))
	}
	return a.msg(lang, "Трат нет (0).", "No expenses (0).")
}

func (a *Agent) formatScope(lang domain.Language, f domain.ExpenseFilter) string {
	if q := strings.TrimSpace(f.Query); q != "" {
		return a.msg(lang, "По «"+q+"»", "For "+q)
	}
	if f.Category != nil {
		lab := f.Category.Label(lang)
		return a.msg(lang, "Категория «"+lab+"»", "Category "+lab)
	}
	return a.msg(lang, "Итого", "Total")
}

func (a *Agent) msg(lang domain.Language, ru, en string) string {
	if lang == domain.LangEN {
		return en
	}
	return ru
}

func commentSuffix(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	return " · " + c
}

// ToolArgsJSON helper for tests.
func ToolArgsJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
