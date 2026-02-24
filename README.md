# Shopify Payment Orchestration Service  
### Multi-Rail Payment Processing (Go + PostgreSQL + Shopify + Green Money + MoneyEU)

This project implements a custom payment orchestration layer between Shopify and multiple external payment processors.

The system supports two distinct payment rails:

- Green Money (ACH / eCheck)
- MoneyEU (Hosted Credit/Debit Card Checkout)

Because neither processor is natively integrated into Shopify’s checkout, all orchestration is handled through:

- Shopify webhooks  
- A Go backend service  
- External processor APIs  
- PostgreSQL for state tracking  
- Background settlement monitoring  

This README documents the architecture, processing flows, and lifecycle handling.

---

## Overview

This service automates the full lifecycle of external payment processing for Shopify orders:

1. Detect eligible Shopify orders  
2. Route the order to the correct payment processor  
3. Generate invoice or hosted checkout session  
4. Track processor status updates (polling or webhook-driven)  
5. Reconcile payment outcome  
6. Mark Shopify order **Paid** when funds are confirmed  
7. Persist full lifecycle state in PostgreSQL  

### Guarantees

- Idempotent processing  
- No duplicate settlement  
- Safe retry handling  
- Event-driven reconciliation  
- Explicit payment state transitions  

---

## System Architecture

### Core Components

| Component | Purpose |
|------------|----------|
| **Shopify** | Order creation + payment state |
| **Go Service** | Webhook handling, API orchestration, state logic |
| **PostgreSQL** | Persistent payment lifecycle tracking |
| **Green Money API** | ACH invoice generation + settlement reporting |
| **MoneyEU API** | Hosted card checkout + webhook-based confirmation |
| **Green Poller** | Background job for ACH settlement monitoring |

---

# Payment Rails

---

# Rail 1: Green Money (ACH / eCheck)

## Workflow

### 1. Customer selects Green Money at checkout

Shopify creates an order using a manual payment method.

---

### 2. Shopify triggers `orders/create`

The Go service:

- Validates payment method  
- Inserts a payment record  
- Calls Green Money `OneTimeInvoice`  
- Stores returned `Invoice_ID`  

Green Money automatically emails the invoice to the customer.

---

### 3. Customer submits bank information

Green generates a `Check_ID` once bank information is entered.

---

### 4. Poller resolves Invoice → Check_ID

The background poller:

- Calls `InvoiceStatus`  
- Stores `Check_ID`  
- Associates debit with Shopify order  

---

### 5. Poller monitors settlement

For each `Check_ID`, the poller calls `CheckStatus`.

| Processed | Rejected | Meaning |
|------------|-----------|-----------|
| False | False | Pending batch |
| True | False | **Cleared** |
| Any | True | **Rejected** |

---

### 6. Shopify marked Paid (ACH only after settlement)

When:

Processed = True  
Rejected = False  

The service:

- Calls Shopify Admin API  
- Marks order as Paid  
- Updates lifecycle state  

Orders are fulfilled only after ACH funds are fully settled.

---

# Rail 2: MoneyEU (Hosted Credit/Debit Card)

MoneyEU uses a hosted checkout model and is fully webhook-driven.

---

## Workflow

### 1. Customer selects Credit/Debit Card

Shopify creates an order using a manual payment method.

---

### 2. Shopify triggers `orders/create`

The Go service:

- Validates payment method  
- Inserts payment record  
- Calls MoneyEU `createOrderExt`  
- Receives secure hosted checkout URL  
- Emails checkout link to customer  

The platform’s checkout is not modified — payment is completed on the processor’s hosted page.

---

### 3. Customer completes card payment

The customer completes payment within MoneyEU’s secure hosted checkout environment.

---

### 4. MoneyEU sends webhook events

MoneyEU posts status updates to:

```/webhooks/moneyeu```

Example lifecycle events:

- Sent
- Completed
- Failed

---

### 5. Webhook processing logic

The service:

- Parses and validates webhook payload  
- Extracts `idOrderExt` (Shopify order ID)  
- Stores raw webhook event  
- Applies lifecycle transition rules  

If status indicates successful capture:

- Ensures idempotency  
- Calls Shopify Admin API  
- Marks order Paid  
- Records reconciliation timestamp  

If status indicates failure:

- Updates status to `failed`  
- Records failure metadata  
- Leaves Shopify order unpaid  

---

## Idempotency & Safety Design

Payment systems must assume:

- Webhooks may be delivered multiple times  
- Events can arrive out of order  
- External APIs may retry  

Safeguards implemented:

- Unique constraints on Shopify order IDs  
- Database-level idempotency checks  
- Safe lifecycle transition rules  
- Duplicate webhook suppression  
- One-time Shopify mark-paid logic  
- Separation of acknowledgement and processing  

The system is resilient to:

- Duplicate order webhooks  
- Processor retry events  
- Slow API responses  
- Partial external failures  

---

## Background Polling (Green Only)

A background job runs at a configurable interval to:

- Query unsettled ACH payments  
- Resolve Invoice → Check_ID  
- Check settlement status  
- Mark Shopify paid after clearance  
- Stop tracking rejected payments  

MoneyEU does not require polling — it is fully webhook-driven.

---

## Testing & Validation

Validated through:

- Live Shopify order creation  
- Duplicate webhook simulations  
- Processor retry testing  
- ACH settlement batching scenarios  
- Card authorization success/failure flows  
- End-to-end Shopify payment reconciliation  
- Idempotency under repeated events  

Both payment rails are currently running end-to-end in production.

---

## Design Principles

This project emphasizes:

- Correctness over speed  
- Explicit payment state modeling  
- Event-driven architecture  
- Clear separation of processor-specific logic  
- Resilience under retry conditions  
- Safe financial reconciliation  
