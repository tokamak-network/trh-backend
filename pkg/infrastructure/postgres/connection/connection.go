package connection

import (
	"fmt"
	"time"

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

	config := &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Warn),
	}

	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		logger.Errorf("Failed to connect to postgres database", "err", err)
		return nil, err
	}

	// Configure connection pool for better performance
	sqlDB, err := db.DB()
	if err != nil {
		logger.Errorf("Failed to get underlying sql.DB", "err", err)
		return nil, err
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)           // Maximum number of idle connections
	sqlDB.SetMaxOpenConns(100)          // Maximum number of open connections
	sqlDB.SetConnMaxLifetime(time.Hour) // Maximum lifetime of a connection

	err = db.AutoMigrate(
		&schemas.Stack{},
		&schemas.Deployment{},
		&schemas.Integration{},
		&schemas.User{},
		&schemas.AWSCredentials{},
		&schemas.Log{},
		&schemas.ApiKey{},
		&schemas.RPCUrl{},
	)

	if err != nil {
		logger.Errorf("Failed to auto migrate DB schemas", "err", err.Error())
		return nil, err
	}

	// Create indexes for better performance
	if err := createIndexes(db); err != nil {
		logger.Errorf("Failed to create indexes", "err", err.Error())
		return nil, err
	}

	return db, nil
}

// createIndexes creates database indexes for better query performance
func createIndexes(db *gorm.DB) error {
	// Stack indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_stacks_status ON stacks(status)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_stacks_network ON stacks(network)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_stacks_created_at ON stacks(created_at)").Error; err != nil {
		return err
	}

	// Deployment indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_deployments_stack_id ON deployments(stack_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_deployments_step ON deployments(step)").Error; err != nil {
		return err
	}

	// Integration indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_integrations_stack_id ON integrations(stack_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_integrations_type ON integrations(type)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_integrations_status ON integrations(status)").Error; err != nil {
		return err
	}

	// User indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)").Error; err != nil {
		return err
	}

	// AWS Credentials indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_aws_credentials_name ON aws_credentials(name)").Error; err != nil {
		return err
	}

	// Logs indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_logs_stack_id ON logs(stack_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_logs_deployment_id ON logs(deployment_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_logs_created_at ON logs(created_at)").Error; err != nil {
		return err
	}

	return nil
}
