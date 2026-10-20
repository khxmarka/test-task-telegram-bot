package service

import (
	"fmt"
	"math"
	"strings"

	"github.com/testTelegramApp/expenses-bot/internal/config"
	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

// ToBYNKopecks converts a positive amount in the given currency to BYN kopecks.
func ToBYNKopecks(amount float64, currency domain.Currency, rates config.CurrencyConfig) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("%w: amount must be > 0", errorsx.ErrValidation)
	}
	cur := domain.Currency(strings.ToUpper(string(currency)))
	var kopecks int64
	switch cur {
	case domain.CurrencyBYN, "":
		kopecks = int64(math.Round(amount * float64(rates.BYNToKopecks)))
	case domain.CurrencyUSD:
		kopecks = int64(math.Round(amount * float64(rates.USDToBYNKopecks)))
	case domain.CurrencyRUB:
		kopecks = int64(math.Round(amount * float64(rates.RUBToBYNKopecks)))
	default:
		return 0, fmt.Errorf("%w: unsupported currency %s", errorsx.ErrValidation, currency)
	}
	if kopecks <= 0 {
		return 0, fmt.Errorf("%w: amount too small after conversion", errorsx.ErrValidation)
	}
	return kopecks, nil
}

func FormatBYN(kopecks int64) string {
	rub := kopecks / 100
	kop := kopecks % 100
	if kop < 0 {
		kop = -kop
	}
	return fmt.Sprintf("%d.%02d BYN", rub, kop)
}

func ParseCurrency(s string) (domain.Currency, error) {
	c := domain.Currency(strings.ToUpper(strings.TrimSpace(s)))
	switch c {
	case domain.CurrencyBYN, domain.CurrencyUSD, domain.CurrencyRUB:
		return c, nil
	case "":
		return domain.CurrencyBYN, nil
	default:
		return "", fmt.Errorf("%w: unsupported currency %s", errorsx.ErrValidation, s)
	}
}

func ParseCategory(s string) (domain.Category, error) {
	c := domain.Category(strings.ToLower(strings.TrimSpace(s)))
	if !c.Valid() {
		return "", fmt.Errorf("%w: invalid category %s", errorsx.ErrValidation, s)
	}
	return c, nil
}
