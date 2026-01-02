package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/service/auth"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/pkg/errors"
	"github.com/jayant-dispral/brand-threat-be/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

// MockUserRepo is a mock implementation of the UserRepository interface
type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepo) GetUserById(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepo) Save(ctx context.Context, user domain.User) (string, error) {
	args := m.Called(ctx, user)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepo) UpdateUser(ctx context.Context, id string, user domain.UpdateUserStruct) error {
	args := m.Called(ctx, id, user)
	return args.Error(0)
}

const jwtSecret = "test-secret"

func TestAuthService_Register(t *testing.T) {
	password := "password123"
	email := "test@example.com"
	user := domain.User{
		Email:    email,
		Password: "hashed_password",
	}

	testCases := []struct {
		name          string
		setupMock     func(*MockUserRepo)
		expectedID    string
		expectedError error
	}{
		{
			name: "Success",
			setupMock: func(mockRepo *MockUserRepo) {
				mockRepo.On("GetUserByEmail", mock.Anything, email).Return(nil, domain.ErrNotFound).Once()
				mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
					return u.Email == email
				})).Return("new_user_id", nil).Once()
			},
			expectedID:    "new_user_id",
			expectedError: nil,
		},
		{
			name: "User already exists",
			setupMock: func(mockRepo *MockUserRepo) {
				mockRepo.On("GetUserByEmail", mock.Anything, email).Return(&user, nil).Once()
			},
			expectedID:    "",
			expectedError: pkgerrors.NewError(domain.ErrConflict, nil),
		},
		{
			name: "Failed to save user",
			setupMock: func(mockRepo *MockUserRepo) {
				mockRepo.On("GetUserByEmail", mock.Anything, email).Return(nil, domain.ErrNotFound).Once()
				mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
					return u.Email == email
				})).Return("", errors.New("db error")).Once()
			},
			expectedID:    "",
			expectedError: pkgerrors.NewError(domain.ErrInternal, errors.New("db error")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockUserRepo)
			tc.setupMock(mockRepo)
			authService := auth.NewService(mockRepo, jwtSecret)

			id, err := authService.Register(context.Background(), email, password)

			assert.Equal(t, tc.expectedID, id)
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

func TestAuthService_Login(t *testing.T) {
	password := "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	email := "test@example.com"
	user := &domain.User{
		ID:       primitive.NewObjectID(),
		Email:    email,
		Password: string(hashedPassword),
	}

	testCases := []struct {
		name           string
		email          string
		password       string
		setupMock      func(*MockUserRepo)
		expectedToken  bool
		expectedUserID string
		expectedError  error
	}{
		{
			name:     "Successful login",
			email:    email,
			password: password,
			setupMock: func(mockRepo *MockUserRepo) {
				mockRepo.On("GetUserByEmail", mock.Anything, email).Return(user, nil).Once()
			},
			expectedToken:  true,
			expectedUserID: user.ID.Hex(),
			expectedError:  nil,
		},
		{
			name:     "User not found",
			email:    "nonexistent@example.com",
			password: password,
			setupMock: func(mockRepo *MockUserRepo) {
				mockRepo.On("GetUserByEmail", mock.Anything, "nonexistent@example.com").Return(nil, domain.ErrNotFound).Once()
			},
			expectedToken:  false,
			expectedUserID: "",
			expectedError:  pkgerrors.NewError(domain.ErrInvalidCredentials, domain.ErrNotFound),
		},
		{
			name:     "Invalid password",
			email:    email,
			password: "wrongpassword",
			setupMock: func(mockRepo *MockUserRepo) {
				mockRepo.On("GetUserByEmail", mock.Anything, email).Return(user, nil).Once()
			},
			expectedToken:  false,
			expectedUserID: "",
			expectedError:  pkgerrors.NewError(domain.ErrInvalidCredentials, bcrypt.ErrMismatchedHashAndPassword),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockUserRepo)
			tc.setupMock(mockRepo)
			authService := auth.NewService(mockRepo, jwtSecret)

			token, userID, err := authService.Login(context.Background(), tc.email, tc.password)

			if tc.expectedToken {
				assert.NotEmpty(t, token)
			} else {
				assert.Empty(t, token)
			}
			assert.Equal(t, tc.expectedUserID, userID)
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

func TestAuthService_ValidateToken(t *testing.T) {
	userID := primitive.NewObjectID().Hex()

	testCases := []struct {
		name          string
		token         string
		expectedID    string
		expectedError error
	}{
		{
			name: "Valid Token",
			token: func() string {
				token, _ := jwt.GenerateToken(userID, jwtSecret)
				return token
			}(),
			expectedID:    userID,
			expectedError: nil,
		},
		{
			name:          "Invalid Token - Bad Signature",
			token:         "invalid.token.string",
			expectedID:    "",
			expectedError: pkgerrors.NewError(domain.ErrUnauthorized, nil),
		},
		{
			name: "Invalid Token - Wrong Secret",
			token: func() string {
				token, _ := jwt.GenerateToken(userID, "wrong-secret")
				return token
			}(),
			expectedID:    "",
			expectedError: pkgerrors.NewError(domain.ErrUnauthorized, nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := auth.NewService(nil, jwtSecret) // No repo needed for simple token validation
			id, err := service.ValidateToken(context.Background(), tc.token)

			assert.Equal(t, tc.expectedID, id)
			if tc.expectedError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tc.expectedError))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
