package scenario

import (
	"net/http"
	"testing"

	"cim-backend/pkg"
)

// notConfiguredBody is the exact JSON the API returns for the not-configured
// finalize error: an ErrorCodeInternal AppError (code "internal", no "key"),
// HTTP 500, whose only distinguishing signal is the localized message.
func notConfiguredBody(t *testing.T) []byte {
	t.Helper()
	msg := pkg.ErrorMessages[pkg.ErrKeyRevenueExpenseSettingsNotConfigured]
	if msg.EN == "" {
		t.Fatal("catalog EN message for revenue-expense-not-configured is empty")
	}
	return []byte(`{"code":"internal","message":"` + msg.EN + `"}`)
}

// isRevenueExpenseNotConfigured is the DoClassified predicate; it must recognise
// ONLY the not-configured 500 and reject every other response.
func TestIsRevenueExpenseNotConfiguredMatchesOnlyThatError(t *testing.T) {
	// The specific not-configured 500 -> tolerated.
	if !isRevenueExpenseNotConfigured(http.StatusInternalServerError, notConfiguredBody(t)) {
		t.Error("a 500 carrying the not-configured message must be classified as not-configured")
	}

	// A DIFFERENT 500 (other internal cause) must NOT be tolerated.
	if isRevenueExpenseNotConfigured(http.StatusInternalServerError, []byte(`{"code":"internal","message":"Failed to finalize revenue expense"}`)) {
		t.Error("a 500 from a different cause must NOT be classified as not-configured")
	}

	// A non-500 (e.g. 409) must NOT match even with a coincidental body.
	if isRevenueExpenseNotConfigured(http.StatusConflict, notConfiguredBody(t)) {
		t.Error("a non-500 status must NOT be classified as not-configured")
	}

	// An empty body must NOT match.
	if isRevenueExpenseNotConfigured(http.StatusInternalServerError, nil) {
		t.Error("an empty body must NOT be classified as not-configured")
	}
}

// The VI-localized message must also be tolerated (the simulate client sends no
// Accept-Language, so EN is the default, but a VI-localized deployment must not
// surprise-fail).
func TestIsRevenueExpenseNotConfiguredMatchesVietnamese(t *testing.T) {
	msg := pkg.ErrorMessages[pkg.ErrKeyRevenueExpenseSettingsNotConfigured]
	if msg.VI == "" {
		t.Skip("no VI message configured")
	}
	body := []byte(`{"code":"internal","message":"` + msg.VI + `"}`)
	if !isRevenueExpenseNotConfigured(http.StatusInternalServerError, body) {
		t.Error("the VI-localized not-configured message must be classified as not-configured")
	}
}
