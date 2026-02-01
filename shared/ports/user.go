package ports

import (
	"context"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

// UserRespository defines how we store users.
// This interface allows us to mock the DB for unit tests.

// UserFilter defines filtering options for user queries
type UserFilter struct {
	SubscriptionTier *domain.SubscriptionTier
	Email            *string
	Limit            int64
	Offset           int64
	SortBy           string // "created_at", "email", "full_name"
	SortOrder        int    // 1 for ascending, -1 for descending
}

type UserRepository interface {
	// Create operations
	Save(ctx context.Context, user domain.User) (string, error)

	// Read operations
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserById(ctx context.Context, id string) (*domain.User, error)
	ListUsers(ctx context.Context, filter UserFilter) ([]domain.User, error)
	FindBySubscriptionTier(ctx context.Context, tier domain.SubscriptionTier) ([]domain.User, error)

	// Update operations
	UpdateUser(ctx context.Context, id string, update domain.UpdateUserStruct) error

	// Delete operations
	Delete(ctx context.Context, id string) error

	// Utility operations
	Count(ctx context.Context, filter UserFilter) (int64, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

type UserService interface {
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserProfile(ctx context.Context, ID string) (*domain.User, error)
	UpdateUser(ctx context.Context, id string, update domain.UpdateUserStruct) error
}
