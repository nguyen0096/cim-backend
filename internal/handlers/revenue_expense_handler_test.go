package handlers

import (
	"cim-backend/internal/config"
	"cim-backend/internal/mocks/repositorymocks"
	"cim-backend/internal/mocks/servicemocks"
	"cim-backend/internal/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRevenueExpenseHandler_FinalizeRevenueExpense(t *testing.T) {
	e := echo.New()

	t.Run("Should finalize successfully with current date when payload is empty", func(t *testing.T) {
		mockExcelService := new(servicemocks.ExcelService)
		mockSettingsService := new(servicemocks.SettingsService)
		mockRepo := new(repositorymocks.RevenueExpenseFinalizationRepository)

		handler := NewRevenueExpenseHandler(mockExcelService, mockSettingsService, mockRepo)

		reqBody := ""
		req := httptest.NewRequest(http.MethodPost, "/revenue-expenses/finalize", strings.NewReader(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Mock Expectations
		today := time.Now()
		mockSettingsService.On("GetSettingValue", mock.Anything, config.LastFinalizedDateSettingsKey, mock.Anything).Return(nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.RevenueExpenseFinalization")).Return(nil)
		mockExcelService.On("FinalizeRevenueExpense", mock.Anything,
			mock.MatchedBy(func(t time.Time) bool { return t.Format("2006-01-02") == today.Format("2006-01-02") }),
			mock.MatchedBy(func(t time.Time) bool { return t.Format("2006-01-02") == today.Format("2006-01-02") }),
		).Return(nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.RevenueExpenseFinalization")).Return(nil)
		mockSettingsService.On("SetSetting", mock.Anything, config.LastFinalizedDateSettingsKey, mock.Anything).Return(nil)

		assert.NoError(t, handler.FinalizeRevenueExpense(c))
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp FinalizeRevenueExpenseResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "Revenue expense finalized successfully", resp.Message)

		mockRepo.AssertNumberOfCalls(t, "Create", 1)
		mockRepo.AssertNumberOfCalls(t, "Update", 1)
		mockExcelService.AssertNumberOfCalls(t, "FinalizeRevenueExpense", 1)
		mockSettingsService.AssertNumberOfCalls(t, "GetSettingValue", 1)
		mockSettingsService.AssertNumberOfCalls(t, "SetSetting", 1)
	})

	t.Run("Should finalize successfully when payload is valid", func(t *testing.T) {
		mockExcelService := new(servicemocks.ExcelService)
		mockSettingsService := new(servicemocks.SettingsService)
		mockRepo := new(repositorymocks.RevenueExpenseFinalizationRepository)

		handler := NewRevenueExpenseHandler(mockExcelService, mockSettingsService, mockRepo)

		reqBody := `{"prefix_date": "2024-03-05", "date_in_excel": "2024-03-05"}`
		req := httptest.NewRequest(http.MethodPost, "/revenue-expenses/finalize", strings.NewReader(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Mock Expectations
		expectedPrefixDate, _ := time.Parse("2006-01-02", "2024-03-05")
		expectedDateInExcel, _ := time.Parse("2006-01-02", "2024-03-05")

		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(f *models.RevenueExpenseFinalization) bool {
			return f.FinalizedDate.Equal(expectedPrefixDate) && f.Status == nil
		})).Return(nil)

		mockExcelService.On("FinalizeRevenueExpense", mock.Anything,
			mock.MatchedBy(func(t time.Time) bool { return t.Equal(expectedPrefixDate) }),
			mock.MatchedBy(func(t time.Time) bool { return t.Equal(expectedDateInExcel) }),
		).Return(nil)

		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(f *models.RevenueExpenseFinalization) bool {
			return f.FinalizedDate.Equal(expectedPrefixDate) && *f.Status == models.RevenueExpenseFinalizationStatusSuccess
		})).Return(nil)

		mockSettingsService.On("SetSetting", mock.Anything, config.LastFinalizedDateSettingsKey, mock.Anything).Return(nil)

		if assert.NoError(t, handler.FinalizeRevenueExpense(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			var resp FinalizeRevenueExpenseResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.Equal(t, "Revenue expense finalized successfully", resp.Message)
			assert.Equal(t, "2024-03-05", resp.Date)
		}

		mockRepo.AssertNumberOfCalls(t, "Create", 1)
		mockRepo.AssertNumberOfCalls(t, "Update", 1)
		mockExcelService.AssertNumberOfCalls(t, "FinalizeRevenueExpense", 1)
		mockSettingsService.AssertNumberOfCalls(t, "SetSetting", 1)
	})

	t.Run("Should ignore payload and finalize with current date when body is invalid", func(t *testing.T) {
		mockExcelService := new(servicemocks.ExcelService)
		mockSettingsService := new(servicemocks.SettingsService)
		mockRepo := new(repositorymocks.RevenueExpenseFinalizationRepository)

		handler := NewRevenueExpenseHandler(mockExcelService, mockSettingsService, mockRepo)

		reqBody := `invalid json`
		req := httptest.NewRequest(http.MethodPost, "/revenue-expenses/finalize", strings.NewReader(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.Error(t, handler.FinalizeRevenueExpense(c))

		mockRepo.AssertNumberOfCalls(t, "Create", 0)
		mockRepo.AssertNumberOfCalls(t, "Update", 0)
		mockExcelService.AssertNumberOfCalls(t, "FinalizeRevenueExpense", 0)
		mockSettingsService.AssertNumberOfCalls(t, "GetSettingValue", 0)
		mockSettingsService.AssertNumberOfCalls(t, "SetSetting", 0)
	})

	t.Run("Should return error and not update settings when repository fails", func(t *testing.T) {
		mockExcelService := new(servicemocks.ExcelService)
		mockSettingsService := new(servicemocks.SettingsService)
		mockRepo := new(repositorymocks.RevenueExpenseFinalizationRepository)

		handler := NewRevenueExpenseHandler(mockExcelService, mockSettingsService, mockRepo)

		reqBody := `{"prefix_date": "2024-03-05", "date_in_excel": "2024-03-05"}`
		req := httptest.NewRequest(http.MethodPost, "/revenue-expenses/finalize", strings.NewReader(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockSettingsService.On("GetSettingValue", mock.Anything, config.LastFinalizedDateSettingsKey, mock.Anything).Return(nil)
		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.RevenueExpenseFinalization")).Return(assert.AnError)
		mockExcelService.On("FinalizeRevenueExpense", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(nil)
		mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.RevenueExpenseFinalization")).Return(nil)
		mockSettingsService.On("SetSetting", mock.Anything, config.LastFinalizedDateSettingsKey, mock.Anything).Return(nil)

		assert.Error(t, handler.FinalizeRevenueExpense(c))

		mockRepo.AssertNumberOfCalls(t, "Create", 1)
		mockRepo.AssertNumberOfCalls(t, "Update", 0)
		mockExcelService.AssertNumberOfCalls(t, "FinalizeRevenueExpense", 0)
		mockSettingsService.AssertNumberOfCalls(t, "SetSetting", 0)
	})

	t.Run("Should create finalization record with failed status, update settings and return error response when finalize is failed", func(t *testing.T) {
		mockExcelService := new(servicemocks.ExcelService)
		mockSettingsService := new(servicemocks.SettingsService)
		mockRepo := new(repositorymocks.RevenueExpenseFinalizationRepository)

		handler := NewRevenueExpenseHandler(mockExcelService, mockSettingsService, mockRepo)

		expectedPrefixDate, _ := time.Parse("2006-01-02", "2024-03-05")
		reqBody := `{"prefix_date": "2024-03-05", "date_in_excel": "2024-03-05"}`
		req := httptest.NewRequest(http.MethodPost, "/revenue-expenses/finalize", strings.NewReader(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockSettingsService.On("GetSettingValue", mock.Anything, config.LastFinalizedDateSettingsKey, mock.Anything).Return(nil)
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(f *models.RevenueExpenseFinalization) bool {
			return f.FinalizedDate.Equal(expectedPrefixDate)
		})).Return(nil)
		mockExcelService.On("FinalizeRevenueExpense", mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(assert.AnError)
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(f *models.RevenueExpenseFinalization) bool {
			return *f.Status == models.RevenueExpenseFinalizationStatusFailed
		})).Return(nil)
		mockSettingsService.On("SetSetting", mock.Anything, config.LastFinalizedDateSettingsKey, mock.Anything).Return(nil)

		assert.NoError(t, handler.FinalizeRevenueExpense(c))
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var resp FinalizeRevenueExpenseResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "Failed to finalize revenue expense", resp.Message)

		mockSettingsService.AssertNumberOfCalls(t, "GetSettingValue", 0)
		mockRepo.AssertNumberOfCalls(t, "Create", 1)
		mockExcelService.AssertNumberOfCalls(t, "FinalizeRevenueExpense", 1)
		mockRepo.AssertNumberOfCalls(t, "Update", 1)
		mockSettingsService.AssertNumberOfCalls(t, "SetSetting", 1)
	})
}

func TestRevenueExpenseHandler_ListFinalizedDates(t *testing.T) {
	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		mockExcelService := new(servicemocks.ExcelService)
		mockSettingsService := new(servicemocks.SettingsService)
		mockRepo := new(repositorymocks.RevenueExpenseFinalizationRepository)

		handler := NewRevenueExpenseHandler(mockExcelService, mockSettingsService, mockRepo)

		req := httptest.NewRequest(http.MethodGet, "/revenue-expenses/finalized-dates?page=1&limit=10", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		finalizations := []models.RevenueExpenseFinalization{
			{FinalizedDate: time.Now()},
		}
		mockRepo.On("List", mock.Anything, 10, 0).Return(finalizations, int64(1), nil)

		if assert.NoError(t, handler.ListFinalizedDates(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			var resp map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.Equal(t, float64(1), resp["total"])
			assert.Len(t, resp["data"], 1)
		}
	})

	t.Run("Repository Error", func(t *testing.T) {
		mockExcelService := new(servicemocks.ExcelService)
		mockSettingsService := new(servicemocks.SettingsService)
		mockRepo := new(repositorymocks.RevenueExpenseFinalizationRepository)

		handler := NewRevenueExpenseHandler(mockExcelService, mockSettingsService, mockRepo)

		req := httptest.NewRequest(http.MethodGet, "/revenue-expenses/finalized-dates", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockRepo.On("List", mock.Anything, 20, 0).Return(nil, int64(0), assert.AnError)

		err := handler.ListFinalizedDates(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
