package auth

import (
	"context"
	"errors"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
	"github.com/jayant-dispral/brand-threat-be/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	repo      ports.UserRepository
	jwtSecret string
}

func NewService(repo ports.UserRepository, jwtSecret string) ports.AuthService {
	return &service{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

func (s *service) Register(ctx context.Context, email, password string) (string, error) {
	//check if the user exists
	existing, _ := s.repo.GetUserByEmail(ctx, email)
	if existing != nil {
		return "", errors.New("user already exists")
	}

	//hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	//save user
	user := domain.User{
		Email:    email,
		Password: string(hashed),
	}

	return s.repo.Save(ctx, user)
}

func (s *service) Login(ctx context.Context, email, password string) (string, error) {
	// 1. Find User
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	// 2. Compare Password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	// 3. Generate JWT
	return jwt.GenerateToken(user.ID.Hex(), s.jwtSecret)
}

func (s *service) ValidateToken(ctx context.Context, token string) (string, error) {
	return jwt.VerifyToken(token, s.jwtSecret)
}
