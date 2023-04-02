package repository

import (
	"context"
	_adminModel "gateway/internal/admin/model"
	_authModel "gateway/internal/auth/model"
	_userModel "gateway/internal/user/model"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Test_authRepo_InvalidateToken(t *testing.T) {
	type args struct {
		ctx    context.Context
		userID uint
		authID string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "invalidate token +",
			args: args{
				ctx:    context.TODO(),
				userID: 1,
				authID: "123123123",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			defer db.Close()
			dialector := mysql.New(mysql.Config{
				DSN:                       "sqlmock_db_0",
				DriverName:                "mysql",
				Conn:                      db,
				SkipInitializeWithVersion: true,
			})
			gorm, _ := gorm.Open(dialector)

			repo := &authRepo{
				db: gorm,
			}
			if err := repo.InvalidateToken(tt.args.ctx, tt.args.userID, tt.args.authID); (err != nil) != tt.wantErr {
				t.Errorf("authRepo.InvalidateToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_authRepo_GetOneUserByEmail(t *testing.T) {
	type args struct {
		ctx   context.Context
		email string
	}
	tests := []struct {
		name string

		args     args
		wantUser _userModel.Client
		wantErr  bool
	}{
		{
			name: "get 1 user",
			args: args{
				ctx:   context.TODO(),
				email: "user@mail.com",
			},
			wantUser: _userModel.Client{
				Email: "user@mail.com",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			defer db.Close()
			dialector := mysql.New(mysql.Config{
				DSN:                       "sqlmock_db_0",
				DriverName:                "mysql",
				Conn:                      db,
				SkipInitializeWithVersion: true,
			})
			gorm, _ := gorm.Open(dialector)

			repo := &authRepo{
				db: gorm,
			}
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT "email","Role"."id" AS "Role__id","Role"."created_at" AS "Role__created_at","Role"."updated_at" AS "Role__updated_at","Role"."deleted_at" AS "Role__deleted_at","Role"."name" AS "Role__name" FROM "clients" LEFT JOIN "roles" "Role" ON "clients"."role_id" = "Role"."id" AND "Role"."deleted_at" IS NULL WHERE "clients"."email" = ? ORDER BY "clients"."id" LIMIT 1`)).WithArgs(tt.args.email)
			gotUser, err := repo.GetOneUserByEmail(tt.args.ctx, tt.args.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("authRepo.GetOneUserByEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotUser, tt.wantUser) {
				t.Errorf("authRepo.GetOneUserByEmail() = %v, want %v", gotUser, tt.wantUser)
			}
		})
	}
}

func Test_authRepo_GetAdminByEmail(t *testing.T) {
	type args struct {
		ctx   context.Context
		email string
	}
	tests := []struct {
		name      string
		args      args
		wantAdmin _adminModel.Admin
		wantErr   bool
	}{
		{
			name: "Get admin +",
			args: args{
				ctx:   context.TODO(),
				email: "admin@mail.com",
			},
			wantAdmin: _adminModel.Admin{
				Email: "admin@mail.com",
				Name:  "admin",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			defer db.Close()
			dialector := mysql.New(mysql.Config{
				DSN:                       "sqlmock_db_0",
				DriverName:                "mysql",
				Conn:                      db,
				SkipInitializeWithVersion: true,
			})
			gorm, _ := gorm.Open(dialector)

			repo := &authRepo{
				db: gorm,
			}

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT "email","Role"."id" AS "Role__id","Role"."created_at" AS "Role__created_at","Role"."updated_at" AS "Role__updated_at","Role"."deleted_at" AS "Role__deleted_at","Role"."name" AS "Role__name" FROM "admins" LEFT JOIN "roles" "Role" ON "admins"."role_id" = "Role"."id" AND "Role"."deleted_at" IS NULL WHERE "admins"."email" = $1 AND "admins"."deleted_at" IS NULL ORDER BY "admins"."id" LIMIT 1`)).WithArgs(tt.args.email)
			gotAdmin, err := repo.GetAdminByEmail(tt.args.ctx, tt.args.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("authRepo.GetAdminByEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotAdmin, tt.wantAdmin) {
				t.Errorf("authRepo.GetAdminByEmail() = %v, want %v", gotAdmin, tt.wantAdmin)
			}
		})
	}
}

func Test_authRepo_GenerateAuthDetail(t *testing.T) {
	type args struct {
		ctx    context.Context
		userId uint
	}
	tests := []struct {
		name string

		args     args
		wantAuth _authModel.TokenAuth
		wantErr  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			defer db.Close()
			dialector := mysql.New(mysql.Config{
				DSN:                       "sqlmock_db_0",
				DriverName:                "mysql",
				Conn:                      db,
				SkipInitializeWithVersion: true,
			})
			gorm, _ := gorm.Open(dialector)

			repo := &authRepo{
				db: gorm,
			}

			gotAuth, err := repo.GenerateAuthDetail(tt.args.ctx, tt.args.userId)
			if (err != nil) != tt.wantErr {
				t.Errorf("authRepo.GenerateAuthDetail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotAuth, tt.wantAuth) {
				t.Errorf("authRepo.GenerateAuthDetail() = %v, want %v", gotAuth, tt.wantAuth)
			}
		})
	}
}
