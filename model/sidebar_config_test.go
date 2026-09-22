package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// TestGenerateDefaultSidebarConfigForRole_InviteCode verifies the fork's invite-code
// sidebar entry survives for both admin and root roles.
func TestGenerateDefaultSidebarConfigForRole_InviteCode(t *testing.T) {
	type adminSection struct {
		InviteCode bool `json:"invite_code"`
	}
	type sidebarConfig struct {
		Admin adminSection `json:"admin"`
	}

	for _, role := range []int{common.RoleAdminUser, common.RoleRootUser} {
		raw := generateDefaultSidebarConfigForRole(role)
		require.NotEmpty(t, raw)

		var cfg sidebarConfig
		require.NoError(t, common.UnmarshalJsonStr(raw, &cfg))
		require.True(t, cfg.Admin.InviteCode)
	}
}
