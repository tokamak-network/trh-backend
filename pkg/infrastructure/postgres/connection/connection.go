package connection

import (
	"fmt"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

func Init(
	postgresUser string,
	postgresHost string,
	postgresPassword string,
	postgresDatabase string,
	postgresPort string,
) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s TimeZone=UTC",
		postgresHost,
		postgresUser,
		postgresPassword,
		postgresDatabase,
		postgresPort)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Warn),
	})
	if err != nil {
		logger.Errorf("Failed to connect to postgres database", "err", err)
		return nil, err
	}

	err = db.AutoMigrate(&schemas.Stack{}, &schemas.Deployment{}, &schemas.Integration{})
	if err != nil {
		logger.Errorf("Failed to auto migrate DB schemas", "err", err.Error())
		return nil, err
	}

	return db, nil
}
