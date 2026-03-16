package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// FirebaseAuth validates Firebase JWT tokens on incoming requests.
type FirebaseAuth struct {
	client *auth.Client
}

// NewFirebaseAuth initializes the Firebase Auth client.
// It uses GOOGLE_APPLICATION_CREDENTIALS env var or Application Default Credentials.
func NewFirebaseAuth(ctx context.Context, projectID string) (*FirebaseAuth, error) {
	conf := &firebase.Config{ProjectID: projectID}
	client, mode, err := newFirebaseAuthClient(ctx, conf)
	if err != nil {
		return nil, err
	}

	slog.Info("firebase auth initialized", "project_id", projectID, "mode", mode)
	return &FirebaseAuth{client: client}, nil
}

func newFirebaseAuthClient(ctx context.Context, conf *firebase.Config) (*auth.Client, string, error) {
	if credsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON"); credsJSON != "" {
		app, err := firebase.NewApp(ctx, conf, option.WithCredentialsJSON([]byte(credsJSON)))
		if err != nil {
			return nil, "", err
		}
		client, err := app.Auth(ctx)
		if err != nil {
			return nil, "", err
		}
		return client, "credentials_json", nil
	}

	app, err := firebase.NewApp(ctx, conf)
	if err == nil {
		client, authErr := app.Auth(ctx)
		if authErr == nil {
			if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
				return client, "google_application_credentials", nil
			}
			return client, "application_default_credentials", nil
		}
		err = authErr
	}

	slog.Warn("firebase auth credentials unavailable, falling back to stateless token verification",
		"error", err,
	)

	app, err = firebase.NewApp(ctx, conf, option.WithoutAuthentication())
	if err != nil {
		return nil, "", err
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, "", err
	}

	return client, "without_authentication_fallback", nil
}

// Authenticate is an HTTP middleware that validates the Bearer token from
// the Authorization header using Firebase Auth. On success, it sets the
// Firebase UID in the request context.
func (fa *FirebaseAuth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			writeError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}
		idToken := parts[1]

		token, err := fa.client.VerifyIDToken(r.Context(), idToken)
		if err != nil {
			slog.Warn("firebase token verification failed",
				"error", err,
				"remote_addr", r.RemoteAddr,
			)
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Set Firebase UID in context
		ctx := context.WithValue(r.Context(), FirebaseUIDKey, token.UID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
