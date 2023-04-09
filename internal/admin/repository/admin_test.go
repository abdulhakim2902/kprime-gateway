package repository

import (
	"context"
	"gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Test_adminRepo_Register(t *testing.T) {
	type args struct {
		ctx  context.Context
		data model.Admin
	}
	tests := []struct {
		name       string
		args       args
		wantAdmin  model.Admin
		wantErr    bool
		beforeTest func(sqlmock.Sqlmock)
	}{
		{
			name: "Register() + Case",
			args: args{
				ctx: context.TODO(),
				data: model.Admin{
					Name:     "Admin 1",
					Email:    "admin@email.com",
					Password: "asdjaukshdbaudad",
					RoleId:   1,
				},
			},
			wantAdmin: model.Admin{
				Name:     "Admin 1",
				Email:    "admin@email.com",
				Password: "asdjaukshdbaudad",
				RoleId:   1,
			},
			wantErr:    false,
			beforeTest: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, _ := sqlmock.New()
			defer db.Close()
			dialector := mysql.New(mysql.Config{
				DSN:                       "sqlmock_db_0",
				DriverName:                "mysql",
				Conn:                      db,
				SkipInitializeWithVersion: true,
			})
			gorm, _ := gorm.Open(dialector)

			repo := &adminRepo{
				db: gorm,
			}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO 'admins' ('created_at','updated_at','deleted_at','name','email','password','role_id') 
			VALUES ($1,$2,$3,$4,$5,$6,$7)`)).WithArgs("2023-03-30 10:41:24.845", "2023-03-30 10:41:24.845", nil, tt.args.data.Name, tt.args.data.Email, tt.args.data.Password, tt.args.data.RoleId)
			gotAdmin, err := repo.Register(tt.args.ctx, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("adminRepo.Register() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotAdmin, tt.wantAdmin) {
				t.Errorf("adminRepo.Register() = %v, want %v", gotAdmin, tt.wantAdmin)
			}
		})
	}
}

func Test_adminRepo_CreateNewClient(t *testing.T) {
	type args struct {
		ctx  context.Context
		data _userModel.Client
	}
	tests := []struct {
		name    string
		args    args
		want    _userModel.Client
		wantErr bool
	}{
		{
			name: "Create Client +",
			args: args{
				ctx: context.TODO(),
				data: _userModel.Client{
					Name:      "Client A",
					Email:     "a@client.com",
					Password:  "123",
					APIKey:    "cla123",
					APISecret: "asdnui1231273",
				},
			},
			want: _userModel.Client{
				Name:      "Client A",
				Email:     "a@client.com",
				Password:  "123",
				APIKey:    "cla123",
				APISecret: "asdnui1231273",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, _ := sqlmock.New()
			defer db.Close()
			dialector := mysql.New(mysql.Config{
				DSN:                       "sqlmock_db_0",
				DriverName:                "mysql",
				Conn:                      db,
				SkipInitializeWithVersion: true,
			})
			gorm, _ := gorm.Open(dialector)

			repo := &adminRepo{
				db: gorm,
			}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO 'clients' ('"name"','"email"','"password"','"client_id"', '"hashed_client_secret"', '"role_id"','"created_at"','"updated_at"') 
			VALUES ($1,$2,$3,$4,$5,$6,$7)`)).WithArgs(tt.args.data.Name, tt.args.data.Email, tt.args.data.Password, tt.args.data.APIKey, tt.args.data.APISecret, nil, "2023-03-30 10:41:24.845", "2023-03-30 10:41:24.845")
			got, err := repo.CreateNewClient(tt.args.ctx, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("adminRepo.CreateNewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("adminRepo.CreateNewClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_adminRepo_GetAllClient(t *testing.T) {
	type args struct {
		ctx   context.Context
		query map[string]interface{}
	}
	tests := []struct {
		name        string
		repo        *adminRepo
		args        args
		wantClients []_userModel.Client
		wantErr     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClients, err := tt.repo.GetAllClient(tt.args.ctx, tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("adminRepo.GetAllClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotClients, tt.wantClients) {
				t.Errorf("adminRepo.GetAllClient() = %v, want %v", gotClients, tt.wantClients)
			}
		})
	}
}
