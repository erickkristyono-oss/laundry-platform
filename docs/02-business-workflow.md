# 02 — Business Workflow

References: `01-business-rules.md` (BR-xx), `10-state-machines.md` for the formal order/refund state machines.

---

## 1. End-to-End Order Workflow (Happy Path)

```mermaid
flowchart TD
    A[Order created<br/>WEBSITE / POS / WHATSAPP] --> B{Outlet selection}
    B -->|Manual| C[Customer/staff picks outlet]
    B -->|Automatic| D[System recommends outlet<br/>via coverage rules]
    C --> E[Order status: CREATED]
    D --> E
    E --> F[Order status: RECEIVED<br/>item physically at outlet]
    F --> G[Order status: WEIGHING<br/>staff records actual weight per item]
    G --> H[Subtotal & total computed<br/>from price snapshot x actual weight]
    H --> I[Payment collected<br/>full amount, single transaction]
    I -->|payment.paid event| J[Order status: WASHING]
    J --> K[Order status: DRYING]
    K --> L[Order status: IRONING]
    L --> M[Order status: PACKING]
    M --> N[Order status: READY]
    N --> O{Fulfillment method}
    O -->|Customer pickup| P[Order status: PICKED_UP]
    O -->|Delivery| Q[Delivery request dispatched]
    Q --> R[Order status: DELIVERED]
    P --> S[Order status: COMPLETED]
    R --> S[Order status: COMPLETED]
```

Notes:
- The transition `WEIGHING → WASHING` is **gated on payment** (BR-02, BR-06, BR-07). It is not purely a staff action; Core Service will not allow it until Payment Service confirms `payment.paid` for the order — with one narrow, audited exception: `OWNER`/`SUPER_ADMIN` may force it via the administrative override (BR-26, `10-state-machines.md` §1.3), which still requires `payment_status = PAID` before the order can later reach `COMPLETED`.
- `ON_HOLD` and `CANCELLED` are reachable from most intermediate states — see `10-state-machines.md` for the exhaustive transition table.

---

## 2. Order Creation Workflow (by channel)

```mermaid
flowchart LR
    subgraph Website
        W1[Customer browses services] --> W2[Customer selects items + estimated weight]
        W2 --> W3[Customer selects/accepts outlet]
        W3 --> W4[Guest or login]
        W4 --> W5[POST /orders]
    end
    subgraph POS
        P1[Staff opens new order] --> P2[Staff selects/creates customer]
        P2 --> P3[Staff selects services]
        P3 --> P4[POST /orders]
    end
    subgraph WhatsApp
        WA1[Customer messages outlet] --> WA2[Staff manually enters order<br/>on customer's behalf]
        WA2 --> WA3[POST /orders with source=WHATSAPP]
    end
    W5 --> ORD[Order Service: order.created]
    P4 --> ORD
    WA3 --> ORD
```

---

## 3. Weighing & Payment Workflow

```mermaid
sequenceDiagram
    participant Staff as Outlet Staff
    participant Core as Core Service
    participant Pay as Payment Service
    participant Bus as RabbitMQ

    Staff->>Core: PUT /orders/{id}/items/{itemId}/weigh (actual_weight_kg)
    Core->>Core: compute subtotal = actual_weight_kg * unit_price_snapshot
    Core->>Bus: outbox: order.weighed
    Core-->>Staff: 200 OK (order still WEIGHING)
    Staff->>Core: POST /orders/{id}/finalize-weighing
    Core->>Core: lock item weights, compute order.total_amount
    Core-->>Staff: 200 OK
    Staff->>Pay: POST /payments (order_id, amount = total_amount, method)
    Pay->>Pay: validate amount == order.total_amount (via Core API/cached snapshot)
    Pay->>Pay: persist payment (status=PENDING -> PAID)
    Pay->>Bus: outbox: payment.paid
    Bus->>Core: payment.paid consumed
    Core->>Core: transition order WEIGHING -> WASHING
    Core->>Core: freeze weight/price/subtotal/total (BR-05)
    Core->>Bus: outbox: order.status_changed
```

---

## 4. Pickup Workflow (manual)

```mermaid
flowchart TD
    A[Customer requests pickup<br/>at order creation or later] --> B[pickup_requests row created<br/>status=REQUESTED]
    B --> C[Staff schedules pickup<br/>assigns time window]
    C --> D[Staff manually performs pickup]
    D --> E[Staff marks pickup COMPLETED]
    E --> F[order.status -> RECEIVED]
    F --> G[pickup.completed event emitted]
```

No driver app or GPS: scheduling and completion are manual staff actions recorded via API (`13-...` not applicable; see `07-api-contract.md` §Pickup).

---

## 5. Delivery Workflow (manual)

```mermaid
flowchart TD
    A[Order reaches READY] --> B[delivery_requests row created<br/>status=REQUESTED]
    B --> C[Staff schedules delivery<br/>assigns time window]
    C --> D[Staff manually performs delivery]
    D --> E[Staff marks delivery COMPLETED]
    E --> F[order.status -> DELIVERED]
    F --> G[delivery.completed event emitted]
```

---

## 6. Outlet Transfer Workflow

```mermaid
flowchart TD
    A[Staff/Owner initiates transfer] --> B{Order status eligible?<br/>Not PICKED_UP/DELIVERED/COMPLETED/CANCELLED}
    B -->|No| X[Reject: 409 INVALID_STATE]
    B -->|Yes| C[Record order_transfers row:<br/>from_outlet_id, to_outlet_id, reason, performed_by, timestamp]
    C --> D[Update orders.current_outlet_id]
    D --> E[order.transferred event emitted]
    E --> F[Order status unchanged by transfer itself]
```

Order status is **not** affected by an outlet transfer (see BR-10/BR-11 and `10-state-machines.md`). Transfer is an orthogonal audit-tracked relocation, not a status transition.

---

## 7. Cancellation / Refund Workflow

```mermaid
flowchart TD
    A[Customer requests cancellation/refund] --> B[POST /refunds — refund status REQUESTED]
    B --> C{Backend eligibility check<br/>order status + payment status}
    C -->|Not eligible| X[409 REFUND_NOT_ELIGIBLE]
    C -->|Eligible| D[OUTLET_ADMIN/OWNER reviews]
    D --> E{Approve?}
    E -->|No| F[refund status REJECTED]
    E -->|Yes| G[refund status APPROVED]
    G --> H[refund status PROCESSING]
    H --> I{Processing result}
    I -->|Success| J[refund status COMPLETED<br/>payment.refunded event]
    I -->|Failure| K[refund status FAILED<br/>retry or manual intervention]
```

Full detail (roles, eligibility rules, order-status interaction) in `10-state-machines.md` §2 and `15` cross-reference.

---

## 8. Workflow-to-Service Mapping

| Workflow step | Owning service | Key event(s) emitted |
|---|---|---|
| Order creation | Core | `order.created` |
| Weighing | Core | `order.weighed` |
| Payment | Payment | `payment.created`, `payment.paid`, `payment.failed` |
| Status progression | Core | `order.status_changed` |
| Pickup | Core | `pickup.requested`, `pickup.completed` |
| Delivery | Core | `delivery.requested`, `delivery.completed` |
| Outlet transfer | Core | `order.transferred` |
| Refund | Payment | `payment.refunded` |
| Notifications (any of the above) | Notification (consumer) | — (consumes events, no domain events of its own beyond delivery receipts) |
| Reporting | Reporting (consumer) | — (builds read models from all of the above) |
