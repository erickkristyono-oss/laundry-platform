package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// coreOrder/coreCustomer are the subsets of Core's internal responses
// Notification needs to render a message (docs/07-api-contract.md §4, §8).
type coreOrder struct {
	ID         string `json:"id"`
	OrderCode  string `json:"order_code"`
	CustomerID string `json:"customer_id"`
}

type coreCustomer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

func (h *Handler) fetchCoreOrder(ctx context.Context, orderID string) (coreOrder, error) {
	var out coreOrder
	return out, h.getJSON(ctx, h.Cfg.CoreServiceURL+"/internal/orders/"+orderID, &out)
}

func (h *Handler) fetchCoreCustomer(ctx context.Context, customerID string) (coreCustomer, error) {
	var out coreCustomer
	return out, h.getJSON(ctx, h.Cfg.CoreServiceURL+"/internal/customers/"+customerID, &out)
}

func (h *Handler) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := h.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("core returned status %d for %s", res.StatusCode, url)
	}
	return json.NewDecoder(res.Body).Decode(out)
}
