package domain

import "time"

type Category string

const (
	CategoryStores        Category = "stores"
	CategoryCafes         Category = "cafes"
	CategoryTransport     Category = "transport"
	CategoryHousing       Category = "housing"
	CategoryEntertainment Category = "entertainment"
	CategoryHealth        Category = "health"
	CategoryShopping      Category = "shopping"
	CategoryHousehold     Category = "household"
	CategoryOther         Category = "other"
)

var ValidCategories = map[Category]struct{}{
	CategoryStores:        {},
	CategoryCafes:         {},
	CategoryTransport:     {},
	CategoryHousing:       {},
	CategoryEntertainment: {},
	CategoryHealth:        {},
	CategoryShopping:      {},
	CategoryHousehold:     {},
	CategoryOther:         {},
}

func (c Category) Valid() bool {
	_, ok := ValidCategories[c]
	return ok
}

// OrderedCategories is the stable display order for menus and prompts.
var OrderedCategories = []Category{
	CategoryStores, CategoryCafes, CategoryTransport, CategoryHousing, CategoryEntertainment,
	CategoryHealth, CategoryShopping, CategoryHousehold, CategoryOther,
}

var categoryLabels = map[Category][2]string{
	CategoryStores:        {"Продукты и магазины", "Groceries & stores"},
	CategoryCafes:         {"Кафе и рестораны", "Cafes & restaurants"},
	CategoryTransport:     {"Транспорт", "Transport"},
	CategoryHousing:       {"Жильё и коммуналка", "Housing & utilities"},
	CategoryEntertainment: {"Развлечения", "Entertainment"},
	CategoryHealth:        {"Здоровье", "Health"},
	CategoryShopping:      {"Покупки (одежда, техника, маркетплейсы)", "Shopping"},
	CategoryHousehold:     {"Быт и хозтовары", "Household"},
	CategoryOther:         {"Другое", "Other"},
}

func (c Category) Label(lang Language) string {
	l, ok := categoryLabels[c]
	if !ok {
		return string(c)
	}
	if lang == LangEN {
		return l[1]
	}
	return l[0]
}

type Currency string

const (
	CurrencyBYN Currency = "BYN"
	CurrencyUSD Currency = "USD"
	CurrencyRUB Currency = "RUB"
)

type Expense struct {
	ID                int64     `db:"id" json:"id"`
	UserID            int64     `db:"user_id" json:"user_id"`
	AmountBYNKopecks  int64     `db:"amount_byn_kopecks" json:"amount_byn_kopecks"`
	Category          Category  `db:"category" json:"category"`
	SpentAt           time.Time `db:"spent_at" json:"spent_at"`
	Comment           string    `db:"comment" json:"comment"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
	// Joined for admin panel.
	UserDisplayName string `db:"user_display_name" json:"user_display_name,omitempty"`
}

type ExpenseFilter struct {
	UserID    *int64
	Category  *Category
	FromDate  *time.Time
	ToDate    *time.Time
	// Query is free text matched against the comment (stemmed, case-insensitive).
	Query  string
	Limit  int
	Offset int
}

type CategorySummary struct {
	Category Category `db:"category" json:"category"`
	Total    int64    `db:"total" json:"total"`
	Count    int64    `db:"count" json:"count"`
}

type PeriodSummary struct {
	Total  int64 `json:"total"`
	Count  int64 `json:"count"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}
