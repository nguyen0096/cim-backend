package spreadsheet

import (
	"fmt"
	"time"
)

const (
	// TestInventoryTrackerExcelFile is the path to the inventory tracker test Excel file
	TestInventoryTrackerExcelFile = "./test/XNT_app_test.xlsx"
)

var (
	// ConfigTestInventoryTrackerExcelFile is the configuration for the inventory tracker test Excel file.
	ConfigTestInventoryTrackerExcelFile = SheetConfig{
		InternalID:     "current_month_inventory_change",
		NamePattern:    "THANG {MM}",
		HeaderStartRow: 5,
		HeaderStartCol: 1,
		HeaderHeight:   3,
	}
)

func getTestFileConfig() FileConfig {
	xntSheet := SheetConfig{
		InternalID:     "xnt_sheet",
		NamePattern:    "THANG {MM}",
		HeaderStartRow: 5,
		HeaderStartCol: 1,
		HeaderHeight:   3,
		FooterHeight:   3,
		IndexColumnNames: []HeaderBranch{
			{fmt.Sprintf("%s%s", MetadataHeaderPrefix, "product_id")},
		},
	}
	xntSheet.SetSheetNameTimeParams(time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC))

	fc := FileConfig{
		FilePath:     TestInventoryTrackerExcelFile,
		SheetConfigs: []SheetConfig{xntSheet},
	}
	return fc
}
