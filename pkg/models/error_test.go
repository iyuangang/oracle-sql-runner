package models

import (
	"errors"
	"strings"
	"testing"
)

func TestNewErrorResult(t *testing.T) {
	err := errors.New("test error")
	result := NewErrorResult(err)

	if result.Success != 0 {
		t.Errorf("Expected Success to be 0, got %d", result.Success)
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	if result.Errors[0] != err {
		t.Errorf("Expected error to be %v, got %v", err, result.Errors[0])
	}
}

func TestSQLError_Error(t *testing.T) {
	tests := []struct {
		name     string
		sqlError *SQLError
		want     []string
	}{
		{
			name: "基本错误信息",
			sqlError: &SQLError{
				SQL:     "SELECT * FROM test",
				Message: "table not found",
				Line:    10,
				File:    "test.sql",
			},
			want: []string{
				"SQL错误",
				"test.sql:10",
				"table not found",
				"SELECT * FROM test",
			},
		},
		{
			name: "空SQL语句",
			sqlError: &SQLError{
				SQL:     "",
				Message: "empty SQL",
				Line:    1,
				File:    "empty.sql",
			},
			want: []string{
				"SQL错误",
				"empty.sql:1",
				"empty SQL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sqlError.Error()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("SQLError.Error() = %v, want to contain %v", got, want)
				}
			}
		})
	}
}

func TestNewSQLError(t *testing.T) {
	sql := "SELECT * FROM test"
	message := "table not found"
	line := 10
	file := "test.sql"

	err := NewSQLError(sql, message, line, file)

	if err.SQL != sql {
		t.Errorf("Expected SQL to be %s, got %s", sql, err.SQL)
	}
	if err.Message != message {
		t.Errorf("Expected Message to be %s, got %s", message, err.Message)
	}
	if err.Line != line {
		t.Errorf("Expected Line to be %d, got %d", line, err.Line)
	}
	if err.File != file {
		t.Errorf("Expected File to be %s, got %s", file, err.File)
	}
}
