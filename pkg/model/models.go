package model

type Article struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	Title     string   `json:"title"`
	Desc      string   `json:"desc"`
	UpdatedAt string   `json:"updateAt"`
	Content   string   `json:"content"`
}

type Articles []Article

type BlogData struct {
	Categories []string `json:"categories"`
	Tags       []string `json:"tags"`
	Articles   Articles `json:"articles"`
}

func (as Articles) GetByID(id string) *Article {
	for _, article := range as {
		if article.ID == id {
			return &article
		}
	}
	return nil
}

func (as Articles) FilterByCategory(category string) Articles {
	var articles Articles
	for _, article := range as {
		if article.Category == category {
			articles = append(articles, article)
		}
	}
	return articles
}

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
