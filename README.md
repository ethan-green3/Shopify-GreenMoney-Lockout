# Shopify–Green Money Integration  
### Payment Processing Pipeline (Go + PostgreSQL + Shopify + Green Money)

This project implements a **custom eDebit payment integration** between Shopify and Green Money for Lockout Supplements.  
Because Green Money is *not* a Shopify-approved payment provider, all logic is handled manually through:

- Shopify webhooks  
- A Go backend service  
- Green Money’s eDebit APIs  
- A local PostgreSQL database  
- A background poller to monitor ACH settlement  

This README explains the full architecture, data flow, endpoints, database model, and operational lifecycle.

---

## Overview

This service automates the complete process of handling Green Money eCheck payments for Shopify orders:

1. Detect Shopify orders using Green Money  
2. Create a Green Money invoice via API  
3. Email invoice to customer (Green sends automatically)  
4. Track customer submission of bank information  
5. Poll Green Money until the ACH debit clears  
6. Mark the Shopify order as **Paid** once settled  
7. Record all statuses in a local PostgreSQL database  

This system ensures that orders are **only fulfilled once funds have fully settled**.

---

## System Architecture

### Components

| Component | Purpose |
|----------|---------|
| **Shopify** | Customer checkout + order creation events |
| **Go server** | API orchestration, polling, database operations |
| **PostgreSQL** | Tracks invoices, Check_IDs, and statuses |
| **Green Money API** | Generates invoices, runs ACH debits, exposes settlement status |
| **Poller** | Background job to check ACH processing status |

---

## End-to-End Workflow

### **1. Customer places order using Green Money**
The customer checks out normally on Shopify and selects **Green Money** as the payment method.

---

### **2. Shopify triggers the webhook**
Shopify sends an `orders/create` webhook payload to the Go service.

The server:

- Reads the payment method  
- If not Green Money → ignores the event  
- If Green Money:  
  - Extracts customer info  
  - Extracts order amount + order ID  
  - Inserts a row into `green_payments`  

---

### **3. Server calls `OneTimeInvoice`**
Using the customer/order details, the service calls:

POST https://cpsandbox.com/echeck.asmx/OneTimeInvoice


Green Money generates an invoice and **emails it to the customer**.

`Invoice_ID` is stored in the database.

---

### **4. Customer submits bank information**
The customer receives the invoice by email and completes the payment form.

Green Money then generates a **Check_ID** (debit).

---

### **5. Poller matches Invoice_ID → Check_ID**
Every X minutes the poller:

1. Calls `InvoiceStatus(Invoice_ID)`  
2. Stores the returned `Check_ID` in the DB  
3. Associates Green Money’s debit with the Shopify order  

---

### **6. Poller monitors ACH settlement via `CheckStatus`**
For each known `Check_ID`, the poller calls:

CheckStatus?Check_ID=xxxxxx


Interpretation:

| Processed | Rejected | Meaning |
|----------|----------|---------|
| False | False | Pending batch (not yet settled) |
| True | False | **Cleared — funds settled** |
| Any | True | **Rejected — NSF or invalid bank info** |

---

### **7. Mark Shopify order as Paid**
When Green Money returns:

Processed = "True"
Rejected = "False"


The server:

- Calls Shopify Admin API → marks order **Paid**
- Updates DB:
  - `current_status = 'cleared'`
  - `is_cleared = true`
  - `shopify_marked_paid_at = NOW()`

Order is now ready for fulfillment.

---

### **8. Rejected Payments**
If:
Rejected = "True"

Then the server:

- Marks `current_status = 'rejected'`
- Sets `rejected_at = NOW()`
- Excludes the row from future polling  

Staff can manually review and follow up with the customer.

---

## Database Schema

### `green_payments` table

| Column | Type | Description |
|--------|------|-------------|
| id | serial | Primary key |
| shopify_order_id | text | Shopify numeric order ID |
| shopify_order_name | text | Shopify order name (#xxxx) |
| amount | numeric | Order total |
| currency | text | ISO currency code |
| invoice_id | text | Green Money invoice ID |
| green_check_id | text | Green Money check/debit ID |
| current_status | text | `invoice_sent`, `cleared`, `rejected`, or `shopify_payment_error` |
| is_cleared | boolean | Whether ACH settled successfully |
| shopify_marked_paid_at | timestamptz | Timestamp when Shopify marked paid |
| rejected_at | timestamptz | Timestamp when debit became rejected |
| created_at | timestamptz | Row creation timestamp |
| last_status_at | timestamptz | Last status check timestamp |
| updated_at | timestamptz | Last DB update timestamp |

---

## Poller Logic Summary

Runs every X minutes:

1. Load rows where:
   `current_status = 'invoice_sent'
AND is_cleared = false
AND shopify_marked_paid_at IS NULL`
2. If no Check_ID → call `InvoiceStatus`  
3. If Check_ID exists → call `CheckStatus`  
4. If cleared → mark Shopify paid  
5. If rejected → mark DB rejected  

---

## Testing

Validated using:

- Shopify order with Green Money payment  
- Shopfiy → webhook → DB insert  
- Invoice email sent successfully  
- InvoiceStatus returning Check_ID  
- CheckStatus returning Processed after batching  
- Poller marking Shopify paid on settlement  
- Handling of canceled Shopify orders  
- Handling of rejected debits  

---
