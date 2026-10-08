package envx_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/narasux/goblog/pkg/utils/envx"
)

func TestGetEnvWithDefault(t *testing.T) {
	// 不存在的环境变量
	ret := envx.Get("NOT_EXISTS_ENV_KEY", "ENV_VAL")
	assert.Equal(t, "ENV_VAL", ret)

	// 已存在的环境变量
	ret = envx.Get("PATH", "")
	assert.NotEqual(t, "", ret)
}

func TestGetIntWithDefault(t *testing.T) {
	// 不存在的环境变量回退默认值
	assert.Equal(t, 100, envx.GetInt("NOT_EXISTS_INT_KEY", 100))

	// 可解析的整型值
	t.Setenv("FOR_TEST_INT_KEY", "42")
	assert.Equal(t, 42, envx.GetInt("FOR_TEST_INT_KEY", 100))

	// 不可解析的值回退默认值
	t.Setenv("FOR_TEST_BAD_INT_KEY", "not-a-number")
	assert.Equal(t, 100, envx.GetInt("FOR_TEST_BAD_INT_KEY", 100))
}

func TestGetBoolWithDefault(t *testing.T) {
	// 不存在的环境变量回退默认值
	assert.True(t, envx.GetBool("NOT_EXISTS_BOOL_KEY", true))
	assert.False(t, envx.GetBool("NOT_EXISTS_BOOL_KEY", false))

	// strconv.ParseBool 支持的各种真值形式
	for _, val := range []string{"1", "t", "T", "true", "TRUE", "True"} {
		t.Setenv("FOR_TEST_BOOL_KEY", val)
		assert.True(t, envx.GetBool("FOR_TEST_BOOL_KEY", false), "value: %s", val)
	}

	// strconv.ParseBool 支持的各种假值形式
	for _, val := range []string{"0", "f", "F", "false", "FALSE", "False"} {
		t.Setenv("FOR_TEST_BOOL_KEY", val)
		assert.False(t, envx.GetBool("FOR_TEST_BOOL_KEY", true), "value: %s", val)
	}

	// 无法解析的值回退默认值，不应 panic
	t.Setenv("FOR_TEST_BOOL_KEY", "yes")
	assert.True(t, envx.GetBool("FOR_TEST_BOOL_KEY", true))
	assert.False(t, envx.GetBool("FOR_TEST_BOOL_KEY", false))
}
