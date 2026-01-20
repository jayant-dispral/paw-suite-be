package ports

import (
	"context"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

// UserRespository defines how we store users.
// This interface allows us to mock the DB for unit tests.

type UserRepository interface {
	Save(ctx context.Context, user domain.User) (string, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserById(ctx context.Context, id string) (*domain.User, error)
	UpdateUser(ctx context.Context, id string, update domain.UpdateUserStruct) error
}

type UserService interface {
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserProfile(ctx context.Context, ID string) (*domain.User, error)
	UpdateUser(ctx context.Context, id string, update domain.UpdateUserStruct) error
}
