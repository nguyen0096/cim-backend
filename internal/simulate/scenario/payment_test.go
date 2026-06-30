package scenario

import (
	"net/http"
	"testing"

	"cim-backend/pkg"
)

// settingsBody is the exact JSON the API returns for a revenue-expense settings
// domain error: an ErrorCodeInternal AppError (code "internal", no "key"), HTTP
// 500, whose only distinguishing signal is the localized message.
func settingsBody(t *testing.T, key string) []byte {
	t.Helper()
	msg := pkg.ErrorMessages[key]
	if msg.EN == "" {
		t.Fatalf("catalog EN message for %q is empty", key)
	}
	return []byte(`{"code":"internal","message":"` + msg.EN + `"}`)
}

// isRevenueExpenseSettingsUnavailable must recognise EVERY revenue-expense
// settings domain error (not just not-configured) and reject everything else.
func TestIsRevenueExpenseSettingsUnavailableMatchesAllSettingsErrors(t *testing.T) {
	// Each settings error (incl. the parse error that broke a live run) -> tolerated.
	for _, key := range revenueExpenseSettingsErrorKeys {
		if !isRevenueExpenseSettingsUnavailable(http.StatusInternalServerError, settingsBody(t, key)) {
			t.Errorf("a 500 carrying the %q message must be tolerated", key)
		}
	}

	// A DIFFERENT 500 (other internal cause) must NOT be tolerated.
	if isRevenueExpenseSettingsUnavailable(http.StatusInternalServerError, []byte(`{"code":"internal","message":"Failed to finalize revenue expense"}`)) {
		t.Error("a 500 from a different cause must NOT be tolerated")
	}

	// A non-500 (e.g. 409) must NOT match even with a coincidental body.
	if isRevenueExpenseSettingsUnavailable(http.StatusConflict, settingsBody(t, pkg.ErrKeyRevenueExpenseSettingsNotConfigured)) {
		t.Error("a non-500 status must NOT be tolerated")
	}

	// An empty body must NOT match.
	if isRevenueExpenseSettingsUnavailable(http.StatusInternalServerError, nil) {
		t.Error("an empty body must NOT be tolerated")
	}
}

// The VI-localized message must also be tolerated (the simulate client sends no
// Accept-Language, so EN is the default, but a VI-localized deployment must not
// surprise-fail).
func TestIsRevenueExpenseSettingsUnavailableMatchesVietnamese(t *testing.T) {
	msg := pkg.ErrorMessages[pkg.ErrKeyFailedToParseRevenueExpenseSettings]
	if msg.VI == "" {
		t.Skip("no VI message configured")
	}
	body := []byte(`{"code":"internal","message":"` + msg.VI + `"}`)
	if !isRevenueExpenseSettingsUnavailable(http.StatusInternalServerError, body) {
		t.Error("the VI-localized parse-failure message must be tolerated")
	}
}
