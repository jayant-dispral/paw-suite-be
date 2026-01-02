package unit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	pkgHttp "github.com/jayant-dispral/brand-threat-be/internal/adapters/handler/http"
	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService is a mock implementation of the AuthService interface
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Register(ctx context.Context, email, password string) (string, error) {
	args := m.Called(ctx, email, password)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	args := m.Called(ctx, email, password)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockAuthService) ValidateToken(ctx context.Context, token string) (string, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Error(1)
}

func TestAuthMiddleware(t *testing.T) {
	testCases := []struct {
		name                string
		setupMock           func(*MockAuthService)
		authHeader          string
		expectedStatusCode  int
		expectContextUserID bool
		expectedResponse    string
	}{
		{
			name: "Success - Valid Token",
			setupMock: func(mockAuthSvc *MockAuthService) {
				mockAuthSvc.On("ValidateToken", mock.Anything, "valid_token").Return("user123", nil).Once()
			},
			authHeader:          "Bearer valid_token",
			expectedStatusCode:  http.StatusOK,
			expectContextUserID: true,
		},
		{
			name:                "Failure - No Auth Header",
			setupMock:           func(mockAuthSvc *MockAuthService) {},
			authHeader:          "",
			expectedStatusCode:  http.StatusUnauthorized,
			expectContextUserID: false,
		},
		{
			name:                "Failure - Malformed Header",
			setupMock:           func(mockAuthSvc *MockAuthService) {},
			authHeader:          "Bearer",
			expectedStatusCode:  http.StatusBadRequest,
			expectContextUserID: false,
		},
		{
			name:                "Failure - Not Bearer token",
			setupMock:           func(mockAuthSvc *MockAuthService) {},
			authHeader:          "Basic some_token",
			expectedStatusCode:  http.StatusBadRequest,
			expectContextUserID: false,
		},
		{
			name: "Failure - Token Validation Fails",
			setupMock: func(mockAuthSvc *MockAuthService) {
				err := pkgerrors.NewError(domain.ErrUnauthorized, errors.New("token expired"))
				mockAuthSvc.On("ValidateToken", mock.Anything, "invalid_token").Return("", err).Once()
			},
			authHeader:          "Bearer invalid_token",
			expectedStatusCode:  http.StatusUnauthorized,
			expectContextUserID: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock and router
			mockAuthSvc := new(MockAuthService)
			tc.setupMock(mockAuthSvc)

			r := chi.NewRouter()
			r.Use(pkgHttp.AuthMiddleware(mockAuthSvc))
			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				userID, ok := r.Context().Value(pkgHttp.UserIDKey).(string)
				if tc.expectContextUserID {
					assert.True(t, ok, "Expected user ID in context")
					assert.Equal(t, "user123", userID, "Expected correct user ID in context")
				} else {
					assert.False(t, ok, "Did not expect user ID in context")
				}
				w.WriteHeader(http.StatusOK)
			})

			// Create request and recorder
			req := httptest.NewRequest("GET", "/test", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()

			// Serve the request
			r.ServeHTTP(rr, req)

			// Assertions
			assert.Equal(t, tc.expectedStatusCode, rr.Code)
			mockAuthSvc.AssertExpectations(t)
		})
	}
}
