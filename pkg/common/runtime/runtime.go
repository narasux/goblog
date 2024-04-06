package runtime

import (
	"github.com/narasux/goblog/pkg/common/runmode"
)

// 以下变量值可通过 --ldflags 的方式修改
var (
	// RunMode 运行模式，可选值为 release，test，debug
	RunMode = runmode.Debug
)
