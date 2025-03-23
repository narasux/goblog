package model

import "time"

// BaseModel 基础模型
type BaseModel struct {
	Creator   string    `json:"creator" gorm:"type:varchar(32);null"`
	Updater   string    `json:"updater" gorm:"type:varchar(32);null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
