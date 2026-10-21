package service

import (
	"strings"

	"github.com/testTelegramApp/expenses-bot/internal/domain"
)

func foldYo(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "ё", "е")
}

// names = explicit category mention (NL filters). items = create-only hints (нож, лего).
// One table so agent and infer cannot drift.
var categoryLexicon = []struct {
	cat   domain.Category
	names []string
	items []string
}{
	{domain.CategoryCafes, []string{
		"кафе", "кофе", "ресторан", "lunch", "обед", "ужин", "завтрак", "cafe", "coffee",
	}, []string{
		"кофейн", "пицц", "pizza", "бар ", "bar", "столов", "суши", "бургер", "шаурм", "доставк еды", "фастфуд",
	}},
	{domain.CategoryTransport, []string{
		"такси", "транспорт", "metro", "метро", "бензин", "transport", "uber", "парковк", "проезд", "автобус",
	}, []string{
		"taxi", "яндекс go", "bus", "топлив", "fuel", "parking", "электрич", "поезд", "билет на", "каршеринг",
	}},
	{domain.CategoryHousing, []string{
		"жилье", "жилья", "жилью", "housing", "аренд", "квартир", "жкх", "коммунал", "ипотек",
	}, []string{
		"rent", "mortgage", "свет за", "электроэнерг", "интернет", "газ за", "отоплен",
	}},
	{domain.CategoryHealth, []string{
		"здоров", "аптек", "врач", "health", "клиник", "лекарств", "стоматолог", "больниц",
	}, []string{
		"pharmacy", "doctor", "clinic", "таблет", "medicine", "анализ", "витамин",
	}},
	{domain.CategoryEntertainment, []string{
		"развлеч", "кино", "entertainment", "театр", "концерт", "netflix", "spotify", "боулинг",
	}, []string{
		"cinema", "игр", "game", "подписк", "лего", "lego", "музей", "парк аттракц",
	}},
	{domain.CategoryShopping, []string{
		"шопинг", "одежд", "обув", "shopping", "ozon", "озон", "wildberries", "вайлдберриз", "маркетплейс",
	}, []string{
		"clothes", "shoes", "кроссовк", "куртк", "amazon", "телефон", "наушник", "ноутбук", "подарок",
	}},
	{domain.CategoryHousehold, []string{
		"бытов", "хозяйств", "хозтовар", "household", "бытовая химия",
	}, []string{
		"моющ", "cleaning", "туалетн", "paper", "мыло", "порошок",
		"посуд", "вилк", "ложк", "тарелк", "кастрюл", "сковород", "нож", "губк", "лампоч",
	}},
	{domain.CategoryStores, []string{
		"магазин", "продукт", "grocery", "store", "супермаркет", "евроопт", "магнит", "пятерочк",
		// Category synonyms (not product comments): «на еду» → stores filter, not query=еду.
		"еда", "еду", "едой", "еде", "food", "foods",
	}, []string{
		"хлеб", "молоко", "овощ", "фрукт",
	}},
}

func containsFolded(text, needle string) bool {
	return strings.Contains(text, foldYo(needle))
}

// InferCategory picks a category from free text; falls back to other.
func InferCategory(text string) domain.Category {
	lower := foldYo(text)
	for _, rule := range categoryLexicon {
		for _, w := range rule.names {
			if containsFolded(lower, w) {
				return rule.cat
			}
		}
		for _, w := range rule.items {
			if containsFolded(lower, w) {
				return rule.cat
			}
		}
	}
	return domain.CategoryOther
}

// NamedCategory is an explicit category mention (nil = none). Item names like «нож» do not count.
func NamedCategory(text string) *domain.Category {
	lower := foldYo(text)
	for _, rule := range categoryLexicon {
		for _, k := range rule.names {
			if containsFolded(lower, k) {
				c := rule.cat
				return &c
			}
		}
	}
	return nil
}

// IsNamedCategoryToken is true for enum codes and name fragments («жилье», «такси», «еду»).
func IsNamedCategoryToken(tok string) bool {
	t := foldYo(tok)
	for _, c := range domain.OrderedCategories {
		if t == string(c) {
			return true
		}
	}
	for _, rule := range categoryLexicon {
		for _, k := range rule.names {
			key := foldYo(k)
			if t == key {
				return true
			}
			// Prefix only for longer stems so «жила» ≠ «жилье».
			if len([]rune(key)) >= 4 && strings.HasPrefix(t, key) {
				return true
			}
		}
	}
	return false
}
