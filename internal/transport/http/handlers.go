package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/testTelegramApp/expenses-bot/internal/config"
	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
	"github.com/testTelegramApp/expenses-bot/internal/service"
)

type API struct {
	Expenses *service.ExpenseService
	Rates    config.CurrencyConfig
	Loc      *time.Location
}

func (a *API) Routes(r chi.Router) {
	r.Get("/expenses", a.listExpenses)
	r.Post("/expenses", a.createExpense)
	r.Patch("/expenses/{id}", a.updateExpense)
	r.Delete("/expenses/{id}", a.deleteExpense)
	r.Get("/expenses/summary", a.summary)
	r.Get("/expenses/by-category", a.byCategory)
	r.Get("/members", a.members)
}

func (a *API) listExpenses(w http.ResponseWriter, r *http.Request) {
	f, err := a.parseFilter(r)
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := a.Expenses.ListFamily(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type expenseWriteBody struct {
	UserID   int64   `json:"user_id"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Category string  `json:"category"`
	SpentAt  string  `json:"spent_at"`
	Comment  string  `json:"comment"`
}

func (a *API) createExpense(w http.ResponseWriter, r *http.Request) {
	var body expenseWriteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, fmt.Errorf("%w: invalid json", errorsx.ErrValidation))
		return
	}
	cat, err := service.ParseCategory(body.Category)
	if err != nil {
		writeError(w, err)
		return
	}
	cur, err := service.ParseCurrency(body.Currency)
	if err != nil {
		writeError(w, err)
		return
	}
	kopecks, err := service.ToBYNKopecks(body.Amount, cur, a.rates())
	if err != nil {
		writeError(w, err)
		return
	}
	spent, err := a.parseSpentAt(body.SpentAt)
	if err != nil {
		writeError(w, err)
		return
	}
	e, err := a.Expenses.Create(r.Context(), service.CreateExpenseInput{
		UserID:           body.UserID,
		AmountBYNKopecks: kopecks,
		Category:         cat,
		SpentAt:          spent,
		Comment:          body.Comment,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (a *API) updateExpense(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, fmt.Errorf("%w: invalid id", errorsx.ErrValidation))
		return
	}
	var body expenseWriteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, fmt.Errorf("%w: invalid json", errorsx.ErrValidation))
		return
	}
	in := service.UpdateExpenseInput{ID: id}
	if body.Amount > 0 {
		cur, err := service.ParseCurrency(body.Currency)
		if err != nil {
			writeError(w, err)
			return
		}
		kopecks, err := service.ToBYNKopecks(body.Amount, cur, a.rates())
		if err != nil {
			writeError(w, err)
			return
		}
		in.AmountBYNKopecks = &kopecks
	}
	if body.Category != "" {
		cat, err := service.ParseCategory(body.Category)
		if err != nil {
			writeError(w, err)
			return
		}
		in.Category = &cat
	}
	if body.SpentAt != "" {
		spent, err := a.parseSpentAt(body.SpentAt)
		if err != nil {
			writeError(w, err)
			return
		}
		in.SpentAt = &spent
	}
	// Panel always sends comment (may be empty string).
	c := body.Comment
	in.Comment = &c
	e, err := a.Expenses.UpdateAsAdmin(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (a *API) deleteExpense(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, fmt.Errorf("%w: invalid id", errorsx.ErrValidation))
		return
	}
	if err := a.Expenses.DeleteAsAdmin(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) summary(w http.ResponseWriter, r *http.Request) {
	f, err := a.parseFilter(r)
	if err != nil {
		writeError(w, err)
		return
	}
	sum, err := a.Expenses.GetFamilySummary(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (a *API) byCategory(w http.ResponseWriter, r *http.Request) {
	f, err := a.parseFilter(r)
	if err != nil {
		writeError(w, err)
		return
	}
	rows, err := a.Expenses.GetFamilyByCategory(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (a *API) members(w http.ResponseWriter, r *http.Request) {
	users, err := a.Expenses.ListMembers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

func (a *API) parseFilter(r *http.Request) (domain.ExpenseFilter, error) {
	f := domain.ExpenseFilter{}
	q := r.URL.Query()
	if v := q.Get("user_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return f, fmt.Errorf("%w: invalid user_id", errorsx.ErrValidation)
		}
		f.UserID = &id
	}
	if v := q.Get("category"); v != "" {
		cat, err := service.ParseCategory(v)
		if err != nil {
			return f, err
		}
		f.Category = &cat
	}
	_, from, to, err := a.parsePeriod(r)
	if err != nil {
		return f, err
	}
	f.FromDate, f.ToDate = from, to
	return f, nil
}

func (a *API) parsePeriod(r *http.Request) (domain.ExpenseFilter, *time.Time, *time.Time, error) {
	q := r.URL.Query()
	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, a.Loc)
		if err != nil {
			return domain.ExpenseFilter{}, nil, nil, fmt.Errorf("%w: invalid from date", errorsx.ErrValidation)
		}
		from = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, a.Loc)
		if err != nil {
			return domain.ExpenseFilter{}, nil, nil, fmt.Errorf("%w: invalid to date", errorsx.ErrValidation)
		}
		to = &t
	}
	return domain.ExpenseFilter{}, from, to, nil
}

func (a *API) parseSpentAt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, a.Loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid spent_at", errorsx.ErrValidation)
	}
	return t, nil
}

func (a *API) rates() config.CurrencyConfig {
	if a.Rates.BYNToKopecks == 0 {
		return config.CurrencyConfig{BYNToKopecks: 100, USDToBYNKopecks: 300, RUBToBYNKopecks: 4}
	}
	return a.Rates
}

func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
