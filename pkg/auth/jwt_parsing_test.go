package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/audit"
	"github.com/go-kit/kit/auth/jwt"
)

func TestParseJWTClaims(t *testing.T) {
	createJWT := func(claims map[string]interface{}) string {
		header := map[string]interface{}{
			"alg": "HS256",
			"typ": "JWT",
		}
		headerJSON, _ := json.Marshal(header)
		headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

		claimsJSON, _ := json.Marshal(claims)
		claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

		signature := "fake-signature"

		return headerB64 + "." + claimsB64 + "." + signature
	}

	tests := []struct {
		name        string
		claims      map[string]interface{}
		wantUser    *audit.UserContext
		wantErr     bool
		errContains string
	}{
		{
			name: "complete claims",
			claims: map[string]interface{}{
				"iss":                "https://auth.example.com",
				"email":              "john.doe@example.com",
				"name":               "John Doe",
				"sub":                "user123",
				"aud":                "client456",
				"preferred_username": "johndoe",
			},
			wantUser: &audit.UserContext{
				UserID:    "john.doe@example.com",
				UserEmail: "john.doe@example.com",
				UserName:  "John Doe",
				Subject:   "user123",
				Issuer:    "https://auth.example.com",
				ClientID:  "client456",
			},
		},
		{
			name: "minimal claims",
			claims: map[string]interface{}{
				"iss":   "https://auth.example.com",
				"email": "jane@example.com",
				"sub":   "user456",
			},
			wantUser: &audit.UserContext{
				UserID:    "jane@example.com",
				UserEmail: "jane@example.com",
				UserName:  "",
				Subject:   "user456",
				Issuer:    "https://auth.example.com",
				ClientID:  "",
			},
		},
		{
			name: "use preferred_username when name missing",
			claims: map[string]interface{}{
				"iss":                "https://auth.example.com",
				"email":              "user@example.com",
				"sub":                "user789",
				"preferred_username": "cooluser",
			},
			wantUser: &audit.UserContext{
				UserID:    "user@example.com",
				UserEmail: "user@example.com",
				UserName:  "cooluser",
				Subject:   "user789",
				Issuer:    "https://auth.example.com",
				ClientID:  "",
			},
		},
		{
			name: "use given_name and family_name",
			claims: map[string]interface{}{
				"iss":         "https://auth.example.com",
				"email":       "user@example.com",
				"sub":         "user101",
				"given_name":  "Alice",
				"family_name": "Johnson",
			},
			wantUser: &audit.UserContext{
				UserID:    "user@example.com",
				UserEmail: "user@example.com",
				UserName:  "Alice Johnson",
				Subject:   "user101",
				Issuer:    "https://auth.example.com",
				ClientID:  "",
			},
		},
		{
			name: "only given_name",
			claims: map[string]interface{}{
				"iss":        "https://auth.example.com",
				"email":      "user@example.com",
				"sub":        "user102",
				"given_name": "Bob",
			},
			wantUser: &audit.UserContext{
				UserID:    "user@example.com",
				UserEmail: "user@example.com",
				UserName:  "Bob",
				Subject:   "user102",
				Issuer:    "https://auth.example.com",
				ClientID:  "",
			},
		},
		{
			name: "only family_name",
			claims: map[string]interface{}{
				"iss":         "https://auth.example.com",
				"email":       "user@example.com",
				"sub":         "user103",
				"family_name": "Smith",
			},
			wantUser: &audit.UserContext{
				UserID:    "user@example.com",
				UserEmail: "user@example.com",
				UserName:  "Smith",
				Subject:   "user103",
				Issuer:    "https://auth.example.com",
				ClientID:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := createJWT(tt.claims)

			user, err := parseJWTClaims(token)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseJWTClaims() expected error, got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("parseJWTClaims() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("parseJWTClaims() unexpected error = %v", err)
				return
			}

			if user == nil {
				t.Fatal("parseJWTClaims() returned nil user")
			}

			if user.UserID != tt.wantUser.UserID {
				t.Errorf("UserID = %v, want %v", user.UserID, tt.wantUser.UserID)
			}
			if user.UserEmail != tt.wantUser.UserEmail {
				t.Errorf("UserEmail = %v, want %v", user.UserEmail, tt.wantUser.UserEmail)
			}
			if user.UserName != tt.wantUser.UserName {
				t.Errorf("UserName = %v, want %v", user.UserName, tt.wantUser.UserName)
			}
			if user.Subject != tt.wantUser.Subject {
				t.Errorf("Subject = %v, want %v", user.Subject, tt.wantUser.Subject)
			}
			if user.Issuer != tt.wantUser.Issuer {
				t.Errorf("Issuer = %v, want %v", user.Issuer, tt.wantUser.Issuer)
			}
			if user.ClientID != tt.wantUser.ClientID {
				t.Errorf("ClientID = %v, want %v", user.ClientID, tt.wantUser.ClientID)
			}
		})
	}
}

func TestParseJWTClaimsErrors(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		errContains string
	}{
		{
			name:        "malformed token - not 3 parts",
			token:       "invalid.token",
			errContains: "malformed jwt, expected 3 parts",
		},
		{
			name:        "invalid base64 payload",
			token:       "header.invalid-base64.signature",
			errContains: "failed to unmarshal claims",
		},
		{
			name:        "invalid json payload",
			token:       "header." + base64.RawURLEncoding.EncodeToString([]byte("invalid-json")) + ".signature",
			errContains: "failed to unmarshal claims",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseJWTClaims(tt.token)
			if err == nil {
				t.Errorf("parseJWTClaims() expected error, got nil")
			} else if !containsString(err.Error(), tt.errContains) {
				t.Errorf("parseJWTClaims() error = %v, want error containing %v", err, tt.errContains)
			}
		})
	}
}

func TestAuthMiddlewareWithAuditContext(t *testing.T) {

	testProvider := &testAuthProvider{
		shouldSucceed: true,
	}

	middleware := Middleware(testProvider)

	var capturedCtx context.Context
	testEndpoint := func(ctx context.Context, request interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "success", nil
	}

	wrappedEndpoint := middleware(testEndpoint)

	claims := map[string]interface{}{
		"iss":   "https://auth.example.com",
		"email": "test@example.com",
		"name":  "Test User",
		"sub":   "user123",
	}
	token := createTestJWT(claims)

	ctx := context.WithValue(context.Background(), jwt.JWTContextKey, token)

	_, err := wrappedEndpoint(ctx, "test-request")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	user := audit.GetUserFromContext(capturedCtx)
	if user == nil {
		t.Fatal("Expected user context to be set, got nil")
	}

	if user.UserEmail != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got %q", user.UserEmail)
	}
	if user.UserName != "Test User" {
		t.Errorf("Expected name 'Test User', got %q", user.UserName)
	}
}

type testAuthProvider struct {
	shouldSucceed bool
}

func (t *testAuthProvider) Verify(ctx context.Context, token string) error {
	if !t.shouldSucceed {
		return errors.New("verification failed")
	}
	return nil
}

func (t *testAuthProvider) String() string {
	return "test-auth-provider"
}

func createTestJWT(claims map[string]interface{}) string {
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signature := "fake-signature"
	return headerB64 + "." + claimsB64 + "." + signature
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) > 0 && len(s) > 0 && findIndex(s, substr) >= 0))
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
