package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBasicAuth(t *testing.T) {
	authStr := base64.StdEncoding.EncodeToString([]byte("username:password"))
	os.Setenv("PROTECT_BASIC", authStr)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BasicAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "Authorized")
	})

	testCases := []struct {
		name           string
		authHeader     string
		expectCode     int
		expectResponse string
	}{
		{
			name:           "Valid Authorization",
			authHeader:     "Basic " + authStr,
			expectCode:     http.StatusOK,
			expectResponse: "Authorized",
		},
		{
			name:           "Invalid Authorization",
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("invalid:credentials")),
			expectCode:     http.StatusUnauthorized,
			expectResponse: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tc.authHeader)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("Expected status code %d, but got %d", tc.expectCode, w.Code)
			}

			if tc.expectResponse != "" && !strings.Contains(w.Body.String(), tc.expectResponse) {
				t.Errorf("Expected response to contain '%s', but got '%s'", tc.expectResponse, w.Body.String())
			}
		})
	}
}
