package testutil

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// AssertDecimalEqual checks that two decimal.Decimal values are equal.
// It delegates to assert so that test output includes the caller's file
// and line number.
func AssertDecimalEqual(t *testing.T, expected, actual decimal.Decimal, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Truef(t, expected.Equal(actual),
		"expected decimal %s but got %s", expected.String(), actual.String())
	if len(msgAndArgs) > 0 {
		// Re-run with the caller-supplied message on failure so it appears
		// in the output.
		if !expected.Equal(actual) {
			assert.Fail(t, "decimal mismatch", msgAndArgs...)
		}
	}
}

// AssertBalanceEqual checks that user.Balance matches the expected value
// (given as a float64 for ergonomic test authoring).
func AssertBalanceEqual(t *testing.T, expected float64, user *domain.User) {
	t.Helper()
	exp := decimal.NewFromFloat(expected)
	assert.Truef(t, exp.Equal(user.Balance),
		"expected balance %s but user %s has %s",
		exp.String(), user.ID, user.Balance.String())
}

// AssertErrorCode checks that err is an *apperrors.AppError whose Code field
// matches expectedCode. If err is nil or is not an AppError the assertion
// fails.
func AssertErrorCode(t *testing.T, expectedCode string, err error) {
	t.Helper()
	if !assert.Error(t, err, "expected an error with code %q but got nil", expectedCode) {
		return
	}
	var appErr *apperrors.AppError
	if !assert.True(t, errors.As(err, &appErr),
		"expected error to be *apperrors.AppError, got %T", err) {
		return
	}
	assert.Equal(t, expectedCode, appErr.Code,
		"expected error code %q but got %q", expectedCode, appErr.Code)
}
