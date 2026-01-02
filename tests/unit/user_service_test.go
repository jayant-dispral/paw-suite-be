package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	userPkg "github.com/jayant-dispral/brand-threat-be/internal/service/user"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Re-using the mock from the other test file for simplicity.
// In a real project, this would be in a shared 'mocks' package.
type MockUserRepoForUserSvc struct {
	mock.Mock
}

func (m *MockUserRepoForUserSvc) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepoForUserSvc) GetUserById(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepoForUserSvc) Save(ctx context.Context, user domain.User) (string, error) {
	args := m.Called(ctx, user)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepoForUserSvc) UpdateUser(ctx context.Context, id string, update domain.UpdateUserStruct) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func TestUserService_GetUserByEmail(t *testing.T) {
	email := "test@example.com"
	user := &domain.User{
		ID:    primitive.NewObjectID(),
		Email: email,
	}

	testCases := []struct {
		name          string
		email         string
		setupMock     func(*MockUserRepoForUserSvc)
		expectedUser  *domain.User
		expectedError error
	}{
		{
			name:  "Success",
			email: email,
			setupMock: func(mockRepo *MockUserRepoForUserSvc) {
				mockRepo.On("GetUserByEmail", mock.Anything, email).Return(user, nil).Once()
			},
			expectedUser:  user,
			expectedError: nil,
		},
		{
			name:  "User not found",
			email: "notfound@example.com",
			setupMock: func(mockRepo *MockUserRepoForUserSvc) {
				mockRepo.On("GetUserByEmail", mock.Anything, "notfound@example.com").Return(nil, domain.ErrNotFound).Once()
			},
			expectedUser:  nil,
			expectedError: pkgerrors.NewError(domain.ErrNotFound, domain.ErrNotFound),
		},
		{
			name:  "Repository error",
			email: email,
			setupMock: func(mockRepo *MockUserRepoForUserSvc) {
				mockRepo.On("GetUserByEmail", mock.Anything, email).Return(nil, errors.New("db error")).Once()
			},
			expectedUser:  nil,
			expectedError: pkgerrors.NewError(domain.ErrInternal, errors.New("db error")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockUserRepoForUserSvc)
			tc.setupMock(mockRepo)
			userService := userPkg.NewService(mockRepo)

			u, err := userService.GetUserByEmail(context.Background(), tc.email)

			assert.Equal(t, tc.expectedUser, u)
			if tc.expectedError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tc.expectedError))
			} else {
				assert.NoError(t, err)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestUserService_GetUserProfile(t *testing.T) {
	userID := primitive.NewObjectID()
	user := &domain.User{
		ID:    userID,
		Email: "test@example.com",
	}

	testCases := []struct {
		name          string
		userID        string
		setupMock     func(*MockUserRepoForUserSvc)
		expectedUser  *domain.User
		expectedError error
	}{
		{
			name:   "Success",
			userID: userID.Hex(),
			setupMock: func(mockRepo *MockUserRepoForUserSvc) {
				mockRepo.On("GetUserById", mock.Anything, userID.Hex()).Return(user, nil).Once()
			},
			expectedUser:  user,
			expectedError: nil,
		},
		{
			name:   "User not found",
			userID: primitive.NewObjectID().Hex(),
			setupMock: func(mockRepo *MockUserRepoForUserSvc) {
				mockRepo.On("GetUserById", mock.Anything, mock.AnythingOfType("string")).Return(nil, domain.ErrNotFound).Once()
			},
			expectedUser:  nil,
			expectedError: pkgerrors.NewError(domain.ErrNotFound, domain.ErrNotFound),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockUserRepoForUserSvc)
			tc.setupMock(mockRepo)
			userService := userPkg.NewService(mockRepo)

			u, err := userService.GetUserProfile(context.Background(), tc.userID)

			assert.Equal(t, tc.expectedUser, u)
			if tc.expectedError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tc.expectedError))
			} else {
				assert.NoError(t, err)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	newName := "New Name"
	updateData := domain.UpdateUserStruct{
		FullName: &newName,
		Preferences: &domain.UserPreferences{
			EmailNotifications: true,
			Timezone:           "UTC",
		},
	}
	user := &domain.User{
		ID:        primitive.NewObjectID(),
		FullName:  "Old Name",
		CreatedAt: time.Now(),
	}

	testCases := []struct {
		name          string
		userID        string
		updateData    domain.UpdateUserStruct
		setupMock     func(*MockUserRepoForUserSvc)
		expectedError error
	}{
		{
			name:       "Success",
			userID:     userID,
			updateData: updateData,
			setupMock: func(mockRepo *MockUserRepoForUserSvc) {
				mockRepo.On("GetUserById", mock.Anything, userID).Return(user, nil).Once()
				mockRepo.On("UpdateUser", mock.Anything, userID, updateData).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name:       "User not found",
			userID:     userID,
			updateData: updateData,
			setupMock: func(mockRepo *MockUserRepoForUserSvc) {
				mockRepo.On("GetUserById", mock.Anything, userID).Return(nil, domain.ErrNotFound).Once()
			},
			expectedError: pkgerrors.NewError(domain.ErrNotFound, domain.ErrNotFound),
		},
		{
			name:       "Update fails in repo",
			userID:     userID,
			updateData: updateData,
			setupMock: func(mockRepo *MockUserRepoForUserSvc) {
				mockRepo.On("GetUserById", mock.Anything, userID).Return(user, nil).Once()
				mockRepo.On("UpdateUser", mock.Anything, userID, updateData).Return(errors.New("db error")).Once()
			},
			expectedError: pkgerrors.NewError(domain.ErrInternal, errors.New("db error")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockUserRepoForUserSvc)
			tc.setupMock(mockRepo)
			userService := userPkg.NewService(mockRepo)

			err := userService.UpdateUser(context.Background(), tc.userID, tc.updateData)

			if tc.expectedError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tc.expectedError))
			} else {
				assert.NoError(t, err)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}
