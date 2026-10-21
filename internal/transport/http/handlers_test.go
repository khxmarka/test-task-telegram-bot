package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/testTelegramApp/expenses-bot/internal/config"
	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
	"github.com/testTelegramApp/expenses-bot/internal/service"
)

type stubExpRepo struct {
	items []domain.Expense
}

func (s *stubExpRepo) Create(ctx context.Context, e *domain.Expense) error {
	e.ID = int64(len(s.items) + 1)
	e.CreatedAt = time.Now().UTC()
	e.UpdatedAt = e.CreatedAt
	s.items = append(s.items, *e)
	return nil
}
func (s *stubExpRepo) GetByID(ctx context.Context, id int64) (*domain.Expense, error) {
	for i := range s.items {
		if s.items[i].ID == id {
			cp := s.items[i]
			return &cp, nil
		}
	}
	return nil, errorsx.ErrNotFound
}
func (s *stubExpRepo) Update(ctx context.Context, e *domain.Expense) error {
	for i := range s.items {
		if s.items[i].ID == e.ID {
			s.items[i] = *e
			return nil
		}
	}
	return errorsx.ErrNotFound
}
func (s *stubExpRepo) Delete(ctx context.Context, id, userID int64) error { return nil }
func (s *stubExpRepo) DeleteByID(ctx context.Context, id int64) error {
	for i, e := range s.items {
		if e.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return nil
		}
	}
	return errorsx.ErrNotFound
}
func (s *stubExpRepo) match(e domain.Expense, f domain.ExpenseFilter) bool {
	if f.UserID != nil && e.UserID != *f.UserID {
		return false
	}
	if f.Category != nil && e.Category != *f.Category {
		return false
	}
	if f.FromDate != nil && e.SpentAt.Before(*f.FromDate) {
		return false
	}
	if f.ToDate != nil && e.SpentAt.After(*f.ToDate) {
		return false
	}
	return true
}

func (s *stubExpRepo) filter(f domain.ExpenseFilter) []domain.Expense {
	var out []domain.Expense
	for _, e := range s.items {
		if s.match(e, f) {
			out = append(out, e)
		}
	}
	return out
}

func (s *stubExpRepo) List(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error) {
	return s.filter(f), nil
}
func (s *stubExpRepo) Count(ctx context.Context, f domain.ExpenseFilter) (int64, error) {
	return int64(len(s.filter(f))), nil
}
func (s *stubExpRepo) GetLastByUser(ctx context.Context, userID int64) (*domain.Expense, error) {
	return nil, errorsx.ErrNotFound
}
func (s *stubExpRepo) Summary(ctx context.Context, f domain.ExpenseFilter) (int64, int64, error) {
	items := s.filter(f)
	var total int64
	for _, e := range items {
		total += e.AmountBYNKopecks
	}
	return total, int64(len(items)), nil
}
func (s *stubExpRepo) ByCategory(ctx context.Context, f domain.ExpenseFilter) ([]domain.CategorySummary, error) {
	agg := map[domain.Category]*domain.CategorySummary{}
	for _, e := range s.filter(f) {
		row, ok := agg[e.Category]
		if !ok {
			row = &domain.CategorySummary{Category: e.Category}
			agg[e.Category] = row
		}
		row.Total += e.AmountBYNKopecks
		row.Count++
	}
	var out []domain.CategorySummary
	for _, row := range agg {
		out = append(out, *row)
	}
	return out, nil
}
func (s *stubExpRepo) ListFamily(ctx context.Context, f domain.ExpenseFilter) ([]domain.Expense, error) {
	return s.filter(f), nil
}

type stubUserRepo struct{}

func (s *stubUserRepo) UpsertByTelegram(ctx context.Context, telegramID int64, displayName string, username *string, lang domain.Language) (*domain.User, error) {
	return &domain.User{ID: 1}, nil
}
func (s *stubUserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	return &domain.User{ID: id}, nil
}
func (s *stubUserRepo) UpdateLanguage(ctx context.Context, id int64, lang domain.Language) error {
	return nil
}
func (s *stubUserRepo) ListMembers(ctx context.Context) ([]domain.User, error) {
	return []domain.User{{ID: 1, DisplayName: "Head", IsFamilyHead: true}}, nil
}

func setupRouter() http.Handler {
	exp := &stubExpRepo{items: []domain.Expense{
		{
			ID: 1, UserID: 1, AmountBYNKopecks: 35000, Category: domain.CategoryCafes,
			SpentAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Comment: "coffee",
			UserDisplayName: "Head",
		},
		{
			ID: 2, UserID: 2, AmountBYNKopecks: 10000, Category: domain.CategoryTransport,
			SpentAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Comment: "taxi",
			UserDisplayName: "Member",
		},
	}}
	svc := service.NewExpenseService(exp, &stubUserRepo{}, time.UTC)
	api := &API{Expenses: svc, Loc: time.UTC, Rates: config.CurrencyConfig{BYNToKopecks: 100}}
	r := chi.NewRouter()
	r.Use(CORS)
	r.Get("/healthz", Healthz)
	r.Route("/api/v1", func(ar chi.Router) {
		ar.Use(BasicAuth("admin", "secret"))
		api.Routes(ar)
	})
	return r
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	setupRouter().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestListUnauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/expenses", nil)
	rr := httptest.NewRecorder()
	setupRouter().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestListOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/expenses", nil)
	req.SetBasicAuth("admin", "secret")
	rr := httptest.NewRecorder()
	setupRouter().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	items, _ := body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items=%v", body)
	}
}

func TestSummaryOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/expenses/summary", nil)
	req.SetBasicAuth("admin", "secret")
	rr := httptest.NewRecorder()
	setupRouter().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestBadCategory(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/expenses?category=food", nil)
	req.SetBasicAuth("admin", "secret")
	rr := httptest.NewRecorder()
	setupRouter().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestBadUserIDAndDateAreValidation(t *testing.T) {
	h := setupRouter()
	for _, path := range []string{
		"/api/v1/expenses?user_id=abc",
		"/api/v1/expenses/summary?user_id=abc",
		"/api/v1/expenses/summary?from=not-a-date",
		"/api/v1/expenses/by-category?to=32.13.2026",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.SetBasicAuth("admin", "secret")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d body %s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestSummaryAndPieHonorMemberAndCategory(t *testing.T) {
	h := setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/expenses/summary?user_id=1&category=cafes", nil)
	req.SetBasicAuth("admin", "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var sum domain.PeriodSummary
	if err := json.Unmarshal(rr.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Total != 35000 || sum.Count != 1 {
		t.Fatalf("summary %+v", sum)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/expenses/by-category?user_id=2", nil)
	req.SetBasicAuth("admin", "secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Items []domain.CategorySummary `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Category != domain.CategoryTransport || body.Items[0].Total != 10000 {
		t.Fatalf("pie %+v", body.Items)
	}
}

func TestCreateUpdateDeleteExpense(t *testing.T) {
	h := setupRouter()

	createBody := `{"user_id":1,"amount":12.5,"currency":"BYN","category":"cafes","spent_at":"2026-10-10","comment":"tea"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/expenses", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status %d body %s", rr.Code, rr.Body.String())
	}
	var created domain.Expense
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.AmountBYNKopecks != 1250 {
		t.Fatalf("created %+v", created)
	}

	patch := `{"amount":15,"currency":"BYN","category":"stores","spent_at":"2026-10-11","comment":"tea+"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/expenses/"+strconv.FormatInt(created.ID, 10), bytes.NewBufferString(patch))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("patch status %d body %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/expenses/"+strconv.FormatInt(created.ID, 10), nil)
	req.SetBasicAuth("admin", "secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("delete status %d body %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/expenses/99999", nil)
	req.SetBasicAuth("admin", "secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("delete missing status %d", rr.Code)
	}
}

func TestCreateValidation(t *testing.T) {
	h := setupRouter()
	cases := []struct {
		body string
	}{
		{`{"user_id":0,"amount":10,"category":"cafes"}`},
		{`{"user_id":1,"amount":-1,"category":"cafes"}`},
		{`{"user_id":1,"amount":10,"category":"food"}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/expenses", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("admin", "secret")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status %d", tc.body, rr.Code)
		}
	}
}
