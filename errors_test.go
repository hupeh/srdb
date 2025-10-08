package srdb

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrNotFound", ErrNotFound, true},
		{"ErrTableNotFound", ErrTableNotFound, true},
		{"ErrDatabaseNotFound", ErrDatabaseNotFound, true},
		{"ErrIndexNotFound", ErrIndexNotFound, true},
		{"ErrFieldNotFound", ErrFieldNotFound, true},
		{"ErrSchemaNotFound", ErrSchemaNotFound, true},
		{"ErrSSTableNotFound", ErrSSTableNotFound, true},
		{"ErrTableExists", ErrTableExists, false},
		{"ErrCorrupted", ErrCorrupted, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsCorrupted(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrCorrupted", ErrCorrupted, true},
		{"ErrWALCorrupted", ErrWALCorrupted, true},
		{"ErrSSTableCorrupted", ErrSSTableCorrupted, true},
		{"ErrIndexCorrupted", ErrIndexCorrupted, true},
		{"ErrChecksumMismatch", ErrChecksumMismatch, true},
		{"ErrSchemaChecksumMismatch", ErrSchemaChecksumMismatch, true},
		{"ErrNotFound", ErrNotFound, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCorrupted(tt.err)
			if result != tt.expected {
				t.Errorf("IsCorrupted(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsClosed(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrClosed", ErrClosed, true},
		{"ErrDatabaseClosed", ErrDatabaseClosed, true},
		{"ErrTableClosed", ErrTableClosed, true},
		{"ErrWALClosed", ErrWALClosed, true},
		{"ErrNotFound", ErrNotFound, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsClosed(tt.err)
			if result != tt.expected {
				t.Errorf("IsClosed(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	baseErr := ErrNotFound

	wrapped := WrapError(baseErr, "failed to get table %s", "users")
	if wrapped == nil {
		t.Fatal("WrapError returned nil")
	}

	// 验证包装后的错误仍然是原始错误
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("Wrapped error should be ErrNotFound")
	}

	// 验证错误消息包含上下文
	errMsg := wrapped.Error()
	if errMsg != "failed to get table users: [1000] not found" {
		t.Errorf("Expected error message to contain context, got: %s", errMsg)
	}
}

func TestWrapErrorNil(t *testing.T) {
	wrapped := WrapError(nil, "some context")
	if wrapped != nil {
		t.Error("WrapError(nil) should return nil")
	}
}

func TestNewError(t *testing.T) {
	err := NewErrorf(ErrCodeInvalidData, "custom error: %s", "test")
	if err == nil {
		t.Fatal("NewErrorf returned nil")
	}

	// 验证错误码
	if err.Code != ErrCodeInvalidData {
		t.Errorf("Expected code %d, got %d", ErrCodeInvalidData, err.Code)
	}

	// 验证错误消息
	expected := "custom error: test"
	if err.Message != expected {
		t.Errorf("Expected message %q, got %q", expected, err.Message)
	}
}

func TestGetErrorCode(t *testing.T) {
	err := NewError(ErrCodeTableNotFound, nil)
	code := GetErrorCode(err)
	if code != ErrCodeTableNotFound {
		t.Errorf("Expected code %d, got %d", ErrCodeTableNotFound, code)
	}

	// 测试非 Error 类型
	stdErr := fmt.Errorf("standard error")
	code = GetErrorCode(stdErr)
	if code != 0 {
		t.Errorf("Expected code 0 for standard error, got %d", code)
	}
}

func TestIsError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     ErrCode
		expected bool
	}{
		{"exact match", NewError(ErrCodeTableNotFound, nil), ErrCodeTableNotFound, true},
		{"no match", NewError(ErrCodeTableNotFound, nil), ErrCodeDatabaseNotFound, false},
		{"wrapped error", WrapError(ErrTableNotFound, "context"), ErrCodeTableNotFound, true},
		{"standard error", fmt.Errorf("standard error"), ErrCodeTableNotFound, false},
		{"nil error", nil, ErrCodeTableNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsError(tt.err, tt.code)
			if result != tt.expected {
				t.Errorf("IsError(%v, %d) = %v, want %v", tt.err, tt.code, result, tt.expected)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	// 测试多层包装
	err1 := ErrTableNotFound
	err2 := WrapError(err1, "database %s", "mydb")
	err3 := WrapError(err2, "operation failed")

	// 验证仍然能识别原始错误
	if !IsNotFound(err3) {
		t.Error("Should recognize wrapped error as NotFound")
	}

	if !errors.Is(err3, ErrTableNotFound) {
		t.Error("Should be able to unwrap to ErrTableNotFound")
	}
}
