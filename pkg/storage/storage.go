package storage

import (
	"sync"

	"github.com/narasux/goblog/pkg/loader"
	"github.com/narasux/goblog/pkg/model"
)

var BlogData *model.BlogData

var initOnce sync.Once

// InitBlogData 加载并初始化博客数据
func InitBlogData() {
	if BlogData != nil {
		return
	}
	initOnce.Do(func() {
		var err error
		if BlogData, err = loader.New().Exec(); err != nil {
			panic(err)
		}
	})
}
