# DiasporaConnect API Documentation

## Overview

DiasporaConnect is a cross-border remittance API that enables USDT transfers on the Solana blockchain, backed by an escrow smart contract, with an off-chain mobile-money payout layer.

**Base URL (development):** `http://localhost:8080`  
**Base URL (production):** `https://<your-deployment>.replit.app`

All request and response bodies are **JSON** (`Content-Type: application/json`).  
Timestamps are **ISO 8601 / RFC 3339** UTC strings.

---

## Authentication

Protected endpoints require a **Bearer token** in the `Authorization` header:

```
Authorization: Bearer <jwt_token>
```

Tokens are issued by `POST /api/login` and expire after **24 hours**.

---

## Endpoints

### Public Endpoints

---

#### `GET /api/health`

Returns server health status.

**Response `200 OK`**
```json
{ "status": "ok" }
```

---

#### `POST /api/register`

Creates a new user account. Generates a Solana keypair automatically.

**Request body**
```json
{
  "phone_number": "+22670123456",
  "name": "Alice Traoré",
  "password": "myS3cureP@ss"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `phone_number` | string | ✓ | E.164 format recommended |
| `name` | string | ✓ | Display name |
| `password` | string | ✓ | Min 8 characters recommended |

**Response `201 Created`**
```json
{
  "message": "account created successfully",
  "user_id": 42,
  "solana_pubkey": "HmTz9wFjGcXyHJ5bYzUiDP3D8VkgAoNYqZtXpLFbnS4e"
}
```

**Error responses**

| Status | Meaning |
|---|---|
| 400 | Missing or invalid fields |
| 409 | Phone number already registered |

---

#### `POST /api/login`

Authenticates a user and returns a JWT.

**Request body**
```json
{
  "phone_number": "+22670123456",
  "password": "myS3cureP@ss"
}
```

**Response `200 OK`**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": 42
}
```

**Error responses**

| Status | Meaning |
|---|---|
| 400 | Missing fields |
| 401 | Invalid credentials |

---

#### `POST /api/verify-otp`

Verifies a one-time password sent after registration.

**Request body**
```json
{
  "phone_number": "+22670123456",
  "otp": "123456"
}
```

**Response `200 OK`**
```json
{ "message": "OTP verified successfully" }
```

**Error responses**

| Status | Meaning |
|---|---|
| 400 | Missing fields |
| 401 | Invalid or expired OTP |

---

### Authenticated Endpoints

All endpoints below require `Authorization: Bearer <token>`.

---

#### `GET /api/account`

Returns the authenticated user's profile.

**Response `200 OK`**
```json
{
  "id": 42,
  "name": "Alice Traoré",
  "phone_number": "+22670123456",
  "solana_pubkey": "HmTz9wFjGcXyHJ5bYzUiDP3D8VkgAoNYqZtXpLFbnS4e",
  "created_at": "2026-01-15T10:30:00Z"
}
```

**Error responses**

| Status | Meaning |
|---|---|
| 401 | Missing or invalid token |
| 404 | User not found |

---

#### `GET /api/balance`

Returns the authenticated user's USDT balance on Solana.

**Response `200 OK`**
```json
{
  "solana_pubkey": "HmTz9wFjGcXyHJ5bYzUiDP3D8VkgAoNYqZtXpLFbnS4e",
  "balance_usdt": 250.50
}
```

When the Solana node is unreachable, balance returns `0.0` with a `warning` field:

```json
{
  "solana_pubkey": "HmTz9wFjGcXyHJ5bYzUiDP3D8VkgAoNYqZtXpLFbnS4e",
  "balance_usdt": 0.0,
  "warning": "could not fetch live balance: ..."
}
```

---

#### `GET /api/transfers`

Returns transfer history. Results include transfers where the user is either the sender or recipient.

**Query parameters**

| Parameter | Values | Default | Description |
|---|---|---|---|
| `direction` | `all` \| `sent` \| `received` | `all` | Filter by direction |
| `status` | `pending` \| `claimed` \| `refunded` | _(all)_ | Filter by status |

**Examples**
```
GET /api/transfers
GET /api/transfers?direction=sent
GET /api/transfers?direction=received&status=pending
```

**Response `200 OK`**
```json
{
  "transfers": [
    {
      "ID": 1,
      "SenderID": 42,
      "RecipientID": 7,
      "AmountUSDT": 99.0,
      "FeesUSDT": 1.0,
      "Status": "pending",
      "SolanaTxHash": "3xG9f...",
      "CreatedAt": "2026-05-10T12:00:00Z",
      "ExpiresAt": "2026-05-17T12:00:00Z",
      "ClaimedAt": null
    }
  ]
}
```

---

#### `GET /api/transfers/detail?id={transfer_id}`

Returns a single transfer by ID. Only the sender or recipient may access it.

**Query parameters**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | integer | ✓ | Transfer ID |

**Response `200 OK`** – same shape as individual item in `GET /api/transfers`.

**Error responses**

| Status | Meaning |
|---|---|
| 400 | Missing or invalid `id` |
| 403 | Authenticated user is not the sender or recipient |
| 404 | Transfer not found |

---

#### `POST /api/transfer`

Initiates a USDT escrow transfer on the Solana blockchain.

A **1% fee** is deducted from the sent amount.  
The escrow expires after **7 days**; after that the sender may request a refund.

**Request body**
```json
{
  "recipient_phone": "+22670987654",
  "amount_usdt": 100.0
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `recipient_phone` | string | ✓ | Must be a registered DiasporaConnect user |
| `amount_usdt` | number | ✓ | Gross amount including fee (> 0) |

**Response `201 Created`**
```json
{
  "transfer_id": 1,
  "tx_hash": "3xG9fHmP...",
  "amount_usdt": 99.0,
  "fees_usdt": 1.0,
  "recipient_phone": "+22670987654",
  "status": "pending",
  "expires_at": "2026-05-22T12:00:00Z"
}
```

**Error responses**

| Status | Meaning |
|---|---|
| 400 | Invalid body / amount ≤ 0 / self-transfer |
| 404 | Recipient phone number not registered |
| 500 | Blockchain or database error |

---

#### `POST /api/claim`

Claims a pending escrow transfer. Only the **recipient** of the transfer may call this.

**Request body**
```json
{
  "transfer_id": 1
}
```

**Response `200 OK`**
```json
{
  "status": "claimed",
  "tx_hash": "3xG9fHmP...",
  "amount_usdt": 99.0,
  "claimed_at": "2026-05-11T09:15:00Z"
}
```

**Error responses**

| Status | Meaning |
|---|---|
| 400 | Invalid body / transfer not pending / transfer expired |
| 403 | Caller is not the recipient |
| 404 | Transfer not found |
| 500 | Blockchain or database error |

---

#### `POST /api/refund`

Refunds an expired pending escrow to the original sender.  
The transfer must be **past its 7-day expiry** and still in `pending` status.

**Request body**
```json
{
  "transfer_id": 1
}
```

**Response `200 OK`**
```json
{
  "status": "refunded",
  "tx_hash": "3xG9fHmP...",
  "transfer_id": "1"
}
```

**Error responses**

| Status | Meaning |
|---|---|
| 400 | Transfer not pending / not yet expired (includes remaining time) |
| 403 | Caller is not the sender |
| 404 | Transfer not found |
| 500 | Blockchain or database error |

**Example error when not yet expired**
```json
{
  "error": "transfer has not expired yet; refund available in 6d23h"
}
```

---

#### `POST /api/withdraw`

Debits the user's USDT balance and triggers a mobile-money payout (MTN, Moov, etc.).

**Request body**
```json
{
  "amount_usdt": 50.0,
  "provider": "mtn"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `amount_usdt` | number | ✓ | Must be > 0 |
| `provider` | string | | `"mtn"` (default) or `"moov"` |

**Response `200 OK`**
```json
{
  "message": "withdrawal initiated",
  "mobile_tx_id": "MM_1001",
  "amount_usdt": 50.0,
  "provider": "mtn"
}
```

**Error responses**

| Status | Meaning |
|---|---|
| 400 | Invalid body / amount ≤ 0 |
| 422 | Insufficient balance |
| 500 | Mobile money API error |

---

## Error Format

All errors return a consistent JSON object:

```json
{ "error": "human-readable error message" }
```

---

## Transfer Lifecycle

```
                    ┌──────────┐
                    │ PENDING  │
                    └────┬─────┘
           ┌─────────────┴──────────────┐
     claim │ (within 7 days)   refund  │ (after 7 days, by sender)
           ▼                           ▼
      ┌─────────┐                ┌──────────┐
      │ CLAIMED │                │ REFUNDED │
      └─────────┘                └──────────┘
```

---

## Rate Limiting

The API enforces a global rate limit of **10 requests/second** with a burst of 20.  
Exceeding the limit returns `429 Too Many Requests`.

---

## Solana Integration Notes

- USDT amounts use **6 decimal places** (1 USDT = 1,000,000 lamports in the program).
- Fees are calculated as **1%** of the gross amount, routed to the treasury token account.
- The escrow PDA seeds are: `["diaspora-escrow", sender_pubkey, recipient_pubkey, nonce_le_bytes]`.
- The vault PDA seeds are: `["diaspora-vault", sender_pubkey, recipient_pubkey, nonce_le_bytes]`.

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `DATABASE_URL` | PostgreSQL connection string | _(Replit managed)_ |
| `JWT_SECRET` | HMAC secret for JWT signing | `diaspora-dev-secret-change-in-prod` |
| `SOLANA_RPC_URL` | Solana RPC endpoint | `https://api.devnet.solana.com` |
| `SOLANA_PROGRAM_ID` | Deployed program ID | `5GHE14Zmpq5yNwpvHR2ZLaTcSckp6QogCRNm43M3Z9BT` |
| `ADMIN_PRIVATE_KEY` | Admin wallet base58 private key | _(empty – Solana disabled)_ |
| `TREASURY_PUBLIC_KEY` | Treasury token account public key | _(required in production)_ |
| `USDT_MINT_ADDRESS` | USDT mint address for the target cluster | _(required in production)_ |
| `MOBILE_MONEY_API_URL` | Mobile money provider base URL | `https://api.mobilemoney.com` |
| `MOBILE_MONEY_API_KEY` | Mobile money API key | _(empty)_ |
| `MOBILE_MONEY_API_SECRET` | Mobile money API secret | _(empty)_ |
| `PORT` | HTTP listen port | `8080` |
| `CACHE_DIR` | BadgerDB directory path | `./badger_data` |
