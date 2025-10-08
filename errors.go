package srdb

import (
	"errors"
	"fmt"
)

// ErrCode 错误码类型
type ErrCode int

// 错误码定义
const (
	// 通用错误 (1000-1999)
	ErrCodeNotFound     ErrCode = 1000 // 数据未找到
	ErrCodeClosed       ErrCode = 1001 // 对象已关闭
	ErrCodeInvalidData  ErrCode = 1002 // 无效数据
	ErrCodeCorrupted    ErrCode = 1003 // 数据损坏
	ErrCodeExists       ErrCode = 1004 // 对象已存在
	ErrCodeInvalidParam ErrCode = 1005 // 无效参数

	// 数据库错误 (2000-2999)
	ErrCodeDatabaseNotFound ErrCode = 2000 // 数据库不存在
	ErrCodeDatabaseExists   ErrCode = 2001 // 数据库已存在
	ErrCodeDatabaseClosed   ErrCode = 2002 // 数据库已关闭

	// 表错误 (3000-3999)
	ErrCodeTableNotFound ErrCode = 3000 // 表不存在
	ErrCodeTableExists   ErrCode = 3001 // 表已存在
	ErrCodeTableClosed   ErrCode = 3002 // 表已关闭

	// Schema 错误 (4000-4999)
	ErrCodeSchemaNotFound         ErrCode = 4000 // Schema 不存在
	ErrCodeSchemaInvalid          ErrCode = 4001 // Schema 无效
	ErrCodeSchemaMismatch         ErrCode = 4002 // Schema 不匹配
	ErrCodeSchemaValidationFailed ErrCode = 4003 // Schema 验证失败
	ErrCodeSchemaChecksumMismatch ErrCode = 4004 // Schema 校验和不匹配

	// 字段错误 (5000-5999)
	ErrCodeFieldNotFound     ErrCode = 5000 // 字段不存在
	ErrCodeFieldTypeMismatch ErrCode = 5001 // 字段类型不匹配
	ErrCodeFieldRequired     ErrCode = 5002 // 必填字段缺失

	// 索引错误 (6000-6999)
	ErrCodeIndexNotFound  ErrCode = 6000 // 索引不存在
	ErrCodeIndexExists    ErrCode = 6001 // 索引已存在
	ErrCodeIndexNotReady  ErrCode = 6002 // 索引未就绪
	ErrCodeIndexCorrupted ErrCode = 6003 // 索引损坏

	// 文件格式错误 (7000-7999)
	ErrCodeInvalidFormat      ErrCode = 7000 // 无效的文件格式
	ErrCodeUnsupportedVersion ErrCode = 7001 // 不支持的版本
	ErrCodeInvalidMagicNumber ErrCode = 7002 // 无效的魔数
	ErrCodeChecksumMismatch   ErrCode = 7003 // 校验和不匹配

	// WAL 错误 (8000-8999)
	ErrCodeWALCorrupted ErrCode = 8000 // WAL 文件损坏
	ErrCodeWALClosed    ErrCode = 8001 // WAL 已关闭

	// SSTable 错误 (9000-9999)
	ErrCodeSSTableNotFound  ErrCode = 9000 // SSTable 文件不存在
	ErrCodeSSTableCorrupted ErrCode = 9001 // SSTable 文件损坏

	// Compaction 错误 (10000-10999)
	ErrCodeCompactionInProgress ErrCode = 10000 // Compaction 正在进行
	ErrCodeNoCompactionNeeded   ErrCode = 10001 // 不需要 Compaction

	// 编解码错误 (11000-11999)
	ErrCodeEncodeFailed ErrCode = 11000 // 编码失败
	ErrCodeDecodeFailed ErrCode = 11001 // 解码失败
)

// 错误码消息映射
var errCodeMessages = map[ErrCode]string{
	// 通用错误
	ErrCodeNotFound:     "not found",
	ErrCodeClosed:       "already closed",
	ErrCodeInvalidData:  "invalid data",
	ErrCodeCorrupted:    "data corrupted",
	ErrCodeExists:       "already exists",
	ErrCodeInvalidParam: "invalid parameter",

	// 数据库错误
	ErrCodeDatabaseNotFound: "database not found",
	ErrCodeDatabaseExists:   "database already exists",
	ErrCodeDatabaseClosed:   "database closed",

	// 表错误
	ErrCodeTableNotFound: "table not found",
	ErrCodeTableExists:   "table already exists",
	ErrCodeTableClosed:   "table closed",

	// Schema 错误
	ErrCodeSchemaNotFound:         "schema not found",
	ErrCodeSchemaInvalid:          "schema invalid",
	ErrCodeSchemaMismatch:         "schema mismatch",
	ErrCodeSchemaValidationFailed: "schema validation failed",
	ErrCodeSchemaChecksumMismatch: "schema checksum mismatch",

	// 字段错误
	ErrCodeFieldNotFound:     "field not found",
	ErrCodeFieldTypeMismatch: "field type mismatch",
	ErrCodeFieldRequired:     "required field missing",

	// 索引错误
	ErrCodeIndexNotFound:  "index not found",
	ErrCodeIndexExists:    "index already exists",
	ErrCodeIndexNotReady:  "index not ready",
	ErrCodeIndexCorrupted: "index corrupted",

	// 文件格式错误
	ErrCodeInvalidFormat:      "invalid file format",
	ErrCodeUnsupportedVersion: "unsupported version",
	ErrCodeInvalidMagicNumber: "invalid magic number",
	ErrCodeChecksumMismatch:   "checksum mismatch",

	// WAL 错误
	ErrCodeWALCorrupted: "wal corrupted",
	ErrCodeWALClosed:    "wal closed",

	// SSTable 错误
	ErrCodeSSTableNotFound:  "sstable not found",
	ErrCodeSSTableCorrupted: "sstable corrupted",

	// Compaction 错误
	ErrCodeCompactionInProgress: "compaction in progress",
	ErrCodeNoCompactionNeeded:   "no compaction needed",

	// 编解码错误
	ErrCodeEncodeFailed: "encode failed",
	ErrCodeDecodeFailed: "decode failed",
}

// Error 错误类型
type Error struct {
	Code    ErrCode // 错误码
	Message string  // 错误消息
	Cause   error   // 原始错误
}

// Error 实现 error 接口
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 支持 errors.Is 和 errors.As
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is 判断错误码是否相同
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// NewError 创建新错误
func NewError(code ErrCode, cause error) *Error {
	msg, ok := errCodeMessages[code]
	if !ok {
		msg = "unknown error"
	}
	return &Error{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}

// NewErrorf 创建带格式化消息的错误
// 注意：如果 args 中最后一个参数是 error 类型，它会被设置为 Cause
func NewErrorf(code ErrCode, format string, args ...any) *Error {
	var cause error

	// 检查最后一个参数是否为 error
	if len(args) > 0 {
		if err, ok := args[len(args)-1].(error); ok {
			cause = err
			// 从 args 中移除最后一个 error 参数
			args = args[:len(args)-1]
		}
	}

	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

// 预定义的常用错误（向后兼容）
var (
	// ErrNotFound 数据未找到
	ErrNotFound = NewError(ErrCodeNotFound, nil)

	// ErrClosed 对象已关闭
	ErrClosed = NewError(ErrCodeClosed, nil)

	// ErrInvalidData 无效数据
	ErrInvalidData = NewError(ErrCodeInvalidData, nil)

	// ErrCorrupted 数据损坏
	ErrCorrupted = NewError(ErrCodeCorrupted, nil)
)

// 数据库错误（向后兼容）
var (
	ErrDatabaseNotFound = NewError(ErrCodeDatabaseNotFound, nil)
	ErrDatabaseExists   = NewError(ErrCodeDatabaseExists, nil)
	ErrDatabaseClosed   = NewError(ErrCodeDatabaseClosed, nil)
)

// 表错误（向后兼容）
var (
	ErrTableNotFound = NewError(ErrCodeTableNotFound, nil)
	ErrTableExists   = NewError(ErrCodeTableExists, nil)
	ErrTableClosed   = NewError(ErrCodeTableClosed, nil)
)

// Schema 错误（向后兼容）
var (
	ErrSchemaNotFound         = NewError(ErrCodeSchemaNotFound, nil)
	ErrSchemaInvalid          = NewError(ErrCodeSchemaInvalid, nil)
	ErrSchemaMismatch         = NewError(ErrCodeSchemaMismatch, nil)
	ErrSchemaValidationFailed = NewError(ErrCodeSchemaValidationFailed, nil)
	ErrSchemaChecksumMismatch = NewError(ErrCodeSchemaChecksumMismatch, nil)
)

// 字段错误（向后兼容）
var (
	ErrFieldNotFound     = NewError(ErrCodeFieldNotFound, nil)
	ErrFieldTypeMismatch = NewError(ErrCodeFieldTypeMismatch, nil)
	ErrFieldRequired     = NewError(ErrCodeFieldRequired, nil)
)

// 索引错误（向后兼容）
var (
	ErrIndexNotFound  = NewError(ErrCodeIndexNotFound, nil)
	ErrIndexExists    = NewError(ErrCodeIndexExists, nil)
	ErrIndexNotReady  = NewError(ErrCodeIndexNotReady, nil)
	ErrIndexCorrupted = NewError(ErrCodeIndexCorrupted, nil)
)

// 文件格式错误（向后兼容）
var (
	ErrInvalidFormat      = NewError(ErrCodeInvalidFormat, nil)
	ErrUnsupportedVersion = NewError(ErrCodeUnsupportedVersion, nil)
	ErrInvalidMagicNumber = NewError(ErrCodeInvalidMagicNumber, nil)
	ErrChecksumMismatch   = NewError(ErrCodeChecksumMismatch, nil)
)

// WAL 错误（向后兼容）
var (
	ErrWALCorrupted = NewError(ErrCodeWALCorrupted, nil)
	ErrWALClosed    = NewError(ErrCodeWALClosed, nil)
)

// SSTable 错误（向后兼容）
var (
	ErrSSTableNotFound  = NewError(ErrCodeSSTableNotFound, nil)
	ErrSSTableCorrupted = NewError(ErrCodeSSTableCorrupted, nil)
)

// Compaction 错误（向后兼容）
var (
	ErrCompactionInProgress = NewError(ErrCodeCompactionInProgress, nil)
	ErrNoCompactionNeeded   = NewError(ErrCodeNoCompactionNeeded, nil)
)

// 编解码错误（向后兼容）
var (
	ErrEncodeFailed = NewError(ErrCodeEncodeFailed, nil)
	ErrDecodeFailed = NewError(ErrCodeDecodeFailed, nil)
)

// 辅助函数

// GetErrorCode 获取错误码
func GetErrorCode(err error) ErrCode {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 0
}

// IsError 判断错误是否匹配指定的错误码
func IsError(err error, code ErrCode) bool {
	return GetErrorCode(err) == code
}

// IsNotFound 判断是否是 NotFound 错误
func IsNotFound(err error) bool {
	code := GetErrorCode(err)
	return code == ErrCodeNotFound ||
		code == ErrCodeTableNotFound ||
		code == ErrCodeDatabaseNotFound ||
		code == ErrCodeIndexNotFound ||
		code == ErrCodeFieldNotFound ||
		code == ErrCodeSchemaNotFound ||
		code == ErrCodeSSTableNotFound
}

// IsCorrupted 判断是否是数据损坏错误
func IsCorrupted(err error) bool {
	code := GetErrorCode(err)
	return code == ErrCodeCorrupted ||
		code == ErrCodeWALCorrupted ||
		code == ErrCodeSSTableCorrupted ||
		code == ErrCodeIndexCorrupted ||
		code == ErrCodeChecksumMismatch ||
		code == ErrCodeSchemaChecksumMismatch
}

// IsClosed 判断是否是已关闭错误
func IsClosed(err error) bool {
	code := GetErrorCode(err)
	return code == ErrCodeClosed ||
		code == ErrCodeDatabaseClosed ||
		code == ErrCodeTableClosed ||
		code == ErrCodeWALClosed
}

// WrapError 包装错误并添加上下文
func WrapError(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", msg, err)
}
