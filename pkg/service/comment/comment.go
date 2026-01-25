package comment

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/model"
)

var (
	// ErrCommentNotFound 评论不存在
	ErrCommentNotFound = errors.New("comment not found")
	// ErrCommentOwnerOnly 评论只能由作者编辑或删除
	ErrCommentOwnerOnly = errors.New("only comment owner can perform this action")
)

// CommentWithRelations 带关联信息的评论
type CommentWithRelations struct {
	ID        int64                 `json:"id"`
	ArticleID string                `json:"articleID"`
	Content   string                `json:"content"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
	User      *CommentUser          `json:"user"`
	Parent    *CommentWithRelations `json:"parent,omitempty"`
}

// CommentUser 评论用户信息
type CommentUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatarURL"`
}

// GetCommentsByArticleID 获取文章的所有评论
func GetCommentsByArticleID(ctx context.Context, articleID string) ([]CommentWithRelations, error) {
	db := database.Client(ctx)

	var comments []model.Comment
	if err := db.Where("article_id = ?", articleID).
		Order("created_at DESC").
		Find(&comments).Error; err != nil {
		return nil, err
	}

	// 构建评论 ID 到评论的映射
	commentMap := make(map[int64]model.Comment)
	commentIDs := make([]int64, 0, len(comments))
	for _, c := range comments {
		commentMap[c.ID] = c
		commentIDs = append(commentIDs, c.ID)
	}

	// 获取用户信息
	userIDs := make([]int64, 0, len(comments))
	for _, c := range comments {
		userIDs = append(userIDs, c.UserID)
	}

	var users []model.User
	if err := db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}

	userMap := make(map[int64]model.User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	// 构建结果
	results := make([]CommentWithRelations, 0, len(comments))
	for _, c := range comments {
		commentRel := CommentWithRelations{
			ID:        c.ID,
			ArticleID: c.ArticleID,
			Content:   c.Content,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		}

		// 添加用户信息
		if user, ok := userMap[c.UserID]; ok {
			commentRel.User = &CommentUser{
				ID:        user.ID,
				Username:  user.Username,
				AvatarURL: user.AvatarURL,
			}
		}

		// 添加父评论信息
		if c.ParentID != nil {
			if parent, ok := commentMap[*c.ParentID]; ok {
				commentRel.Parent = &CommentWithRelations{
					ID:        parent.ID,
					ArticleID: parent.ArticleID,
					Content:   parent.Content,
					CreatedAt: parent.CreatedAt,
					UpdatedAt: parent.UpdatedAt,
				}
				if parentUser, ok := userMap[parent.UserID]; ok {
					commentRel.Parent.User = &CommentUser{
						ID:        parentUser.ID,
						Username:  parentUser.Username,
						AvatarURL: parentUser.AvatarURL,
					}
				}
			}
		}

		results = append(results, commentRel)
	}

	return results, nil
}

// CreateComment 创建新评论
func CreateComment(
	ctx context.Context, articleID string, userID int64, content string, parentID *int64,
) (*model.Comment, error) {
	db := database.Client(ctx)

	comment := &model.Comment{
		ArticleID: articleID,
		UserID:    userID,
		ParentID:  parentID,
		Content:   content,
		BaseModel: model.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := db.Create(comment).Error; err != nil {
		return nil, err
	}

	return comment, nil
}

// UpdateComment 更新评论
func UpdateComment(ctx context.Context, commentID int64, userID int64, content string) error {
	db := database.Client(ctx)

	// 查询评论并验证所有权
	comment, err := getAndValidateCommentOwnership(db, commentID, userID)
	if err != nil {
		return err
	}

	// 更新评论
	updates := map[string]any{
		"content":    content,
		"updated_at": time.Now(),
	}

	return db.Model(comment).Updates(updates).Error
}

// DeleteComment 删除评论
func DeleteComment(ctx context.Context, commentID int64, userID int64) error {
	db := database.Client(ctx)

	// 查询评论并验证所有权
	comment, err := getAndValidateCommentOwnership(db, commentID, userID)
	if err != nil {
		return err
	}

	// 软删除评论
	return db.Delete(comment).Error
}

// getAndValidateCommentOwnership 查询评论并验证所有权
func getAndValidateCommentOwnership(db *gorm.DB, commentID int64, userID int64) (*model.Comment, error) {
	var comment model.Comment
	if err := db.Where("id = ?", commentID).First(&comment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCommentNotFound
		}
		return nil, err
	}

	// 验证所有权
	if comment.UserID != userID {
		return nil, ErrCommentOwnerOnly
	}

	return &comment, nil
}
