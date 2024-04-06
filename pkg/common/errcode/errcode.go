package errcode

const (
	// NoErr 无错误
	NoErr = 0

	// TokenInvalid Token 不合法
	TokenInvalid = 40101
	// TokenExpired Token 过期
	TokenExpired = 40102

	// ForTest 测试用错误
	ForTest = 500
	// Unknown 未知错误
	Unknown = 50001
)
