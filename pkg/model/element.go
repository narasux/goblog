package model

// Example 例子
type Example struct {
	Name        string `json:"name"`
	Description string `json:"desc"`
}

// WatchOut 注意事项
type WatchOut struct {
	Name        string `json:"name"`
	Description string `json:"desc"`
}

// ArticleRef 文章引用
type ArticleRef struct {
	Name   string `json:"name"`
	Author string `json:"author"`
	Year   string `json:"year"`
	Url    string `json:"url"`
}

// Element 元素
type Element struct {
	Symbol      string       `json:"symbol"`
	Name        string       `json:"name"`
	Description string       `json:"desc"`
	Examples    []Example    `json:"examples"`
	WatchOuts   []WatchOut   `json:"watchOuts"`
	Articles    []ArticleRef `json:"articles"`
}

// ElementGroup 元素族
type ElementGroup struct {
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Description string    `json:"desc"`
	Elements    []Element `json:"elements"`
}

// ElementPeriodicTable 元素周期表
type ElementPeriodicTable struct {
	Name   string         `json:"name"`
	Source string         `json:"source"`
	Groups []ElementGroup `json:"groups"`
}
