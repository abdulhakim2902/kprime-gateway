package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIPWhitelist(t *testing.T) {
	os.Setenv("PROTECT_IP_WHITELISTS", "127.0.0.1,::1")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(IPWhitelist())
	router.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "Authorized")
	})

	reqWhitelisted, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	reqWhitelisted.Header.Set("X-Forwarded-For", "127.0.0.1")
	reqWhitelisted.Header.Set("X-Real-IP", "127.0.0.1")
	respWhitelisted := httptest.NewRecorder()

	router.ServeHTTP(respWhitelisted, reqWhitelisted)

	if respWhitelisted.Code != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, respWhitelisted.Code)
	}

	reqNonWhitelisted, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	reqNonWhitelisted.Header.Set("X-Forwarded-For", "192.168.0.1")
	reqNonWhitelisted.Header.Set("X-Real-IP", "192.168.0.1")
	respNonWhitelisted := httptest.NewRecorder()

	router.ServeHTTP(respNonWhitelisted, reqNonWhitelisted)

	if respNonWhitelisted.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, but got %d", http.StatusUnauthorized, respNonWhitelisted.Code)
	}
}
