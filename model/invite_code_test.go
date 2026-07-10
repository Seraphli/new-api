package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInviteCodeTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&InviteCode{}, &User{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&InviteCode{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
	t.Cleanup(func() {
		_ = DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&InviteCode{}).Error
		_ = DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error
	})
}

func TestConsumeInviteCodeTx_EmptyCodeFails(t *testing.T) {
	setupInviteCodeTest(t)
	err := DB.Transaction(func(tx *gorm.DB) error {
		return ConsumeInviteCodeTx(tx, "   ", 1, "user")
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "required")
}

func TestConsumeInviteCodeTx_InvalidCodeFails(t *testing.T) {
	setupInviteCodeTest(t)
	err := DB.Transaction(func(tx *gorm.DB) error {
		return ConsumeInviteCodeTx(tx, "DEADCODE", 1, "user")
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "invalid")
}

func TestConsumeInviteCodeTx_FirstSuccessThenSecondFails(t *testing.T) {
	setupInviteCodeTest(t)
	codes, err := GenerateInviteCodes(1)
	require.NoError(t, err)
	require.Len(t, codes, 1)
	code := codes[0]

	err = DB.Transaction(func(tx *gorm.DB) error {
		return ConsumeInviteCodeTx(tx, code, 42, "alice")
	})
	require.NoError(t, err)

	var got InviteCode
	require.NoError(t, DB.Where("code = ?", normalizeInviteCode(code)).First(&got).Error)
	assert.True(t, got.IsUsed)
	assert.Equal(t, 42, got.UsedUserId)
	assert.Equal(t, "alice", got.UsedBy)
	assert.NotZero(t, got.UsedTime)

	err = DB.Transaction(func(tx *gorm.DB) error {
		return ConsumeInviteCodeTx(tx, code, 43, "bob")
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "invalid")
}

func TestConsumeInviteCodeTx_AlreadyUsedFailsWithoutConsume(t *testing.T) {
	setupInviteCodeTest(t)
	require.NoError(t, DB.Create(&InviteCode{
		Code:        "USEDCODE",
		CreatedTime: common.GetTimestamp(),
		IsUsed:      true,
		UsedUserId:  7,
		UsedBy:      "prior",
		UsedTime:    common.GetTimestamp(),
	}).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		return ConsumeInviteCodeTx(tx, "usedcode", 99, "new")
	})
	require.Error(t, err)

	var got InviteCode
	require.NoError(t, DB.Where("code = ?", "USEDCODE").First(&got).Error)
	assert.True(t, got.IsUsed)
	assert.Equal(t, 7, got.UsedUserId)
	assert.Equal(t, "prior", got.UsedBy)
}

func TestRegisterWithInvite_NoCodePathAndConsume(t *testing.T) {
	// Hard gate: empty invite is rejected by controller; model path requires non-empty consume.
	// Also verify InsertWithTx + ConsumeInviteCodeTx atomic success for first use.
	setupInviteCodeTest(t)
	codes, err := GenerateInviteCodes(1)
	require.NoError(t, err)
	code := codes[0]

	user := User{
		Username:    "invitee1",
		Password:    "password123",
		DisplayName: "invitee1",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := user.InsertWithTx(tx, 0); err != nil {
			return err
		}
		return ConsumeInviteCodeTx(tx, code, user.Id, user.Username)
	})
	require.NoError(t, err)
	require.NotZero(t, user.Id)

	// second user same code must fail (code already consumed)
	user2 := User{
		Username:    "invitee2",
		Password:    "password123",
		DisplayName: "invitee2",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := user2.InsertWithTx(tx, 0); err != nil {
			return err
		}
		return ConsumeInviteCodeTx(tx, code, user2.Id, user2.Username)
	})
	require.Error(t, err)
}
