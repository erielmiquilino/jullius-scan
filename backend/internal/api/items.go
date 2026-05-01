package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/erielfranco/jullius-scan/backend/internal/api/middleware"
	"github.com/erielfranco/jullius-scan/backend/internal/database"
)

// --- Request / Response types ---

// ItemSearchResponse is the envelope returned by GET /api/v1/items/search.
type ItemSearchResponse struct {
	Items     []ItemSearchResult  `json:"items"`
	Truncated bool                `json:"truncated"`
	Query     ItemSearchQueryEcho `json:"query"`
}

// ItemSearchResult is one aggregated item in the search response.
type ItemSearchResult struct {
	Description       string          `json:"description"`
	Barcode           *string         `json:"barcode,omitempty"`
	LastPurchasedAt   string          `json:"last_purchased_at"`
	LastUnitPrice     float64         `json:"last_unit_price"`
	LastTotalPrice    float64         `json:"last_total_price"`
	PreviousUnitPrice *float64        `json:"previous_unit_price,omitempty"`
	AverageUnitPrice  float64         `json:"average_unit_price"`
	PurchaseCount     int             `json:"purchase_count"`
	Store             ItemSearchStore `json:"store"`
	ReceiptID         int64           `json:"receipt_id"`
}

// ItemSearchStore is the store reference attached to each result.
type ItemSearchStore struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	CNPJ string `json:"cnpj"`
}

// ItemSearchQueryEcho echoes back the parsed query so clients can reflect what
// was actually applied (notably which match strategy ran).
type ItemSearchQueryEcho struct {
	Q          string `json:"q"`
	PeriodDays *int   `json:"period_days"`
	MatchedBy  string `json:"matched_by"`
}

// --- Handler ---

const (
	itemSearchMinQueryLen = 3
	itemSearchDefaultDays = 30
)

// SearchItems handles GET /api/v1/items/search.
// Returns items already purchased by the authenticated user's house, grouped
// by barcode (when present) or exact description, with last-purchase summary
// and period statistics.
func (h *Handlers) SearchItems(w http.ResponseWriter, r *http.Request) {
	houseID, ok := middleware.GetHouseID(r.Context())
	if !ok {
		respondError(w, http.StatusForbidden, "no house context", "NO_HOUSE")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) < itemSearchMinQueryLen {
		respondError(w, http.StatusBadRequest, "q must be at least 3 non-whitespace characters", "INVALID_QUERY")
		return
	}

	periodDays, err := parsePeriodDays(r.URL.Query())
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error(), "INVALID_PERIOD_DAYS")
		return
	}

	items := database.NewItemQueries(h.db)
	aggregates, truncated, matchedBy, err := items.SearchItemsByHouse(r.Context(), houseID, query, periodDays)
	if err != nil {
		slog.Error("failed to search items", "error", err, "house_id", houseID)
		respondError(w, http.StatusInternalServerError, "failed to search items", "INTERNAL")
		return
	}

	results := make([]ItemSearchResult, 0, len(aggregates))
	for i := range aggregates {
		results = append(results, toItemSearchResult(&aggregates[i]))
	}

	respondJSON(w, http.StatusOK, ItemSearchResponse{
		Items:     results,
		Truncated: truncated,
		Query: ItemSearchQueryEcho{
			Q:          query,
			PeriodDays: periodDays,
			MatchedBy:  matchedBy,
		},
	})
}

// parsePeriodDays decodes the period_days query parameter into a *int.
//
// Semantics:
//   - parameter missing entirely    → default 30 days
//   - parameter present but empty   → no filter (all-time)
//   - parameter present as "null"   → no filter (all-time)
//   - parameter as integer >= 0     → filter by N days
//   - anything else (negative, NaN) → 400 error
func parsePeriodDays(q map[string][]string) (*int, error) {
	values, ok := q["period_days"]
	if !ok {
		def := itemSearchDefaultDays
		return &def, nil
	}
	raw := ""
	if len(values) > 0 {
		raw = strings.TrimSpace(values[0])
	}
	if raw == "" || strings.EqualFold(raw, "null") {
		return nil, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, errInvalidPeriodDays
	}
	if n < 0 {
		return nil, errInvalidPeriodDays
	}
	return &n, nil
}

var errInvalidPeriodDays = &periodDaysError{}

type periodDaysError struct{}

func (e *periodDaysError) Error() string {
	return "period_days must be a non-negative integer, empty, or 'null'"
}

func toItemSearchResult(a *database.ItemSearchAggregate) ItemSearchResult {
	return ItemSearchResult{
		Description:       a.Description,
		Barcode:           a.Barcode,
		LastPurchasedAt:   a.LastPurchasedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		LastUnitPrice:     a.LastUnitPrice,
		LastTotalPrice:    a.LastTotalPrice,
		PreviousUnitPrice: a.PreviousUnitPrice,
		AverageUnitPrice:  a.AverageUnitPrice,
		PurchaseCount:     a.PurchaseCount,
		Store: ItemSearchStore{
			ID:   a.StoreID,
			Name: a.StoreName,
			CNPJ: a.StoreCNPJ,
		},
		ReceiptID: a.ReceiptID,
	}
}
