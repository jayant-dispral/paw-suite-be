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
	repo ports.UserRepository
}

func NewService(repo ports.UserRepository) ports.AuthService {
	return &service{
		repo: repo,
	}
}

func (s *service) Register(ctx context.Context, email, password string) (string, error) {
	//check if the user existis
	existing, _ := s.repo.GetUserByEmail(ctx, email)
	if existing != nil {
		return "", errors.New("user already exisits")
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
	//checking if the user exists
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	//compare password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))

	if err != nil {
		return "", errors.New("invalid credentials")
	}

	tokenStr, err := jwt.CreateToken(email, user.ID.Hex())
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func (s *service) ValidateToken(ctx context.Context, token string) (string, error) {

	err := jwt.VerifyToken(token)
	if err != nil {
		return "", err
	}

	//TODO: add logic here
	return "user_id_123", nil
}
