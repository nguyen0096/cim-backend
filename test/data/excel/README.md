# Test fixtures

`revenue_expense_sample.xlsx` is intentionally absent from this public snapshot.

The original fixture was a real operational spreadsheet and was removed, along with
its history, before publication. The revenue/expense tests still reference the path,
so they will not run here.

To regenerate a working fixture, create a single sheet named `TIỀN MẶT` with:

- a header row containing `STT`, `DIỄN GIẢI`, `NƯỚC`, and `ĂN NHẸ,CƠM`
- at least one row whose first column holds a date in one of `CommonDateFormats`
  (see `internal/repository/excel/helper.go`)
- cell fills for `17B319`, `27B4F5`, and `F5E727` (see `pkg/constants.go`)
