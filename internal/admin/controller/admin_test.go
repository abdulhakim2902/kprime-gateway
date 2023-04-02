package controller

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func Test_adminHandler_CreateNewClient(t *testing.T) {
	type args struct {
		r *gin.Context
	}
	tests := []struct {
		name string
		h    *adminHandler
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.h.CreateNewClient(tt.args.r)
		})
	}
}
