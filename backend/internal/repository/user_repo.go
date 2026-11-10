package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// ErrEmailTaken 邮箱已经被注册，上层据此返回 409
var ErrEmailTaken = errors.New("email already registered")

// DefaultNickname 注册时按用户 ID 生成昵称，如 MORE12。
// 用户 ID 本身不是敏感信息（业务表只按登录态过滤），拿来当默认昵称省一次输入。
func DefaultNickname(userID int) string {
	return fmt.Sprintf("MORE%d", userID)
}

// GetUserByEmail 按邮箱（大小写不敏感）取账号
func (r *Repository) GetUserByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("LOWER(email) = ?", strings.ToLower(strings.TrimSpace(email))).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUser 按主键取账号
func (r *Repository) GetUser(id int) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CountUsers 是否已经有账号，前端用它决定默认停在注册还是登录
func (r *Repository) CountUsers() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Count(&count).Error
	return count, err
}

// CreateUser 落一条账号；邮箱唯一，撞了返回 ErrEmailTaken。
// 昵称在拿到自增 ID 之后补上，注册完就有一个能看的名字。
func (r *Repository) CreateUser(email, passwordHash, passwordSalt string, verified bool) (*model.User, error) {
	user := &model.User{
		Email:         strings.ToLower(strings.TrimSpace(email)),
		EmailVerified: verified,
		PasswordHash:  passwordHash,
		PasswordSalt:  passwordSalt,
	}
	if err := r.db.Create(user).Error; err != nil {
		if isDuplicate(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	user.Nickname = DefaultNickname(user.ID)
	if err := r.db.Model(&model.User{}).Where("id = ?", user.ID).
		Update("nickname", user.Nickname).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateProfile 改昵称与头像；userID 来自登录态，只能改自己那条
func (r *Repository) UpdateProfile(userID int, nickname, avatar string) error {
	return r.updateUser(userID, map[string]any{"nickname": nickname, "avatar": avatar})
}

// SetAvatar 只换头像，昵称不动（上传头像后单独落一次地址）
func (r *Repository) SetAvatar(userID int, avatar string) error {
	return r.updateUser(userID, map[string]any{"avatar": avatar})
}

func (r *Repository) updateUser(userID int, fields map[string]any) error {
	res := r.db.Model(&model.User{}).Where("id = ?", userID).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdatePassword 换密码：哈希与盐一起换，旧密码校验在 handler 里做
func (r *Repository) UpdatePassword(userID int, passwordHash, passwordSalt string) error {
	res := r.db.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]any{"password_hash": passwordHash, "password_salt": passwordSalt})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// BackfillNicknames 老账号没有昵称，按 MORE+ID 补一条，别让界面显示空白
func (r *Repository) BackfillNicknames() error {
	var users []model.User
	if err := r.db.Where("nickname = ? OR nickname IS NULL", "").Find(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		if err := r.db.Model(&model.User{}).Where("id = ?", u.ID).
			Update("nickname", DefaultNickname(u.ID)).Error; err != nil {
			return err
		}
	}
	return nil
}
