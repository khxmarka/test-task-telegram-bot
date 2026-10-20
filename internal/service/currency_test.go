package service

import (
	"errors"
	"testing"

	"github.com/testTelegramApp/expenses-bot/internal/config"
	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/errorsx"
)

func TestToBYNKopecks(t *testing.T) {
	rates := config.CurrencyConfig{
		BYNToKopecks:    100,
		USDToBYNKopecks: 300,
		RUBToBYNKopecks: 4,
	}

	tests := []struct {
		name     string
		amount   float64
		currency domain.Currency
		want     int64
		wantErr  error
	}{
		{"byn integer", 10, domain.CurrencyBYN, 1000, nil},
		{"byn fractional", 2.5, domain.CurrencyBYN, 250, nil},
		{"usd", 1, domain.CurrencyUSD, 300, nil}, // 1 USD = 3 BYN
		{"rub", 10, domain.CurrencyRUB, 40, nil}, // 10 RUB ≈ 0.40 BYN
		{"zero", 0, domain.CurrencyBYN, 0, errorsx.ErrValidation},
		{"negative", -1, domain.CurrencyBYN, 0, errorsx.ErrValidation},
		{"bad currency", 1, domain.Currency("EUR"), 0, errorsx.ErrValidation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToBYNKopecks(tt.amount, tt.currency, rates)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseCategory(t *testing.T) {
	tests := []struct {
		in      string
		want    domain.Category
		wantErr bool
	}{
		{"cafes", domain.CategoryCafes, false},
		{"OTHER", domain.CategoryOther, false},
		{"food", "", true},
	}
	for _, tt := range tests {
		got, err := ParseCategory(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", tt.in)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Fatalf("%s: got %v %v", tt.in, got, err)
		}
	}
}

func TestFormatBYN(t *testing.T) {
	if got := FormatBYN(12345); got != "123.45 BYN" {
		t.Fatalf("got %s", got)
	}
}
