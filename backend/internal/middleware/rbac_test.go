package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/dto"
)

func TestRBAC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		actor      *dto.Actor
		wantStatus int
	}{
		{"allowed reviewer", &dto.Actor{ID: 1, Role: constants.RoleReviewer}, http.StatusNoContent},
		{"forbidden programmer", &dto.Actor{ID: 2, Role: constants.RoleRobotProgrammer}, http.StatusForbidden},
		{"missing actor", nil, http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.GET("/review", func(context *gin.Context) {
				if test.actor != nil {
					context.Set(actorContextKey, *test.actor)
				}
			}, RBAC(constants.RoleReviewer, constants.RoleAdmin), func(context *gin.Context) { context.Status(http.StatusNoContent) })
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/review", nil))
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}
