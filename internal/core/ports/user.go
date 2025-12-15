package ports

import (
	"context"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
)

type UserRepository interface {
	Save(ctx context.Context, user domain.User) (string, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserById(ctx context.Context, id string) (*domain.User, error)
}
