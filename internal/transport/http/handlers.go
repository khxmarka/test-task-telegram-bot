package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
	"github.com/testTelegramApp/expenses-bot/internal/service"
)

type API struct {
	Expenses *service.ExpenseService
	Loc      *time.Location
}

func (a *API) Routes(r chi.Router) {
	r.Get("/expenses", a.listExpenses)
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

func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
