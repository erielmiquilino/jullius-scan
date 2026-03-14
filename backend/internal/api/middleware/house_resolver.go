package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/erielfranco/jullius-scan/backend/internal/database"
)

// ResolveHouse is an HTTP middleware that resolves the authenticated user's
// database record and active House from the Firebase UID set by the auth middleware.
// It sets both UserID and HouseID in the request context.
func ResolveHouse(db *database.DB) func(http.Handler) http.Handler {
	userQueries := database.NewUserQueries(db)
	houseQueries := database.NewHouseQueries(db)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			firebaseUID, ok := GetFirebaseUID(r.Context())
			if !ok || firebaseUID == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "authentication required",
				})
				return
			}

			// Look up the user by Firebase UID
			user, err := userQueries.FindByFirebaseID(r.Context(), firebaseUID)
			if err != nil {
				slog.Warn("user not found for firebase uid",
					"firebase_uid", firebaseUID,
					"error", err,
				)
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error": "user not provisioned — contact administrator",
				})
				return
			}

			// Look up the user's active House
			house, err := houseQueries.FindActiveHouseForUser(r.Context(), user.ID)
			if err != nil {
				slog.Warn("no house membership for user",
					"user_id", user.ID,
					"firebase_uid", firebaseUID,
					"error", err,
				)
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error": "no house membership found — contact administrator",
				})
				return
			}

			// Enrich context with user and house identifiers
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, user.ID)
			ctx = context.WithValue(ctx, HouseIDKey, house.ID)

			slog.Debug("request context resolved",
				"user_id", user.ID,
				"house_id", house.ID,
				"firebase_uid", firebaseUID,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
