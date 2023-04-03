package controller

import (
	mock_controller "gateway/internal/auth/controller/mock"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
)

func Test_authHandler_Login(t *testing.T) {
	type args struct {
		r *gin.Context
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "login handler",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := mock_controller.NewMockIAuthHandler(ctrl)
			gomock.InOrder(
				svc.EXPECT().Login(tt.args.r),
			)
			svc.Login(tt.args.r)
		})
	}
}

func Test_authHandler_AdminLogin(t *testing.T) {
	type args struct {
		r *gin.Context
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "admin login handler",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := mock_controller.NewMockIAuthHandler(ctrl)
			gomock.InOrder(
				svc.EXPECT().AdminLogin(tt.args.r),
			)
			svc.AdminLogin(tt.args.r)
		})
	}
}
