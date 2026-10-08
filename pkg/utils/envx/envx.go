package envx

import (
	"os"
	"strconv"
)

// Get 读取环境变量，支持默认值
func Get(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// GetInt 读取整型环境变量，支持默认值
func GetInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

// GetBool 读取布尔型环境变量，支持默认值
// 仅当值为 strconv.ParseBool 可识别的形式（如 1/t/true/0/f/false）时才生效，否则回退到默认值
func GetBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return fallback
}
