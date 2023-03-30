package service

import (
	"context"
	_userModel "gateway/internal/user/model"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
)

func Test_adminService_CreateNewClient(t *testing.T) {
	type args struct {
		ctx  context.Context
		data _userModel.CreateClient
	}
	tests := []struct {
		name    string
		args    args
		want    _userModel.Client
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		mockRepo := gomock.NewController(t)
		svc := &adminService{
			repo: mockRepo,
		}
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CreateNewClient(tt.args.ctx, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("adminService.CreateNewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("adminService.CreateNewClient() = %v, want %v", got, tt.want)
			}
		})
	}
}
