package model

// Comment 文章评论
type Comment struct {
	BaseModel
	ID        int64  `json:"id" gorm:"primaryKey"`
	ArticleID string `json:"articleID" gorm:"type:varchar(64);index;not null"` // 文章 ID
	UserID    int64  `json:"userID" gorm:"index;not null"`                     // 用户 ID
	ParentID  *int64 `json:"parentID" gorm:"index"`                            // 父评论 ID（用于引用，nil 表示顶级评论）
	Content   string `json:"content" gorm:"type:text;not null"`                // 评论内容（Markdown 格式）
}
