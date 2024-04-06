package loader

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/TencentBlueKing/gopkg/collection/set"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/model"
	"github.com/narasux/goblog/pkg/utils/markdownx"
)

// BlogLoader 博客文章加载器
type BlogLoader struct {
	blogData model.BlogData
}

// New ...
func New() *BlogLoader {
	return &BlogLoader{blogData: model.BlogData{}}
}

func (l *BlogLoader) Exec() (*model.BlogData, error) {
	for _, f := range []func() error{
		l.loadArticleMetadata,
		l.loadArticleContent,
		l.collectCategories,
		l.collectTags,
	} {
		if err := f(); err != nil {
			return nil, err
		}
	}
	return &l.blogData, nil
}

// 加载博客文章元数据
func (l *BlogLoader) loadArticleMetadata() error {
	content, err := os.ReadFile(filepath.Join(envs.BlogDataBaseDir, "articles.json"))
	if err != nil {
		return err
	}

	if err = json.Unmarshal(content, &l.blogData.Articles); err != nil {
		return err
	}
	return nil
}

// 加载博客文章内容
func (l *BlogLoader) loadArticleContent() error {
	for idx, article := range l.blogData.Articles {
		content, err := os.ReadFile(filepath.Join(envs.BlogDataBaseDir, "articles", article.ID+".md"))
		if err != nil {
			return err
		}
		l.blogData.Articles[idx].Content = markdownx.ToHTML(string(content))
	}
	return nil
}

// 从元数据中采集分类信息
func (l *BlogLoader) collectCategories() error {
	categories := set.NewStringSet()
	for _, article := range l.blogData.Articles {
		categories.Append(article.Category)
	}
	l.blogData.Categories = categories.ToSlice()
	return nil
}

// 从元数据中采集标签信息
func (l *BlogLoader) collectTags() error {
	tags := set.NewStringSet()
	for _, article := range l.blogData.Articles {
		tags.Append(article.Tags...)
	}
	l.blogData.Tags = tags.ToSlice()
	return nil
}
