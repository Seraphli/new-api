package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestInviteCodeRouteRequiresRootNotAdmin builds the real API router via
// SetApiRouter and dispatches authenticated requests through the full
// middleware chain to confirm GET /api/invite-code/ is guarded by
// middleware.RootAuth() (root only), not middleware.AdminAuth().
func TestInviteCodeRouteRequiresRootNotAdmin(t *testing.T) {
	previousDB := model.DB
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.InviteCode{}))
	model.DB = db
	common.RedisEnabled = false
	common.SessionSecret = "invite-code-route-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
	})

	adminUser := &model.User{
		Username: "invite-code-admin", Password: "unused", Role: common.RoleAdminUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "invite-code-admin",
	}
	require.NoError(t, db.Create(adminUser).Error)
	adminSession, err := service.CreateLoginSession(adminUser.Id, "password", "127.0.0.1", "admin-agent")
	require.NoError(t, err)

	rootUser := &model.User{
		Username: "invite-code-root", Password: "unused", Role: common.RoleRootUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "invite-code-root",
	}
	require.NoError(t, db.Create(rootUser).Error)
	rootSession, err := service.CreateLoginSession(rootUser.Id, "password", "127.0.0.1", "root-agent")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	// Admin (role 10) must be denied by middleware.RootAuth() before reaching
	// controller.GetAllInviteCodes: expect authHelper's privilege-denial response.
	adminRecorder := httptest.NewRecorder()
	adminRequest := httptest.NewRequest(http.MethodGet, "/api/invite-code/", nil)
	adminRequest.Header.Set("Authorization", "Bearer "+adminSession.AccessToken)
	engine.ServeHTTP(adminRecorder, adminRequest)

	assert.Equal(t, http.StatusForbidden, adminRecorder.Code)
	var adminResponse struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
	}
	require.NoError(t, common.Unmarshal(adminRecorder.Body.Bytes(), &adminResponse))
	assert.False(t, adminResponse.Success)
	assert.Equal(t, "AUTH_INSUFFICIENT_PRIVILEGE", adminResponse.Code)

	// Root (role 100) must pass middleware.RootAuth() and reach controller.GetAllInviteCodes.
	rootRecorder := httptest.NewRecorder()
	rootRequest := httptest.NewRequest(http.MethodGet, "/api/invite-code/", nil)
	rootRequest.Header.Set("Authorization", "Bearer "+rootSession.AccessToken)
	engine.ServeHTTP(rootRecorder, rootRequest)

	assert.Equal(t, http.StatusOK, rootRecorder.Code)
	var rootResponse struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(rootRecorder.Body.Bytes(), &rootResponse))
	assert.True(t, rootResponse.Success)
}
