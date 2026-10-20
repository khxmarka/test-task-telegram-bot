package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

func TestCreateListUpdateDeleteSummary(t *testing.T) {
	expRepo := newMockExpenseRepo()
	userRepo := newMockUserRepo()
	svc := NewExpenseService(expRepo, userRepo, time.UTC)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateExpenseInput{
		UserID: 1, AmountBYNKopecks: 35000, Category: domain.CategoryCafes,
		Comment: "coffee",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatal("expected id")
	}

	list, err := svc.List(ctx, 1, domain.ExpenseFilter{Limit: 10})
	if err != nil || list.Total != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}

	amt := int64(45000)
	updated, err := svc.Update(ctx, UpdateExpenseInput{ID: created.ID, UserID: 1, AmountBYNKopecks: &amt})
	if err != nil || updated.AmountBYNKopecks != 45000 {
		t.Fatalf("update: %+v %v", updated, err)
	}

	sum, err := svc.GetSummary(ctx, 1, domain.ExpenseFilter{})
	if err != nil || sum.Total != 45000 || sum.Count != 1 {
		t.Fatalf("summary: %+v %v", sum, err)
	}

	byCat, err := svc.GetByCategory(ctx, 1, domain.ExpenseFilter{})
	if err != nil || len(byCat) != 1 || byCat[0].Category != domain.CategoryCafes {
		t.Fatalf("byCat: %+v %v", byCat, err)
	}
	cat := domain.CategoryCafes
	sumCat, err := svc.GetSummary(ctx, 1, domain.ExpenseFilter{Category: &cat})
	if err != nil || sumCat.Total != 45000 {
		t.Fatalf("summary by cat: %+v %v", sumCat, err)
	}
	other := domain.CategoryTransport
	sumEmpty, err := svc.GetSummary(ctx, 1, domain.ExpenseFilter{Category: &other})
	if err != nil || sumEmpty.Total != 0 {
		t.Fatalf("expected 0 for other cat: %+v %v", sumEmpty, err)
	}
	sumQuery, err := svc.GetSummary(ctx, 1, domain.ExpenseFilter{Query: "coff"})
	if err != nil || sumQuery.Count != 1 {
		t.Fatalf("summary by comment: %+v %v", sumQuery, err)
	}
	sumNoMatch, err := svc.GetSummary(ctx, 1, domain.ExpenseFilter{Query: "ноги"})
	if err != nil || sumNoMatch.Count != 0 {
		t.Fatalf("expected no match: %+v %v", sumNoMatch, err)
	}

	if err := svc.Delete(ctx, created.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetByIDForUser(ctx, created.ID, 1); !errors.Is(err, errorsx.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestFamilySummaryHonorsMemberAndCategory(t *testing.T) {
	svc := NewExpenseService(newMockExpenseRepo(), newMockUserRepo(), time.UTC)
	ctx := context.Background()
	_, _ = svc.Create(ctx, CreateExpenseInput{UserID: 1, AmountBYNKopecks: 1000, Category: domain.CategoryCafes})
	_, _ = svc.Create(ctx, CreateExpenseInput{UserID: 2, AmountBYNKopecks: 4000, Category: domain.CategoryTransport})

	all, err := svc.GetFamilySummary(ctx, domain.ExpenseFilter{})
	if err != nil || all.Total != 5000 || all.Count != 2 {
		t.Fatalf("all: %+v %v", all, err)
	}
	uid := int64(1)
	one, err := svc.GetFamilySummary(ctx, domain.ExpenseFilter{UserID: &uid})
	if err != nil || one.Total != 1000 || one.Count != 1 {
		t.Fatalf("member: %+v %v", one, err)
	}
	cat := domain.CategoryTransport
	pie, err := svc.GetFamilyByCategory(ctx, domain.ExpenseFilter{Category: &cat})
	if err != nil || len(pie) != 1 || pie[0].Total != 4000 {
		t.Fatalf("pie: %+v %v", pie, err)
	}
}

func TestUpdateForbiddenOtherUser(t *testing.T) {
	expRepo := newMockExpenseRepo()
	svc := NewExpenseService(expRepo, newMockUserRepo(), time.UTC)
	ctx := context.Background()
	e, _ := svc.Create(ctx, CreateExpenseInput{UserID: 1, AmountBYNKopecks: 100, Category: domain.CategoryOther})
	amt := int64(200)
	_, err := svc.Update(ctx, UpdateExpenseInput{ID: e.ID, UserID: 2, AmountBYNKopecks: &amt})
	if !errors.Is(err, errorsx.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
}

func TestBulkDeleteOnlyOwn(t *testing.T) {
	svc := NewExpenseService(newMockExpenseRepo(), newMockUserRepo(), time.UTC)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, _ = svc.Create(ctx, CreateExpenseInput{UserID: 1, AmountBYNKopecks: 100, Category: domain.CategoryOther})
	}
	foreign, _ := svc.Create(ctx, CreateExpenseInput{UserID: 2, AmountBYNKopecks: 100, Category: domain.CategoryOther})

	items, err := svc.FindForBulkDelete(ctx, 1, domain.ExpenseFilter{})
	if err != nil || len(items) != 3 {
		t.Fatalf("find: %d %v", len(items), err)
	}
	ids := []int64{foreign.ID}
	for _, e := range items {
		ids = append(ids, e.ID)
	}
	if _, err := svc.DeleteMany(ctx, 1, ids); !errors.Is(err, errorsx.ErrForbidden) {
		t.Fatalf("foreign id must be forbidden, got %v", err)
	}
	if _, err := svc.GetByIDForUser(ctx, foreign.ID, 2); err != nil {
		t.Fatalf("foreign expense must survive: %v", err)
	}
	n, err := svc.DeleteMany(ctx, 1, ids[1:])
	if err != nil || n != 3 {
		t.Fatalf("deleted %d %v", n, err)
	}
}

func TestDeleteMatchingPagesAllOwn(t *testing.T) {
	old := bulkDeletePage
	bulkDeletePage = 2
	t.Cleanup(func() { bulkDeletePage = old })

	svc := NewExpenseService(newMockExpenseRepo(), newMockUserRepo(), time.UTC)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = svc.Create(ctx, CreateExpenseInput{UserID: 1, AmountBYNKopecks: 100, Category: domain.CategoryOther})
	}
	_, _ = svc.Create(ctx, CreateExpenseInput{UserID: 2, AmountBYNKopecks: 999, Category: domain.CategoryOther})

	n, err := svc.DeleteMatching(ctx, 1, domain.ExpenseFilter{})
	if err != nil || n != 5 {
		t.Fatalf("deleted %d %v", n, err)
	}
	left, err := svc.List(ctx, 1, domain.ExpenseFilter{Limit: 50})
	if err != nil || left.Total != 0 {
		t.Fatalf("own leftover %+v %v", left, err)
	}
	foreign, err := svc.List(ctx, 2, domain.ExpenseFilter{Limit: 50})
	if err != nil || foreign.Total != 1 {
		t.Fatalf("foreign must survive %+v %v", foreign, err)
	}
}

func TestInferCategoryDeterministic(t *testing.T) {
	for i := 0; i < 50; i++ {
		if got := InferCategory("кофе в магазине"); got != domain.CategoryCafes {
			t.Fatalf("iteration %d: got %s", i, got)
		}
	}
}

func TestCreateValidation(t *testing.T) {
	svc := NewExpenseService(newMockExpenseRepo(), newMockUserRepo(), time.UTC)
	_, err := svc.Create(context.Background(), CreateExpenseInput{UserID: 1, AmountBYNKopecks: 0, Category: domain.CategoryOther})
	if !errors.Is(err, errorsx.ErrValidation) {
		t.Fatalf("want validation, got %v", err)
	}
}

func TestInferCategory(t *testing.T) {
	if InferCategory("кофе 350") != domain.CategoryCafes {
		t.Fatal("expected cafes")
	}
	if InferCategory("такси вчера") != domain.CategoryTransport {
		t.Fatal("expected transport")
	}
	if InferCategory("что-то странное") != domain.CategoryOther {
		t.Fatal("expected other")
	}
	if InferCategory("жила в минске") != domain.CategoryOther {
		t.Fatal("жила must not become housing")
	}
	if InferCategory("нож") != domain.CategoryHousehold {
		t.Fatal("нож is a household item on create")
	}
	if NamedCategory("траты на нож") != nil {
		t.Fatal("нож is not a named category filter")
	}
	if c := NamedCategory("сколько на жилье"); c == nil || *c != domain.CategoryHousing {
		t.Fatal("жилье is a named category")
	}
}
