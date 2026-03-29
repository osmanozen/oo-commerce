package types

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Money is an immutable value object representing a monetary amount with currency.
// Uses shopspring/decimal for precise arithmetic — no floating-point errors.
type Money struct {
	Amount   decimal.Decimal `json:"amount" db:"amount"`
	Currency string          `json:"currency" db:"currency"`
}

// validCurrencies is the allowlist of ISO 4217 codes.
var validCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true,
	"TRY": true, "JPY": true, "CAD": true,
	"AUD": true, "CHF": true, "SEK": true,
}

// NewMoney creates a validated Money value object.
func NewMoney(amount decimal.Decimal, currency string) (Money, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return Money{}, errors.New("amount must be greater than zero")
	}
	curr := strings.ToUpper(strings.TrimSpace(currency))
	if !validCurrencies[curr] {
		return Money{}, fmt.Errorf("invalid currency code: %s", curr)
	}
	return Money{
		Amount:   amount.Round(2),
		Currency: curr,
	}, nil
}

// ZeroMoney returns a zero-value Money for the given currency.
func ZeroMoney(currency string) Money {
	return Money{
		Amount:   decimal.Zero,
		Currency: strings.ToUpper(currency),
	}
}

// Add adds two Money values. Currencies must match.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("cannot add %s to %s", other.Currency, m.Currency)
	}
	return Money{
		Amount:   m.Amount.Add(other.Amount).Round(2),
		Currency: m.Currency,
	}, nil
}

// Subtract subtracts a Money value. Currencies must match.
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("cannot subtract %s from %s", other.Currency, m.Currency)
	}
	return Money{
		Amount:   m.Amount.Sub(other.Amount).Round(2),
		Currency: m.Currency,
	}, nil
}

// Multiply multiplies the amount by the given factor.
func (m Money) Multiply(factor int) Money {
	return Money{
		Amount:   m.Amount.Mul(decimal.NewFromInt(int64(factor))).Round(2),
		Currency: m.Currency,
	}
}

// IsPositive returns true if amount > 0.
func (m Money) IsPositive() bool {
	return m.Amount.IsPositive()
}

// IsZero returns true if amount == 0.
func (m Money) IsZero() bool {
	return m.Amount.IsZero()
}

// String returns a human-readable representation.
func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.Amount.StringFixed(2), m.Currency)
}
