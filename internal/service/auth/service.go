package auth

import (
	"context"
	"errors"
	"log"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/pkg/errors"
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
	existing, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		log.Printf("Auth service register error: failed to check existing user: %v", err)
		return "", pkgerrors.NewError(domain.ErrInternal, err)
	}
	if existing != nil {
		log.Printf("Auth service register error: user already exists: %s", email)
		return "", pkgerrors.NewError(domain.ErrConflict, nil)
	}

	//hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Auth service register error: password hashing failed: %v", err)
		return "", pkgerrors.NewError(domain.ErrInternal, err)
	}

	//save user
	user := domain.User{
		Email:    email,
		Password: string(hashed),
	}

	id, err := s.repo.Save(ctx, user)
	if err != nil {
		log.Printf("Auth service register error: failed to save user: %v", err)
		return "", pkgerrors.NewError(domain.ErrInternal, err)
	}
	return id, nil
}

func (s *service) Login(ctx context.Context, email, password string) (string, string, error) {
	// 1. Find User
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Printf("Auth service login error: user not found: %s", email)
			return "", "", pkgerrors.NewError(domain.ErrInvalidCredentials, err)
		}
		log.Printf("Auth service login error: failed to get user: %v", err)
		return "", "", pkgerrors.NewError(domain.ErrInternal, err)
	}

	// 2. Compare Password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		log.Printf("Auth service login error: invalid password for user: %s", email)
		return "", "", pkgerrors.NewError(domain.ErrInvalidCredentials, err)
	}

	// 3. Generate JWT
	token, err := jwt.GenerateToken(user.ID.Hex(), s.jwtSecret)
	if err != nil {
		log.Printf("Auth service login error: token generation failed: %v", err)
		return "", "", pkgerrors.NewError(domain.ErrInternal, err)
	}

	return token, user.ID.Hex(), nil
}

func (s *service) ValidateToken(ctx context.Context, token string) (string, error) {
	userID, err := jwt.VerifyToken(token, s.jwtSecret)
	if err != nil {
		log.Printf("Auth service validate token error: %v", err)
		return "", pkgerrors.NewError(domain.ErrUnauthorized, err)
	}
	return userID, nil
}
