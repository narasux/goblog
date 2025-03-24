package model

// Article 文章
type Article struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	Title     string   `json:"title"`
	Desc      string   `json:"desc"`
	UpdatedAt string   `json:"updateAt"`
	Content   string   `json:"content"`
}

// Articles 文章列表
type Articles []Article

// BlogData 博客数据
type BlogData struct {
	Categories []string `json:"categories"`
	Tags       []string `json:"tags"`
	Articles   Articles `json:"articles"`
}

// GetByID 根据 ID 获取文章
func (as Articles) GetByID(id string) *Article {
	for _, article := range as {
		if article.ID == id {
			return &article
		}
	}
	return nil
}

// FilterByCategory 根据分类过滤文章
func (as Articles) FilterByCategory(category string) Articles {
	var articles Articles
	for _, article := range as {
		if article.Category == category {
			articles = append(articles, article)
		}
	}
	return articles
}

// FilterByTag 根据标签过滤文章
func (as Articles) FilterByTag(tag string) Articles {
	var articles Articles
	for _, article := range as {
		for _, t := range article.Tags {
			if t == tag {
				articles = append(articles, article)
			}
		}
	}
	return articles
}
