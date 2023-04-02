package service

import (
	"context"
	"gateway/internal/auth/model"
	mock_service "gateway/internal/auth/service/mock"
	"testing"

	"github.com/golang/mock/gomock"
)

func TestAuthService_Login(t *testing.T) {
	type args struct {
		ctx  context.Context
		data model.LoginRequest
	}
	tests := []struct {
		name            string
		args            args
		wantSignedToken string
		wantErr         bool
	}{
		{
			name: "login",
			args: args{
				ctx: context.TODO(),
				data: model.LoginRequest{
					Email:    "user@mail.com",
					Password: "123123",
				},
			},
			wantSignedToken: "tokenjwt",
			wantErr:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := mock_service.NewMockIAuthService(ctrl)
			gomock.InOrder(
				svc.EXPECT().Login(tt.args.ctx, tt.args.data).Return(tt.wantSignedToken, nil),
			)
			gotSignedToken, err := svc.Login(tt.args.ctx, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthService.Login() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotSignedToken != tt.wantSignedToken {
				t.Errorf("AuthService.Login() = %v, want %v", gotSignedToken, tt.wantSignedToken)
			}
		})
	}
}

func TestAuthService_AdminLogin(t *testing.T) {
	type args struct {
		ctx  context.Context
		data model.LoginRequest
	}
	tests := []struct {
		name            string
		args            args
		wantSignedToken string
		wantErr         bool
	}{
		{
			name: "admin login",
			args: args{
				ctx: context.TODO(),
				data: model.LoginRequest{
					Email:    "admin@mail.com",
					Password: "123123",
				},
			},
			wantSignedToken: "tokenjwt",
			wantErr:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := mock_service.NewMockIAuthService(ctrl)
			gomock.InOrder(
				svc.EXPECT().AdminLogin(tt.args.ctx, tt.args.data).Return(tt.wantSignedToken, nil),
			)
			gotSignedToken, err := svc.AdminLogin(tt.args.ctx, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthService.AdminLogin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotSignedToken != tt.wantSignedToken {
				t.Errorf("AuthService.AdminLogin() = %v, want %v", gotSignedToken, tt.wantSignedToken)
			}
		})
	}
}

func TestAuthService_Logout(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		s       AuthService
		args    args
		wantErr bool
	}{
		{
			name: "logout",
			args: args{
				ctx: context.WithValue(context.Background(), "userID", uint(5)),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.WithValue(tt.args.ctx, "authUUID", "123")
			svc := mock_service.NewMockIAuthService(ctrl)
			gomock.InOrder(
				svc.EXPECT().Logout(ctx).Return(nil),
			)
			if err := tt.s.Logout(ctx); (err != nil) != tt.wantErr {
				t.Errorf("AuthService.Logout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
