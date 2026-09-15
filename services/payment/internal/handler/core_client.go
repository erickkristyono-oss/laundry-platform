package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// coreOrder is the subset of Core's order response
// (docs/07-api-contract.md §8) Payment needs to verify a charge.
type coreOrder struct {
	ID            string `json:"id"`
	CustomerID    string `json:"customer_id"`
	Status        string `json:"status"`
	PaymentStatus string `json:"payment_status"`
	TotalAmount   *int64 `json:"total_amount"`
}

// fetchCoreOrder performs the synchronous, service-to-service read
// Payment relies on to independently verify the amount it is charging
// (docs/14-architecture-decisions.md ADR-011: Payment "treats order_id/
// amount_due as externally-supplied facts it independently verifies"
// rather than trusting client input).
func (h *Handler) fetchCoreOrder(ctx context.Context, orderID string) (coreOrder, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.Cfg.CoreServiceURL+"/internal/orders/"+orderID, nil)
	if err != nil {
		return coreOrder{}, err
	}
	res, err := h.HTTP.Do(req)
	if err != nil {
		return coreOrder{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return coreOrder{}, fmt.Errorf("order not found")
	}
	if res.StatusCode != http.StatusOK {
		return coreOrder{}, fmt.Errorf("core returned status %d", res.StatusCode)
	}
	var out coreOrder
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return coreOrder{}, err
	}
	return out, nil
}
