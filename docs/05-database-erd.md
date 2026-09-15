# 05 — Database ERD

Logical database-per-service. All cross-service relationships shown as dashed/no-FK "reference" notes, never actual foreign keys. Full column-level spec is in `06-database-schema.md`; this document is the relationship map.

## 1. Database Boundary Diagram

```mermaid
flowchart TB
    subgraph identity_db
        U[users] --- R[roles]
        U --- UR[user_roles]
        R --- UR
        R --- RP[role_permissions]
        P[permissions] --- RP
        U --- UO[user_outlets]
        U --- S[sessions]
        OBX1[outbox_events]
    end
    subgraph core_db
        OUT[outlets] --- OCA[outlet_coverage_areas]
        CUS[customers] --- CADDR[customer_addresses]
        SVC[services] --- SP[service_prices]
        ORD[orders] --- OI[order_items]
        ORD --- OSH[order_status_history]
        ORD --- OT[order_transfers]
        ORD --- PR[pickup_requests]
        ORD --- DR[delivery_requests]
        OBX2[outbox_events]
    end
    subgraph payment_db
        PAY[payments] --- PT[payment_transactions]
        PAY --- REF[refunds]
        OBX3[outbox_events]
    end
    subgraph notification_db
        NOT[notifications] --- ND[notification_deliveries]
        OBX4[outbox_events]
    end
    subgraph reporting_db
        RM1[rpt_daily_outlet_sales]
        RM2[rpt_order_summary]
        RM3[rpt_service_performance]
        RM4[rpt_refund_summary]
        INBOX[projection_checkpoints]
    end

    U -. user_id reference .-> ORD
    CUS -. customer_id reference .-> ORD
    OUT -. outlet_id reference .-> ORD
    ORD -. order_id reference .-> PAY
    CUS -. customer_id reference .-> PAY
```

---

## 2. Identity ERD

```mermaid
erDiagram
    USERS ||--o{ USER_ROLES : has
    ROLES ||--o{ USER_ROLES : assigned_to
    ROLES ||--o{ ROLE_PERMISSIONS : has
    PERMISSIONS ||--o{ ROLE_PERMISSIONS : granted_in
    USERS ||--o{ USER_OUTLETS : scoped_to
    USERS ||--o{ SESSIONS : owns

    USERS {
        uuid id PK
        string email
        string phone
        string password_hash
        bool is_active
        timestamptz created_at
    }
    ROLES {
        uuid id PK
        string code
        string name
    }
    PERMISSIONS {
        uuid id PK
        string code
        string resource
        string action
    }
    USER_ROLES {
        uuid user_id FK
        uuid role_id FK
    }
    ROLE_PERMISSIONS {
        uuid role_id FK
        uuid permission_id FK
    }
    USER_OUTLETS {
        uuid user_id FK
        uuid outlet_id "reference only, no FK - outlet lives in core_db"
    }
    SESSIONS {
        uuid id PK
        uuid user_id FK
        string refresh_token_hash
        timestamptz expires_at
    }
```

---

## 3. Core ERD

```mermaid
erDiagram
    OUTLETS ||--o{ OUTLET_COVERAGE_AREAS : covers
    CUSTOMERS ||--o{ CUSTOMER_ADDRESSES : has
    SERVICES ||--o{ SERVICE_PRICES : priced_by
    OUTLETS ||--o{ SERVICE_PRICES : "optionally scopes (future)"
    CUSTOMERS ||--o{ ORDERS : places
    OUTLETS ||--o{ ORDERS : "current outlet"
    ORDERS ||--|{ ORDER_ITEMS : contains
    SERVICES ||--o{ ORDER_ITEMS : priced_from
    ORDERS ||--o{ ORDER_STATUS_HISTORY : logs
    ORDERS ||--o{ ORDER_TRANSFERS : logs
    ORDERS ||--o| PICKUP_REQUESTS : has
    ORDERS ||--o| DELIVERY_REQUESTS : has

    OUTLETS {
        uuid id PK
        string code
        string name
    }
    OUTLET_COVERAGE_AREAS {
        uuid id PK
        uuid outlet_id FK
        string area_label
        string postal_code
    }
    CUSTOMERS {
        uuid id PK
        string customer_code
        string name
        string phone
        bool is_guest
        uuid identity_user_id "reference only, nullable, no FK"
    }
    CUSTOMER_ADDRESSES {
        uuid id PK
        uuid customer_id FK
        string label
        text address_line
    }
    SERVICES {
        uuid id PK
        string code
        string name
        string unit
    }
    SERVICE_PRICES {
        uuid id PK
        uuid service_id FK
        uuid outlet_id "nullable = global"
        bigint price_per_unit
        timestamptz effective_from
        timestamptz effective_to
    }
    ORDERS {
        uuid id PK
        string order_code
        uuid customer_id FK
        uuid current_outlet_id FK
        string source
        string fulfillment_type
        string status
        string payment_status "separate state machine, projected from payment_db"
        bigint subtotal_amount
        bigint discount_amount "conditional, default 0"
        bigint tax_amount "conditional, default 0"
        bigint pickup_fee "conditional, default 0"
        bigint delivery_fee "conditional, default 0"
        bigint total_amount
        timestamptz created_at
    }
    ORDER_ITEMS {
        uuid id PK
        uuid order_id FK
        uuid service_id FK
        numeric estimated_weight_kg
        numeric actual_weight_kg
        bigint unit_price_snapshot
        bigint subtotal
    }
    ORDER_STATUS_HISTORY {
        uuid id PK
        uuid order_id FK
        string from_status
        string to_status
        uuid performed_by "reference only, no FK"
        timestamptz created_at
    }
    ORDER_TRANSFERS {
        uuid id PK
        uuid order_id FK
        uuid from_outlet_id FK
        uuid to_outlet_id FK
        string reason
        uuid performed_by "reference only, no FK"
        timestamptz created_at
    }
    PICKUP_REQUESTS {
        uuid id PK
        uuid order_id FK
        string status
        timestamptz scheduled_at
        timestamptz completed_at
    }
    DELIVERY_REQUESTS {
        uuid id PK
        uuid order_id FK
        string status
        timestamptz scheduled_at
        timestamptz completed_at
    }
```

---

## 4. Payment ERD

```mermaid
erDiagram
    PAYMENTS ||--o{ PAYMENT_TRANSACTIONS : logs
    PAYMENTS ||--o{ REFUNDS : may_have

    PAYMENTS {
        uuid id PK
        string payment_code
        uuid order_id "reference only, no FK"
        uuid customer_id "reference only, no FK"
        bigint amount
        string status
        string method
        timestamptz created_at
    }
    PAYMENT_TRANSACTIONS {
        uuid id PK
        uuid payment_id FK
        string transaction_type
        bigint amount
        string status
        jsonb gateway_response
        timestamptz created_at
    }
    REFUNDS {
        uuid id PK
        uuid payment_id FK
        uuid order_id "reference only, no FK"
        bigint amount
        string status
        string reason
        uuid requested_by "reference only, no FK"
        uuid approved_by "reference only, no FK"
        timestamptz created_at
    }
```

---

## 5. Notification ERD

```mermaid
erDiagram
    NOTIFICATIONS ||--o{ NOTIFICATION_DELIVERIES : has

    NOTIFICATIONS {
        uuid id PK
        string type
        uuid recipient_customer_id "reference only, no FK"
        uuid recipient_user_id "reference only, no FK"
        jsonb payload
        timestamptz created_at
    }
    NOTIFICATION_DELIVERIES {
        uuid id PK
        uuid notification_id FK
        string channel
        string status
        int attempt_count
        timestamptz last_attempt_at
    }
```

---

## 6. Reporting ERD (Read Models)

```mermaid
erDiagram
    RPT_DAILY_OUTLET_SALES {
        date sales_date PK
        uuid outlet_id PK
        bigint gross_amount
        bigint refunded_amount
        int order_count
    }
    RPT_ORDER_SUMMARY {
        uuid order_id PK
        uuid outlet_id
        string status
        bigint total_amount
        timestamptz created_at
        timestamptz completed_at
    }
    RPT_SERVICE_PERFORMANCE {
        date period_date PK
        uuid service_id PK
        numeric total_weight_kg
        bigint total_revenue
    }
    RPT_REFUND_SUMMARY {
        date period_date PK
        uuid outlet_id PK
        int refund_count
        bigint refund_amount
    }
    PROJECTION_CHECKPOINTS {
        string projection_name PK
        string last_event_id
        timestamptz updated_at
    }
```

Reporting tables are rebuildable at any time by replaying events from the earliest retained offset; `projection_checkpoints` tracks per-projection idempotency (see `08-event-contract.md` §4).

## 7. Cross-Service Reference Rule

Every `uuid ... "reference only, no FK"` field above is validated at the application layer (existence checked via the owning service's synchronous API when correctness matters at write time, e.g., Payment validating `order_id` against Core) and/or trusted from the event payload that originated it. No migration in any service may add a `REFERENCES` clause pointing at a table in another service's database.
