package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"laundry-platform/shared/authtoken"
	"laundry-platform/shared/respond"
	"laundry-platform/shared/security"
)

// --- Request/response shapes (docs/07-api-contract.md §2, UQ-03) ---

type customerRegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type customerLoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type customerAuthResponse struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	ExpiresIn    int             `json:"expires_in"`
	Customer     customerSummary `json:"customer"`
}

type customerSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CustomerCode string `json:"customer_code"`
}

// coreCustomer mirrors Core's POST /api/v1/customers response
// (docs/07-api-contract.md §4).
type coreCustomer struct {
	ID           string `json:"id"`
	CustomerCode string `json:"customer_code"`
	IsGuest      bool   `json:"is_guest"`
}

// Register implements POST /api/v1/customer-auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req customerRegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(req.Email)

	var details []respond.ErrorDetail
	if req.Name == "" {
		details = append(details, respond.ErrorDetail{Field: "name", Issue: "required"})
	}
	if req.Phone == "" {
		details = append(details, respond.ErrorDetail{Field: "phone", Issue: "required"})
	}
	if len(req.Password) < 8 {
		details = append(details, respond.ErrorDetail{Field: "password", Issue: "minimum 8 characters"})
	}
	if len(details) > 0 {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "One or more fields are invalid.", details...)
		return
	}

	ctx := r.Context()

	// 1. Create-or-fetch the Core customer profile (upsert-by-phone).
	customer, err := h.upsertCoreCustomer(ctx, req.Name, req.Phone, req.Email)
	if err != nil {
		h.Logger.Error("register: upsert core customer failed", "error", err)
		respond.Error(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Could not reach the customer profile service.")
		return
	}

	// 2. Hash the password and insert the credential row.
	hash, err := security.HashPassword(req.Password)
	if err != nil {
		h.Logger.Error("register: hash password failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	accountID := uuid.New()
	var email *string
	if req.Email != "" {
		email = &req.Email
	}

	_, err = h.Pool.Exec(ctx, `
		INSERT INTO customer_accounts (id, customer_id, email, phone, password_hash)
		VALUES ($1, $2, $3, $4, $5)
	`, accountID, customer.ID, email, req.Phone, hash)
	if err != nil {
		if isUniqueViolation(err) {
			respond.Error(w, r, http.StatusConflict, "ACCOUNT_ALREADY_EXISTS", "An account with this email or phone already exists.")
			return
		}
		h.Logger.Error("register: insert customer_account failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	// 3. Link the two records on Core's side (is_guest=false, identity_account_id set).
	if err := h.linkCoreCustomerIdentity(ctx, customer.ID, accountID.String()); err != nil {
		h.Logger.Error("register: link identity failed", "error", err)
		// Non-fatal for the caller: the login itself succeeded and the
		// customer can still authenticate; the link can be repaired by a
		// retry/reconciliation job. We still surface it in logs loudly.
	}

	h.issueCustomerSession(w, r, accountID.String(), customer, http.StatusCreated)
}

// Login implements POST /api/v1/customer-auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req customerLoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Identifier = strings.TrimSpace(req.Identifier)
	if req.Identifier == "" || req.Password == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Identifier and password are required.")
		return
	}

	ctx := r.Context()
	var (
		accountID    uuid.UUID
		customerID   uuid.UUID
		passwordHash string
		isActive     bool
	)
	err := h.Pool.QueryRow(ctx, `
		SELECT id, customer_id, password_hash, is_active
		FROM customer_accounts
		WHERE email = $1 OR phone = $1
	`, req.Identifier).Scan(&accountID, &customerID, &passwordHash, &isActive)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email/phone or password is incorrect.")
		return
	}
	if err != nil {
		h.Logger.Error("login: query customer_account failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}
	if !isActive {
		respond.Error(w, r, http.StatusLocked, "ACCOUNT_INACTIVE", "This account has been deactivated.")
		return
	}

	ok, err := security.VerifyPassword(req.Password, passwordHash)
	if err != nil || !ok {
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email/phone or password is incorrect.")
		return
	}

	// Fetch the customer's current profile from Core for the response body.
	customer, err := h.fetchCoreCustomer(ctx, customerID.String())
	if err != nil {
		h.Logger.Error("login: fetch core customer failed", "error", err)
		respond.Error(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Could not reach the customer profile service.")
		return
	}

	if _, err := h.Pool.Exec(ctx, `UPDATE customer_accounts SET last_login_at = now() WHERE id = $1`, accountID); err != nil {
		h.Logger.Warn("login: update last_login_at failed", "error", err)
	}

	h.issueCustomerSession(w, r, accountID.String(), customer, http.StatusOK)
}

// Me implements GET /api/v1/customer-auth/me.
func (h *Handler) CustomerMe(w http.ResponseWriter, r *http.Request, principalID string) {
	ctx := r.Context()
	var customerID uuid.UUID
	err := h.Pool.QueryRow(ctx, `SELECT customer_id FROM customer_accounts WHERE id = $1`, principalID).Scan(&customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Account not found.")
		return
	}
	if err != nil {
		h.Logger.Error("me: query failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	customer, err := h.fetchCoreCustomer(ctx, customerID.String())
	if err != nil {
		respond.Error(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Could not reach the customer profile service.")
		return
	}
	respond.JSON(w, http.StatusOK, customer)
}

// --- Session/token issuance shared by register & login ---

func (h *Handler) issueCustomerSession(w http.ResponseWriter, r *http.Request, accountID string, customer coreCustomer, status int) {
	accessToken, _, err := authtoken.Issue(h.Cfg.JWTSigningKey, accountID, authtoken.PrincipalCustomer, nil, nil, h.Cfg.AccessTokenTTL)
	if err != nil {
		h.Logger.Error("issue access token failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	refreshToken := uuid.NewString() + uuid.NewString()
	hash := hashToken(refreshToken)

	_, err = h.Pool.Exec(r.Context(), `
		INSERT INTO customer_sessions (customer_account_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, accountID, hash, r.UserAgent(), clientIP(r), time.Now().Add(h.Cfg.RefreshTTL))
	if err != nil {
		h.Logger.Error("issue session: insert customer_session failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	respond.JSON(w, status, customerAuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.Cfg.AccessTokenTTL.Seconds()),
		Customer: customerSummary{
			ID:           customer.ID,
			Name:         "", // Core doesn't echo name back today; left blank rather than a second round-trip.
			CustomerCode: customer.CustomerCode,
		},
	})
}

// --- Core Service integration (synchronous, docs/03-system-architecture.md §2) ---

func (h *Handler) upsertCoreCustomer(ctx context.Context, name, phone, email string) (coreCustomer, error) {
	body, _ := json.Marshal(map[string]string{"name": name, "phone": phone, "email": email})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.Cfg.CoreServiceURL+"/api/v1/customers", bytes.NewReader(body))
	if err != nil {
		return coreCustomer{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := h.HTTP.Do(req)
	if err != nil {
		return coreCustomer{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return coreCustomer{}, fmt.Errorf("core returned status %d", res.StatusCode)
	}

	var out coreCustomer
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return coreCustomer{}, err
	}
	return out, nil
}

func (h *Handler) fetchCoreCustomer(ctx context.Context, customerID string) (coreCustomer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.Cfg.CoreServiceURL+"/internal/customers/"+customerID, nil)
	if err != nil {
		return coreCustomer{}, err
	}
	res, err := h.HTTP.Do(req)
	if err != nil {
		return coreCustomer{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return coreCustomer{}, fmt.Errorf("core returned status %d", res.StatusCode)
	}
	var out coreCustomer
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return coreCustomer{}, err
	}
	return out, nil
}

func (h *Handler) linkCoreCustomerIdentity(ctx context.Context, customerID, identityAccountID string) error {
	body, _ := json.Marshal(map[string]string{"identity_account_id": identityAccountID})
	url := fmt.Sprintf("%s/internal/customers/%s/link-identity", h.Cfg.CoreServiceURL, customerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := h.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("core returned status %d", res.StatusCode)
	}
	return nil
}

// --- helpers ---

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.Split(fwd, ",")[0]
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
