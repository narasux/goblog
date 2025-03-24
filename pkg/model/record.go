package model

// ViewRecord 阅读记录
type ViewRecord struct {
	BaseModel
	ID        int64  `json:"id" gorm:"primaryKey"`
	IP        string `json:"ip" gorm:"type:varchar(64);not null"`
	ArticleID string `json:"articleID" gorm:"type:varchar(128);not null"`
}

// LikeRecord 点赞记录
type LikeRecord struct {
	BaseModel
	ID        int64  `json:"id" gorm:"primaryKey"`
	IP        string `json:"ip" gorm:"type:varchar(64);not null"`
	ArticleID string `json:"articleID" gorm:"type:varchar(128);not null"`
}
