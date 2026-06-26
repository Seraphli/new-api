package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type InviteCode struct {
	Id          int            `json:"id"`
	Code        string         `json:"code" gorm:"type:varchar(32);uniqueIndex;index"`
	CreatedTime int64          `json:"created_time" gorm:"bigint"`
	UsedTime    int64          `json:"used_time" gorm:"bigint"`
	UsedUserId  int            `json:"used_user_id" gorm:"index"`
	UsedBy      string         `json:"used_by" gorm:"type:varchar(64)"`
	IsUsed      bool           `json:"is_used" gorm:"default:false;index"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func normalizeInviteCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func GetAllInviteCodes(startIdx int, num int) (codes []*InviteCode, total int64, err error) {
	err = DB.Model(&InviteCode{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = DB.Order("id desc").Limit(num).Offset(startIdx).Find(&codes).Error
	if err != nil {
		return nil, 0, err
	}
	return codes, total, nil
}

func GenerateInviteCodes(count int) ([]string, error) {
	if count <= 0 {
		count = 1
	}
	if count > 100 {
		count = 100
	}
	codes := make([]string, 0, count)
	for len(codes) < count {
		code := strings.ToUpper(common.GetRandomString(8))
		item := &InviteCode{
			Code:        code,
			CreatedTime: common.GetTimestamp(),
			IsUsed:      false,
		}
		if err := DB.Create(item).Error; err != nil {
			continue
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func DeleteInviteCodeById(id int) error {
	if id == 0 {
		return errors.New("id is empty")
	}
	return DB.Delete(&InviteCode{}, "id = ?", id).Error
}

func ConsumeInviteCodeTx(tx *gorm.DB, code string, userId int, username string) error {
	if tx == nil {
		return errors.New("transaction cannot be nil")
	}
	normCode := normalizeInviteCode(code)
	if normCode == "" {
		return errors.New("invite code is required")
	}
	var invite InviteCode
	err := tx.Set("gorm:query_option", "FOR UPDATE").Where("code = ?", normCode).First(&invite).Error
	if err != nil {
		return errors.New("invalid or already used invite code")
	}
	if invite.IsUsed {
		return errors.New("invalid or already used invite code")
	}
	result := tx.Model(&InviteCode{}).
		Where("id = ? AND is_used = ?", invite.Id, false).
		Updates(map[string]any{
			"is_used":      true,
			"used_time":    common.GetTimestamp(),
			"used_user_id": userId,
			"used_by":      username,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("invalid or already used invite code")
	}
	return nil
}
