package spreadsheet

import (
	"testing"


	"github.com/stretchr/testify/assert"
)

func Test_trimFileConfigValues(t *testing.T) {
	tests := []struct {
		name     string
		input    FileConfig
		expected FileConfig
	}{
		{
			name: "should trim whitespace from all string fields",
			input: FileConfig{
				FilePath: "  /path/to/file.xlsx  ",
				SheetConfigs: []SheetConfig{
					{
						InternalID:  "  sheet1  ",
						NamePattern: "  THANG {MM}  ",
						NameParams: map[string]string{
							"  key1  ": "  value1  ",
							"key2":     "value2",
						},
						HeaderStartRow:   1,
						HeaderStartCol:   1,
						HeaderHeight:     2,
						IndexColumnNames: []TreeHeader{{"  col1  "}, {"col2"}, {"  col3  "}},
					},
				},
			},
			expected: FileConfig{
				FilePath: "/path/to/file.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:  "sheet1",
						NamePattern: "THANG {MM}",
						NameParams: map[string]string{
							"  key1  ": "value1",
							"key2":     "value2",
						},
						HeaderStartRow:   1,
						HeaderStartCol:   1,
						HeaderHeight:     2,
						IndexColumnNames: []TreeHeader{{"col1"}, {"col2"}, {"col3"}},
					},
				},
			},
		},
		{
			name: "should handle empty strings",
			input: FileConfig{
				FilePath: "",
				SheetConfigs: []SheetConfig{
					{
						InternalID:       "",
						NamePattern:      "",
						NameParams:       map[string]string{},
						HeaderStartRow:   1,
						HeaderStartCol:   1,
						HeaderHeight:     1,
						IndexColumnNames: []TreeHeader{},
					},
				},
			},
			expected: FileConfig{
				FilePath: "",
				SheetConfigs: []SheetConfig{
					{
						InternalID:       "",
						NamePattern:      "",
						NameParams:       map[string]string{},
						HeaderStartRow:   1,
						HeaderStartCol:   1,
						HeaderHeight:     1,
						IndexColumnNames: []TreeHeader{},
					},
				},
			},
		},
		{
			name: "should handle nil maps",
			input: FileConfig{
				FilePath: "file.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:       "sheet1",
						NamePattern:      "SHEET_{MM}",
						NameParams:       nil,
						HeaderStartRow:   2,
						HeaderStartCol:   3,
						HeaderHeight:     4,
						IndexColumnNames: nil,
					},
				},
			},
			expected: FileConfig{
				FilePath: "file.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:       "sheet1",
						NamePattern:      "SHEET_{MM}",
						NameParams:       nil,
						HeaderStartRow:   2,
						HeaderStartCol:   3,
						HeaderHeight:     4,
						IndexColumnNames: nil,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimFileConfigValues(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_validateFileConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      FileConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "should pass validation with valid config",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:  "sheet1",
						NamePattern: "SHEET_{MM}",
						NameParams: map[string]string{
							"{MM}": "01",
						},
						HeaderStartRow:   1,
						HeaderStartCol:   1,
						HeaderHeight:     2,
						IndexColumnNames: []TreeHeader{{"col1"}, {"col2"}},
					},
				},
			},
			expectError: false,
		},
		{
			name: "should fail validation when FilePath is empty",
			config: FileConfig{
				FilePath: "",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: 1,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
				},
			},
			expectError: true,
			errorMsg:    "failed to validate file config",
		},
		{
			name: "should fail validation when SheetConfigs is empty",
			config: FileConfig{
				FilePath:     "test.xlsx",
				SheetConfigs: []SheetConfig{},
			},
			expectError: true,
			errorMsg:    "failed to validate file config",
		},
		{
			name: "should fail validation when InternalID is empty",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: 1,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
				},
			},
			expectError: true,
			errorMsg:    "failed to validate file config",
		},
		{
			name: "should fail validation when NamePattern is empty",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "",
						HeaderStartRow: 1,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
				},
			},
			expectError: true,
			errorMsg:    "failed to validate file config",
		},
		{
			name: "should fail validation when HeaderStartRow is zero",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: 0,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
				},
			},
			expectError: true,
			errorMsg:    "failed to validate file config",
		},
		{
			name: "should fail validation when HeaderStartCol is zero",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: 1,
						HeaderStartCol: 0,
						HeaderHeight:   2,
					},
				},
			},
			expectError: true,
			errorMsg:    "failed to validate file config",
		},
		{
			name: "should fail validation when HeaderHeight is zero",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: 1,
						HeaderStartCol: 1,
						HeaderHeight:   0,
					},
				},
			},
			expectError: true,
			errorMsg:    "failed to validate file config",
		},
		{
			name: "should fail validation when HeaderStartRow is negative",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: -1,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
				},
			},
			expectError: true,
			errorMsg:    "failed to validate file config",
		},
		{
			name: "should pass validation with multiple sheets",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: 1,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
					{
						InternalID:     "sheet2",
						NamePattern:    "DATA_{YYYY}",
						HeaderStartRow: 2,
						HeaderStartCol: 3,
						HeaderHeight:   1,
					},
				},
			},
			expectError: false,
		},
		{
			name: "should pass validation with empty IndexColumnNames",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:       "sheet1",
						NamePattern:      "SHEET_{MM}",
						HeaderStartRow:   1,
						HeaderStartCol:   1,
						HeaderHeight:     2,
						IndexColumnNames: []TreeHeader{},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewFile(t *testing.T) {
	tests := []struct {
		name        string
		config      FileConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "should create file successfully with valid config",
			config: FileConfig{
				FilePath: "test.xlsx",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: 1,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
				},
			},
			expectError: false,
		},
		{
			name: "should trim values and create file successfully",
			config: FileConfig{
				FilePath: "  test.xlsx  ",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "  sheet1  ",
						NamePattern:    "  SHEET_{MM}  ",
						HeaderStartRow: 1,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
				},
			},
			expectError: false,
		},
		{
			name: "should fail with invalid config",
			config: FileConfig{
				FilePath: "",
				SheetConfigs: []SheetConfig{
					{
						InternalID:     "sheet1",
						NamePattern:    "SHEET_{MM}",
						HeaderStartRow: 1,
						HeaderStartCol: 1,
						HeaderHeight:   2,
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid file config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &ExcelFileProvider{}
			file, err := NewFile(tt.config, prov)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, file)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, file)
				// For the trimming test, we expect the trimmed values
				if tt.name == "should trim values and create file successfully" {
					assert.Equal(t, "test.xlsx", file.FilePath)
					assert.Equal(t, SheetInternalID("sheet1"), file.SheetConfigs[0].InternalID)
					assert.Equal(t, "SHEET_{MM}", string(file.SheetConfigs[0].NamePattern))
				} else {
					assert.Equal(t, tt.config.FilePath, file.FilePath)
					assert.Equal(t, tt.config.SheetConfigs, file.SheetConfigs)
				}
			}
		})
	}
}
