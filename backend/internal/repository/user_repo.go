package repository

import (
	"github.com/graydovee/todo-manager/internal/model"
	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) FindByAuthProvider(provider, subject string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("auth_provider = ? AND auth_subject = ?", provider, subject).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) UpsertByAuthProvider(user *model.User) (*model.User, error) {
	existing, err := r.FindByAuthProvider(user.AuthProvider, user.AuthSubject)
	if err == gorm.ErrRecordNotFound {
		if err := r.Create(user); err != nil {
			return nil, err
		}
		return user, nil
	}
	if err != nil {
		return nil, err
	}
	return existing, nil
}
