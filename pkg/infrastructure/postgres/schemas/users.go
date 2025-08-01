package schemas

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	Email     string            `gorm:"column:email;not null;uniqueIndex"`
	Password  string            `gorm:"column:password;not null"`
	Role      entities.UserRole `gorm:"column:role;not null;default:'User'"`
	CreatedAt time.Time         `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time         `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt gorm.DeletedAt    `gorm:"column:deleted_at;default:null"`
}

func (User) TableName() string {
	return "users"
}

// HashPassword encrypts the password using bcrypt
func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword compares the provided password with the hashed password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}
