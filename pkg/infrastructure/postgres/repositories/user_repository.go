package repositories

import (
	"errors"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *schemas.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) FindByID(id uuid.UUID) (*schemas.User, error) {
	var user schemas.User
	err := r.db.Select("id, email, password, role, created_at, updated_at").Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByEmail(email string) (*schemas.User, error) {
	var user schemas.User
	err := r.db.Select("id, email, password, role, created_at, updated_at").Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(user *schemas.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&schemas.User{}, id).Error
}

func (r *UserRepository) List(offset, limit int) ([]schemas.User, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if limit > 100 {
		limit = 100 // Maximum limit
	}

	var users []schemas.User
	err := r.db.Select("id, email, role, created_at, updated_at").Offset(offset).Limit(limit).Order("created_at DESC").Find(&users).Error
	return users, err
}

func (r *UserRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&schemas.User{}).Count(&count).Error
	return count, err
}
