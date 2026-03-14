package middleware

import "context"

// contextKey is used for storing values in request context.
type contextKey string

const (
	// UserIDKey stores the authenticated user's database ID.
	UserIDKey contextKey = "user_id"

	// FirebaseUIDKey stores the Firebase UID from the JWT.
	FirebaseUIDKey contextKey = "firebase_uid"

	// HouseIDKey stores the resolved House ID for the authenticated user.
	HouseIDKey contextKey = "house_id"
)

// GetUserID retrieves the user ID from the request context.
func GetUserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(UserIDKey).(int64)
	return id, ok
}

// GetFirebaseUID retrieves the Firebase UID from the request context.
func GetFirebaseUID(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(FirebaseUIDKey).(string)
	return uid, ok
}

// GetHouseID retrieves the House ID from the request context.
func GetHouseID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(HouseIDKey).(int64)
	return id, ok
}
