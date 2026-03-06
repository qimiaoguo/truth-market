// Package decimal provides convenience wrappers and domain constants around the
// shopspring/decimal library for the truth-market platform.
//
// It re-exports the [decimal.Decimal] type so that callers only need to import
// this package. Common constructors ([NewFromString], [NewFromFloat],
// [NewFromInt]) and domain-specific constants ([MinPrice], [MaxPrice],
// [InitialBalance]) are provided for consistency across services.
package decimal

import "github.com/shopspring/decimal"

// Decimal is a re-export of [decimal.Decimal] so that callers do not need to
// import the shopspring package directly.
type Decimal = decimal.Decimal

// Zero is the decimal representation of 0.
var Zero = decimal.NewFromInt(0)

// One is the decimal representation of 1.
var One = decimal.NewFromInt(1)

// MinPrice is the minimum valid order price (0.01) in a prediction market.
// Prices represent implied probabilities and must be strictly positive.
var MinPrice Decimal

// MaxPrice is the maximum valid order price (0.99) in a prediction market.
// A price of 1.00 would imply certainty and is not allowed.
var MaxPrice Decimal

// InitialBalance is the starting balance credited to every new user account.
var InitialBalance = decimal.NewFromInt(1000)

func init() {
	var err error
	MinPrice, err = decimal.NewFromString("0.01")
	if err != nil {
		panic("decimal: failed to parse MinPrice: " + err.Error())
	}
	MaxPrice, err = decimal.NewFromString("0.99")
	if err != nil {
		panic("decimal: failed to parse MaxPrice: " + err.Error())
	}
}

// NewFromString creates a Decimal from a string representation. It returns an
// error if the string cannot be parsed as a valid decimal number.
func NewFromString(s string) (Decimal, error) {
	return decimal.NewFromString(s)
}

// NewFromFloat creates a Decimal from a float64. Note that floating-point
// representation may introduce small rounding artifacts; prefer
// [NewFromString] for exact values.
func NewFromFloat(f float64) Decimal {
	return decimal.NewFromFloat(f)
}

// NewFromInt creates a Decimal from an int64.
func NewFromInt(i int64) Decimal {
	return decimal.NewFromInt(i)
}

// ValidatePrice reports whether price falls within the valid range for a
// prediction market order: [MinPrice, MaxPrice] (inclusive). Both bounds are
// checked using the exact decimal values 0.01 and 0.99.
func ValidatePrice(price Decimal) bool {
	return price.GreaterThanOrEqual(MinPrice) && price.LessThanOrEqual(MaxPrice)
}
