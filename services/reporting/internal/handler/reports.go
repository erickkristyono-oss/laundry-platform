package handler

import (
	"net/http"
	"strconv"
	"time"

	"laundry-platform/shared/httpauth"
	"laundry-platform/shared/respond"
)

func dateRange(r *http.Request) (from, to string) {
	q := r.URL.Query()
	from = q.Get("from")
	to = q.Get("to")
	if from == "" {
		from = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	return from, to
}

// outletFilter resolves the effective outlet scope for a report request:
// non-global staff are always restricted to their own outlets regardless
// of what they pass in ?outlet_id= (docs/09-rbac.md §4).
func outletFilter(r *http.Request) (outletIDs []string, restricted bool) {
	principal := httpauth.FromRequest(r)
	if principal.IsGlobalStaff() {
		if v := r.URL.Query().Get("outlet_id"); v != "" {
			return []string{v}, true
		}
		return nil, false
	}
	return principal.OutletIDs, true
}

type dailySales struct {
	SalesDate      string `json:"sales_date"`
	OutletID       string `json:"outlet_id"`
	GrossAmount    int64  `json:"gross_amount"`
	RefundedAmount int64  `json:"refunded_amount"`
	OrderCount     int    `json:"order_count"`
}

// GetSalesReport implements GET /api/v1/reports/sales (docs/07-api-contract.md §13).
func (h *Handler) GetSalesReport(w http.ResponseWriter, r *http.Request) {
	from, to := dateRange(r)
	outletIDs, restricted := outletFilter(r)
	if restricted && len(outletIDs) == 0 {
		respond.JSON(w, http.StatusOK, map[string]any{"data": []dailySales{}})
		return
	}

	query := `
		SELECT sales_date::text, outlet_id, gross_amount, refunded_amount, order_count
		FROM rpt_daily_outlet_sales
		WHERE sales_date BETWEEN $1 AND $2
	`
	args := []any{from, to}
	if restricted {
		query += ` AND outlet_id = ANY($3)`
		args = append(args, outletIDs)
	}
	query += ` ORDER BY sales_date, outlet_id`

	rows, err := h.Pool.Query(r.Context(), query, args...)
	if err != nil {
		h.Logger.Error("sales report failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer rows.Close()

	out := []dailySales{}
	for rows.Next() {
		var d dailySales
		if err := rows.Scan(&d.SalesDate, &d.OutletID, &d.GrossAmount, &d.RefundedAmount, &d.OrderCount); err != nil {
			h.Logger.Error("sales report: scan failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		out = append(out, d)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out, "from": from, "to": to})
}

type orderSummary struct {
	OrderID     string  `json:"order_id"`
	OrderCode   string  `json:"order_code"`
	OutletID    string  `json:"outlet_id"`
	CustomerID  string  `json:"customer_id"`
	Status      string  `json:"status"`
	TotalAmount *int64  `json:"total_amount,omitempty"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

// GetOrderSummaryReport implements GET /api/v1/reports/orders/summary.
func (h *Handler) GetOrderSummaryReport(w http.ResponseWriter, r *http.Request) {
	from, to := dateRange(r)
	outletIDs, restricted := outletFilter(r)
	if restricted && len(outletIDs) == 0 {
		respond.JSON(w, http.StatusOK, map[string]any{"data": []orderSummary{}})
		return
	}

	query := `
		SELECT order_id, order_code, outlet_id, customer_id, status, total_amount, created_at::text, completed_at::text
		FROM rpt_order_summary
		WHERE created_at::date BETWEEN $1 AND $2
	`
	args := []any{from, to}
	argN := 3
	if restricted {
		query += ` AND outlet_id = ANY($3)`
		args = append(args, outletIDs)
		argN = 4
	}
	if v := r.URL.Query().Get("status"); v != "" {
		query += ` AND status = $` + strconv.Itoa(argN)
		args = append(args, v)
	}
	query += ` ORDER BY created_at DESC LIMIT 200`

	rows, err := h.Pool.Query(r.Context(), query, args...)
	if err != nil {
		h.Logger.Error("order summary report failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer rows.Close()

	out := []orderSummary{}
	for rows.Next() {
		var o orderSummary
		if err := rows.Scan(&o.OrderID, &o.OrderCode, &o.OutletID, &o.CustomerID, &o.Status, &o.TotalAmount, &o.CreatedAt, &o.CompletedAt); err != nil {
			h.Logger.Error("order summary report: scan failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		out = append(out, o)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out, "from": from, "to": to})
}

// GetServicePerformanceReport implements GET /api/v1/reports/services/performance.
//
// NOTE(phase-2+): rpt_service_performance is never populated by the
// current event consumers (see events.go's doc comment) — no event
// carries per-service weight/revenue breakdown yet. This returns
// whatever the table has, which today is nothing, rather than
// fabricating numbers from order-level totals.
func (h *Handler) GetServicePerformanceReport(w http.ResponseWriter, r *http.Request) {
	from, to := dateRange(r)
	rows, err := h.Pool.Query(r.Context(), `
		SELECT period_date::text, service_id, total_weight_kg::text, total_revenue
		FROM rpt_service_performance
		WHERE period_date BETWEEN $1 AND $2
		ORDER BY period_date, service_id
	`, from, to)
	if err != nil {
		h.Logger.Error("service performance report failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer rows.Close()

	type row struct {
		PeriodDate    string `json:"period_date"`
		ServiceID     string `json:"service_id"`
		TotalWeightKg string `json:"total_weight_kg"`
		TotalRevenue  int64  `json:"total_revenue"`
	}
	out := []row{}
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.PeriodDate, &rr.ServiceID, &rr.TotalWeightKg, &rr.TotalRevenue); err != nil {
			h.Logger.Error("service performance report: scan failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		out = append(out, rr)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out, "from": from, "to": to})
}

type refundSummary struct {
	PeriodDate   string `json:"period_date"`
	OutletID     string `json:"outlet_id"`
	RefundCount  int    `json:"refund_count"`
	RefundAmount int64  `json:"refund_amount"`
}

// GetRefundsReport implements GET /api/v1/reports/refunds.
func (h *Handler) GetRefundsReport(w http.ResponseWriter, r *http.Request) {
	from, to := dateRange(r)
	outletIDs, restricted := outletFilter(r)
	if restricted && len(outletIDs) == 0 {
		respond.JSON(w, http.StatusOK, map[string]any{"data": []refundSummary{}})
		return
	}

	query := `
		SELECT period_date::text, outlet_id, refund_count, refund_amount
		FROM rpt_refund_summary
		WHERE period_date BETWEEN $1 AND $2
	`
	args := []any{from, to}
	if restricted {
		query += ` AND outlet_id = ANY($3)`
		args = append(args, outletIDs)
	}
	query += ` ORDER BY period_date, outlet_id`

	rows, err := h.Pool.Query(r.Context(), query, args...)
	if err != nil {
		h.Logger.Error("refunds report failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer rows.Close()

	out := []refundSummary{}
	for rows.Next() {
		var ref refundSummary
		if err := rows.Scan(&ref.PeriodDate, &ref.OutletID, &ref.RefundCount, &ref.RefundAmount); err != nil {
			h.Logger.Error("refunds report: scan failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		out = append(out, ref)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out, "from": from, "to": to})
}
