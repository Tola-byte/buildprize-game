# X402 Payment Integration Architecture

## Overview
Integrate X402 (HTTP 402 Payment Required) protocol to enable pay-per-question gameplay using cryptocurrency (USDC stablecoins).

---

## System Architecture

### 1. **Data Models**

#### Player Model Extensions
```go
type Player struct {
    ID            string    `json:"id"`
    Username      string    `json:"username"`
    Score         int       `json:"score"`
    Streak        int       `json:"streak"`
    IsReady       bool      `json:"is_ready"`
    
    // X402 Payment Fields
    WalletAddress string    `json:"wallet_address,omitempty"` // User's crypto wallet
    Balance       float64   `json:"balance,omitempty"`          // Cached balance (USDC)
    PaymentStatus string    `json:"payment_status,omitempty"`   // "pending", "paid", "failed"
}
```

#### Payment Transaction Model (New)
```go
type PaymentTransaction struct {
    ID            string    `json:"id"`
    PlayerID      string    `json:"player_id"`
    LobbyID       string    `json:"lobby_id"`
    QuestionID    string    `json:"question_id"`
    Amount        float64   `json:"amount"`        // USDC amount
    Status        string    `json:"status"`        // "pending", "confirmed", "failed"
    TxHash        string    `json:"tx_hash,omitempty"` // Blockchain transaction hash
    CreatedAt     time.Time `json:"created_at"`
    ConfirmedAt   *time.Time `json:"confirmed_at,omitempty"`
}
```

#### Lobby Payment Settings (New)
```go
type LobbyPaymentConfig struct {
    LobbyID       string  `json:"lobby_id"`
    PricePerQuestion float64 `json:"price_per_question"` // e.g., 0.10 USDC
    PaymentRequired bool   `json:"payment_required"`     // Enable/disable payments
    PaymentNetwork string  `json:"payment_network"`     // "base", "ethereum", etc.
}
```

---

## 2. **Backend Architecture**

### Payment Service (`internal/services/payment_service.go`)
```go
type PaymentService interface {
    // Request payment (returns HTTP 402 response data)
    RequestPayment(playerID, lobbyID, questionID string, amount float64) (*PaymentRequest, error)
    
    // Verify payment transaction
    VerifyPayment(txHash string) (bool, error)
    
    // Check wallet balance
    GetBalance(walletAddress string) (float64, error)
    
    // Process payment callback (when payment confirmed)
    ProcessPaymentCallback(txHash string, playerID string) error
}
```

### Payment Flow in Game Service
```go
// Modified SubmitAnswer flow:
func (gs *GameService) SubmitAnswer(lobbyID, playerID string, answer int, responseTime int64) error {
    // 1. Check if payment required
    if gs.paymentService.IsPaymentRequired(lobbyID) {
        // 2. Check if player has paid for this question
        if !gs.paymentService.HasPaidForQuestion(playerID, lobbyID, questionID) {
            return ErrPaymentRequired // Returns HTTP 402
        }
    }
    
    // 3. Process answer (existing logic)
    // ... rest of SubmitAnswer
}
```

### New API Endpoints

#### `POST /api/v1/lobbies/:id/answer` (Modified)
- **Before**: Directly processes answer
- **After**: 
  - If payment required → Check payment status
  - If not paid → Return HTTP 402 with payment details
  - If paid → Process answer normally

#### `POST /api/v1/payments/request` (New)
```json
Request:
{
  "player_id": "uuid",
  "lobby_id": "uuid",
  "question_id": "uuid",
  "amount": 0.10
}

Response (HTTP 402):
{
  "error": "Payment Required",
  "payment_request": {
    "amount": "0.10",
    "currency": "USDC",
    "network": "base",
    "recipient_address": "0x...",
    "payment_id": "uuid",
    "expires_at": "2024-01-01T12:00:00Z"
  }
}
```

#### `POST /api/v1/payments/verify` (New)
```json
Request:
{
  "tx_hash": "0x...",
  "player_id": "uuid"
}

Response:
{
  "verified": true,
  "transaction": {
    "id": "uuid",
    "amount": 0.10,
    "status": "confirmed",
    "confirmed_at": "2024-01-01T12:00:00Z"
  }
}
```

#### `GET /api/v1/players/:id/balance` (New)
```json
Response:
{
  "wallet_address": "0x...",
  "balance": 5.50,
  "currency": "USDC",
  "network": "base"
}
```

#### `POST /api/v1/players/:id/wallet` (New)
```json
Request:
{
  "wallet_address": "0x..."
}

Response:
{
  "wallet_address": "0x...",
  "verified": true
}
```

---

## 3. **Frontend Architecture**

### Wallet Integration Service (`frontend/src/services/wallet.js`)
```javascript
class WalletService {
  // Connect wallet (MetaMask, Coinbase Wallet, etc.)
  async connectWallet() {
    // Use ethers.js or web3.js
    // Request account access
    // Return wallet address
  }
  
  // Check if wallet is connected
  isConnected() {
    // Check if wallet address exists in localStorage/state
  }
  
  // Get wallet balance
  async getBalance(walletAddress) {
    // Query blockchain for USDC balance
  }
  
  // Send payment transaction
  async sendPayment(amount, recipientAddress, network) {
    // Create transaction
    // Sign with wallet
    // Broadcast to network
    // Return transaction hash
  }
}
```

### Payment Flow in GameScreen Component
```javascript
// Modified handleSubmitAnswer:
const handleSubmitAnswer = async (answer) => {
  try {
    // Try to submit answer
    await api.submitAnswer(lobby.id, player.id, answer, responseTime);
  } catch (error) {
    if (error.status === 402) {
      // Payment required - show payment modal
      const paymentRequest = error.data.payment_request;
      await handlePayment(paymentRequest);
      
      // Retry answer submission after payment
      await api.submitAnswer(lobby.id, player.id, answer, responseTime);
    } else {
      // Other error
      throw error;
    }
  }
};

// New payment handler:
const handlePayment = async (paymentRequest) => {
  // 1. Show payment modal with amount, recipient, etc.
  // 2. User confirms payment
  // 3. Send transaction via wallet
  // 4. Wait for confirmation
  // 5. Verify payment with backend
  // 6. Close modal, continue game
};
```

### Payment Modal Component (`frontend/src/components/PaymentModal.jsx`)
```javascript
// Shows:
// - Amount to pay (e.g., "0.10 USDC")
// - Recipient address
// - Network (Base, Ethereum, etc.)
// - "Connect Wallet" button (if not connected)
// - "Pay Now" button
// - Transaction status (pending, confirmed, failed)
// - Transaction hash link (block explorer)
```

---

## 4. **Payment Flow Sequence**

```
┌─────────┐         ┌──────────┐         ┌──────────┐         ┌─────────┐
│ Player  │         │ Frontend │         │ Backend  │         │ Wallet  │
└────┬────┘         └────┬─────┘         └────┬─────┘         └────┬────┘
     │                   │                     │                     │
     │ 1. Select Answer  │                     │                     │
     │──────────────────>│                     │                     │
     │                   │ 2. POST /answer    │                     │
     │                   │─────────────────────>│                     │
     │                   │                     │ 3. Check Payment    │
     │                   │                     │    Status            │
     │                   │                     │                     │
     │                   │ 4. HTTP 402         │                     │
     │                   │    Payment Required │                     │
     │                   │<────────────────────│                     │
     │                   │                     │                     │
     │ 5. Show Payment   │                     │                     │
     │    Modal          │                     │                     │
     │<──────────────────│                     │                     │
     │                   │                     │                     │
     │ 6. Connect Wallet │                     │                     │
     │──────────────────────────────────────────────────────────────>│
     │                   │                     │                     │
     │ 7. Approve Payment│                     │                     │
     │──────────────────────────────────────────────────────────────>│
     │                   │                     │                     │
     │                   │ 8. Get Tx Hash      │                     │
     │                   │<──────────────────────────────────────────│
     │                   │                     │                     │
     │                   │ 9. POST /verify     │                     │
     │                   │    {tx_hash}       │                     │
     │                   │─────────────────────>│                     │
     │                   │                     │ 10. Verify on-chain │
     │                   │                     │                     │
     │                   │ 11. Payment Verified│                     │
     │                   │<────────────────────│                     │
     │                   │                     │                     │
     │ 12. Retry Answer  │                     │                     │
     │──────────────────>│                     │                     │
     │                   │ 13. POST /answer    │                     │
     │                   │─────────────────────>│                     │
     │                   │                     │ 14. Process Answer  │
     │                   │                     │                     │
     │                   │ 15. Success         │                     │
     │                   │<────────────────────│                     │
     │                   │                     │                     │
     │ 16. Show Result   │                     │                     │
     │<──────────────────│                     │                     │
```

---

## 5. **Database Schema Extensions**

### New Tables

#### `player_wallets`
```sql
CREATE TABLE player_wallets (
    id VARCHAR(36) PRIMARY KEY,
    player_id VARCHAR(36) NOT NULL,
    wallet_address VARCHAR(42) NOT NULL UNIQUE, -- Ethereum address format
    network VARCHAR(20) NOT NULL DEFAULT 'base',
    verified BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);
```

#### `payment_transactions`
```sql
CREATE TABLE payment_transactions (
    id VARCHAR(36) PRIMARY KEY,
    player_id VARCHAR(36) NOT NULL,
    lobby_id VARCHAR(36) NOT NULL,
    question_id VARCHAR(36),
    amount DECIMAL(18, 6) NOT NULL, -- USDC (6 decimals)
    currency VARCHAR(10) DEFAULT 'USDC',
    network VARCHAR(20) DEFAULT 'base',
    status VARCHAR(20) DEFAULT 'pending', -- pending, confirmed, failed
    tx_hash VARCHAR(66), -- Transaction hash
    recipient_address VARCHAR(42) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    confirmed_at TIMESTAMP WITH TIME ZONE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE,
    FOREIGN KEY (lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE
);
```

#### `lobby_payment_configs`
```sql
CREATE TABLE lobby_payment_configs (
    lobby_id VARCHAR(36) PRIMARY KEY,
    payment_required BOOLEAN DEFAULT false,
    price_per_question DECIMAL(18, 6) DEFAULT 0.10,
    currency VARCHAR(10) DEFAULT 'USDC',
    network VARCHAR(20) DEFAULT 'base',
    recipient_address VARCHAR(42) NOT NULL, -- Your wallet to receive payments
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE
);
```

---

## 6. **Blockchain Integration**

### Required Libraries
- **Go Backend**: 
  - `github.com/ethereum/go-ethereum` - Ethereum client
  - `github.com/ethers-io/ethers.go` (if available) or direct RPC calls

- **Frontend**:
  - `ethers` (v6) - Wallet interaction
  - `@coinbase/wallet-sdk` - Coinbase Wallet support
  - `@metamask/detect-provider` - MetaMask detection

### Network Configuration
```go
// internal/config/payment.go
type PaymentConfig struct {
    Network          string  // "base", "ethereum", "polygon"
    RPCURL           string  // e.g., "https://mainnet.base.org"
    USDCContractAddr string  // USDC contract address on network
    RecipientAddress string  // Your wallet to receive payments
    PricePerQuestion float64 // Default: 0.10 USDC
}
```

### Payment Verification
```go
// Verify transaction on-chain
func (ps *PaymentService) VerifyPayment(txHash string) (bool, error) {
    // 1. Get transaction from blockchain
    // 2. Verify recipient address matches
    // 3. Verify amount matches
    // 4. Verify transaction is confirmed (enough block confirmations)
    // 5. Return true if all checks pass
}
```

---

## 7. **Security Considerations**

### 1. **Payment Verification**
- Always verify payments on-chain (don't trust frontend)
- Require minimum block confirmations (e.g., 2-3 blocks)
- Verify transaction recipient matches your wallet
- Verify amount matches expected payment

### 2. **Replay Prevention**
- Track which questions have been paid for
- Prevent duplicate payments for same question
- Use unique payment IDs per question

### 3. **Rate Limiting**
- Limit payment verification requests
- Prevent spam payment attempts

### 4. **Wallet Validation**
- Validate wallet address format
- Optionally verify wallet ownership (signature challenge)

---

## 8. **Error Handling**

### Payment Errors
```go
var (
    ErrPaymentRequired    = errors.New("payment required")
    ErrInsufficientFunds = errors.New("insufficient funds")
    ErrPaymentFailed      = errors.New("payment failed")
    ErrPaymentPending     = errors.New("payment pending confirmation")
    ErrInvalidTransaction = errors.New("invalid transaction")
)
```

### Frontend Error States
- Payment modal shows error messages
- Retry mechanism for failed payments
- Clear user feedback on payment status

---

## 9. **Configuration**

### Environment Variables
```bash
# Payment Configuration
PAYMENT_ENABLED=true
PAYMENT_NETWORK=base
PAYMENT_RPC_URL=https://mainnet.base.org
PAYMENT_USDC_CONTRACT=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913
PAYMENT_RECIPIENT_ADDRESS=0x...
PAYMENT_PRICE_PER_QUESTION=0.10
PAYMENT_MIN_CONFIRMATIONS=2
```

---

## 10. **Implementation Phases**

### Phase 1: Foundation
1. Add wallet address to Player model
2. Create payment transaction table
3. Add wallet connection UI
4. Store wallet address on player

### Phase 2: Payment Request
1. Implement HTTP 402 response in SubmitAnswer
2. Create payment request endpoint
3. Build payment modal component
4. Integrate wallet SDK

### Phase 3: Payment Processing
1. Implement payment transaction sending
2. Add payment verification endpoint
3. Verify transactions on-chain
4. Process answers after payment confirmation

### Phase 4: Lobby Configuration
1. Add payment settings to lobby creation
2. Allow hosts to set price per question
3. Enable/disable payments per lobby

### Phase 5: Balance & History
1. Show wallet balance in UI
2. Display payment history
3. Add balance checks before game start

---

## 11. **Testing Strategy**

### Unit Tests
- Payment service methods
- Transaction verification logic
- Balance calculations

### Integration Tests
- End-to-end payment flow
- WebSocket + payment integration
- Database transaction handling

### Manual Testing
- Connect different wallets (MetaMask, Coinbase)
- Test payment on testnet first
- Verify payments on block explorer
- Test error scenarios (insufficient funds, failed tx)

---

## 12. **Future Enhancements**

- **Payment Pools**: Pre-fund account, deduct per question
- **Subscription Model**: Pay once, play multiple games
- **Prize Pools**: Winners get portion of collected fees
- **Multi-Currency**: Support multiple stablecoins
- **Gas Optimization**: Batch payments or use Layer 2
- **Payment Analytics**: Track revenue, popular games, etc.

---

## Summary

**Key Points:**
- ✅ No Solidity needed - HTTP-based protocol
- ✅ Pay-per-question model
- ✅ Real-time payment verification
- ✅ Works with existing wallet infrastructure
- ✅ Blockchain-agnostic (Base, Ethereum, etc.)

**Next Steps:**
1. Choose network (Base recommended for low gas fees)
2. Set up RPC connection
3. Implement wallet connection
4. Add payment verification
5. Test on testnet first!

