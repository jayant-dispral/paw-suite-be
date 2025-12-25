package user

import (
	"context"
	"errors"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
)

type service struct {
	repo ports.UserRepository
}

func NewService(repo ports.UserRepository) ports.UserService {
	return &service{
		repo: repo,
	}
}

func (s *service) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	//check if the email exists or not
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("invalid email, does not exists on database")
	}
	return user, nil
}
