package user

import (
	"context"
	"errors"
	"log"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/pkg/errors"
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
		if errors.Is(err, domain.ErrNotFound) {
			log.Printf("User service get by email error: user not found: %s", email)
			return nil, pkgerrors.NewError(domain.ErrNotFound, err)
		}
		log.Printf("User service get by email error: %v", err)
		return nil, pkgerrors.NewError(domain.ErrInternal, err)
	}
	return user, nil
}

func (s *service) GetUserProfile(ctx context.Context, ID string) (*domain.User, error) {
	user, err := s.repo.GetUserById(ctx, ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Printf("User service get profile error: user not found: %s", ID)
			return nil, pkgerrors.NewError(domain.ErrNotFound, err)
		}
		log.Printf("User service get profile error: %v", err)
		return nil, pkgerrors.NewError(domain.ErrInternal, err)
	}
	return user, nil
}
