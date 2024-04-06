package funcs

import (
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
)

func NewFuncMap() template.FuncMap {
	funcMap := sprig.FuncMap()
	// 获取当前年份
	funcMap["curYear"] = func() int {
		return time.Now().Year()
	}
	return funcMap
}
