package seeds

import (
	"gateway/database/seeder"

	"gorm.io/gorm"
)

func All() []seeder.Seed {
	return []seeder.Seed{
		{
			Name: "Seed Initial Roles",
			Run: func(db *gorm.DB) error {
				return Seed_Role(db)
			},
		},
		{
			Name: "Seed Initial Admin",
			Run: func(db *gorm.DB) error {
				return Seed_Admin(db)
			},
		},
	}
}
