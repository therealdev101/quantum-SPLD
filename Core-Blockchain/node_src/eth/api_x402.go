// Copyright 2024 Splendor Blockchain
// Native x402 payments protocol implementation

package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"
	"sync"
	"os"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	ethapi "github.com/ethereum/go-ethereum/internal/ethapi"
	"strings"
)

	// X402API provides native x402 payment functionality
type X402API struct {
	eth *Ethereum

	// In-memory replay protection (demo-level; not durable across restarts)
	nonceMu    sync.Mutex
	usedNonces map[common.Address]map[common.Hash]uint64

	// Configurable protocol treasury
	treasuryAddr common.Address

	// Strict signature verification (production): if true, only accept canonical v2 EIP-191 format
	strictVerify bool
}

// NewX402API creates a new x402 API instance
func NewX402API(eth *Ethereum) *X402API {
	api := &X402API{
		eth:          eth,
		usedNonces:   make(map[common.Address]map[common.Hash]uint64),
		treasuryAddr: common.Address{},
	}
	// Load protocol treasury from env if provided
	if env := os.Getenv("X402_TREASURY_ADDRESS"); env != "" {
		if common.IsHexAddress(env) {
			api.treasuryAddr = common.HexToAddress(env)
			log.Info("X402: Protocol treasury configured from env", "address", api.treasuryAddr)
		} else {
			log.Warn("X402: Invalid X402_TREASURY_ADDRESS, falling back to default", "value", env)
		}
	}
	// Strict verify mode (production): enable with X402_STRICT_VERIFY=1|true
	if sv := os.Getenv("X402_STRICT_VERIFY"); sv == "1" || strings.EqualFold(sv, "true") {
		api.strictVerify = true
		log.Info("X402: Strict signature verification ENABLED")
	} else {
		log.Info("X402: Strict signature verification DISABLED (dev compatibility)")
	}
	return api
}

	// Helper methods for X402API (nonce tracking and config)
func (api *X402API) getProtocolTreasury() common.Address {
	// If treasuryAddr is set via env, use it; otherwise return default
	if (api.treasuryAddr != common.Address{}) {
		return api.treasuryAddr
	}
	// Default protocol treasury address
	return common.HexToAddress("0xd1D6E4F8777393Ac4dE10067EF6073048da0607d")
}

func (api *X402API) isNonceUsed(from common.Address, nonce common.Hash) bool {
	api.nonceMu.Lock()
	defer api.nonceMu.Unlock()
	if byFrom, ok := api.usedNonces[from]; ok {
		_, exists := byFrom[nonce]
		return exists
	}
	return false
}

func (api *X402API) isNonceUsedAndMark(from common.Address, nonce common.Hash) bool {
	api.nonceMu.Lock()
	defer api.nonceMu.Unlock()
	if byFrom, ok := api.usedNonces[from]; ok {
		if _, exists := byFrom[nonce]; exists {
			return true
		}
	} else {
		api.usedNonces[from] = make(map[common.Hash]uint64)
	}
	api.usedNonces[from][nonce] = uint64(time.Now().Unix())
	return false
}

// PaymentRequirements represents x402 payment requirements
type PaymentRequirements struct {
	Scheme              string         `json:"scheme"`
	Network             string         `json:"network"`
	MaxAmountRequired   *hexutil.Big   `json:"maxAmountRequired"`
	Resource            string         `json:"resource"`
	Description         string         `json:"description"`
	MimeType            string         `json:"mimeType"`
	PayTo               common.Address `json:"payTo"`
	MaxTimeoutSeconds   uint64         `json:"maxTimeoutSeconds"`
	Asset               common.Address `json:"asset"`
}

// PaymentPayload represents x402 payment data
type PaymentPayload struct {
	X402Version int                 `json:"x402Version"`
	Scheme      string              `json:"scheme"`
	Network     string              `json:"network"`
	Payload     PaymentPayloadData  `json:"payload"`
}

// PaymentPayloadData contains the actual payment data
type PaymentPayloadData struct {
	From            common.Address `json:"from"`
	To              common.Address `json:"to"`
	Value           *hexutil.Big   `json:"value"`
	ValidAfter      uint64         `json:"validAfter"`
	ValidBefore     uint64         `json:"validBefore"`
	Nonce           common.Hash    `json:"nonce"`
	Signature       hexutil.Bytes  `json:"signature"`
}

// VerificationResponse represents payment verification result
type VerificationResponse struct {
	IsValid       bool   `json:"isValid"`
	InvalidReason string `json:"invalidReason,omitempty"`
	PayerAddress  string `json:"payerAddress,omitempty"`
}

// SettlementResponse represents payment settlement result
type SettlementResponse struct {
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
	TxHash    common.Hash `json:"txHash,omitempty"`
	NetworkId string      `json:"networkId,omitempty"`
}

// SupportedResponse represents supported payment schemes
type SupportedResponse struct {
	Kinds []PaymentKind `json:"kinds"`
}

// PaymentKind represents a supported payment type
type PaymentKind struct {
	Scheme  string `json:"scheme"`
	Network string `json:"network"`
}

// Verify validates a payment without executing it
func (api *X402API) Verify(ctx context.Context, requirements PaymentRequirements, payload PaymentPayload) (*VerificationResponse, error) {
	log.Info("X402: Verifying payment", "from", payload.Payload.From, "to", payload.Payload.To, "value", payload.Payload.Value)

	// Basic validation
	if payload.Scheme != "exact" {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Unsupported payment scheme",
		}, nil
	}

	if payload.Network != "splendor" {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Unsupported network",
		}, nil
	}

	// Check timestamp validity
	now := uint64(time.Now().Unix())
	if now < payload.Payload.ValidAfter {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Payment not yet valid",
		}, nil
	}

	if now > payload.Payload.ValidBefore {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Payment expired",
		}, nil
	}

	// Verify signature
	if !api.verifyPaymentSignature(payload.Payload) {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Invalid signature",
		}, nil
	}

	// Check balance using state
	state, err := api.eth.blockchain.State()
	if err != nil {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Could not get blockchain state",
		}, nil
	}
	
	balance := state.GetBalance(payload.Payload.From)
	requiredAmount := (*big.Int)(payload.Payload.Value)
	
	if balance.Cmp(requiredAmount) < 0 {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Insufficient balance",
		}, nil
	}

	// Enforce exact-amount semantics for "exact" scheme
	maxRequired := (*big.Int)(requirements.MaxAmountRequired)
	if requiredAmount.Cmp(maxRequired) != 0 {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Payment amount must equal required amount",
		}, nil
	}

	// Verify recipient matches requirements
	if payload.Payload.To != requirements.PayTo {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Payment recipient mismatch",
		}, nil
	}

	// Check nonce replay (best-effort, in-memory)
	if api.isNonceUsed(payload.Payload.From, payload.Payload.Nonce) {
		return &VerificationResponse{
			IsValid:       false,
			InvalidReason: "Payment nonce already used",
		}, nil
	}

	return &VerificationResponse{
		IsValid:      true,
		PayerAddress: payload.Payload.From.Hex(),
	}, nil
}

// Settle executes a verified payment
func (api *X402API) Settle(ctx context.Context, requirements PaymentRequirements, payload PaymentPayload) (*SettlementResponse, error) {
	log.Info("X402: Settling payment", "from", payload.Payload.From, "to", payload.Payload.To, "value", payload.Payload.Value)

	// First verify the payment
	verification, err := api.Verify(ctx, requirements, payload)
	if err != nil {
		return &SettlementResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if !verification.IsValid {
		return &SettlementResponse{
			Success: false,
			Error:   verification.InvalidReason,
		}, nil
	}

	// Atomically check-and-mark nonce to prevent replay
	if api.isNonceUsedAndMark(payload.Payload.From, payload.Payload.Nonce) {
		return &SettlementResponse{
			Success: false,
			Error:   "payment nonce already used",
		}, nil
	}

	// Build typed X402 consensus transaction (system tx) and submit to txpool
	type x402Payload struct {
		From        common.Address
		To          common.Address
		Value       *big.Int
		ValidAfter  uint64
		ValidBefore uint64
		Nonce       common.Hash
		Signature   []byte
	}
	// Prepare payload (use the same signature and fields already verified above)
	p := x402Payload{
		From:        payload.Payload.From,
		To:          payload.Payload.To,
		Value:       (*big.Int)(payload.Payload.Value),
		ValidAfter:  payload.Payload.ValidAfter,
		ValidBefore: payload.Payload.ValidBefore,
		Nonce:       payload.Payload.Nonce,
		Signature:   append([]byte(nil), payload.Payload.Signature...),
	}
	enc, err := rlp.EncodeToBytes(&p)
	if err != nil {
		return &SettlementResponse{Success: false, Error: fmt.Sprintf("x402: encode payload failed: %v", err)}, nil
	}
	chainID := api.eth.blockchain.Config().ChainID
	xTx := types.NewX402Tx(chainID, 0, nil, enc)

	// Sign envelope with local etherbase (consensus engine will treat it as system tx)
	eb, err := api.eth.Etherbase()
	if err != nil {
		return &SettlementResponse{Success: false, Error: fmt.Sprintf("x402: etherbase not available: %v", err)}, nil
	}
	// Try to sign the envelope with local etherbase. If no wallet/signing is available,
	// fall back to submitting the unsigned typed tx (it will still execute as a system tx).
	var finalTx *types.Transaction = xTx
	if wallet, werr := api.eth.AccountManager().Find(accounts.Account{Address: eb}); wallet != nil && werr == nil {
		if signed, serr := wallet.SignTx(accounts.Account{Address: eb}, xTx, chainID); serr == nil {
			finalTx = signed
		} else {
			log.Warn("X402: failed to sign X402 envelope, submitting unsigned", "error", serr)
		}
	} else {
		log.Warn("X402: etherbase wallet not found, submitting unsigned X402 envelope", "error", werr)
	}
	// Submit to txpool for inclusion; consensus engine will execute during block processing
	txHash, addErr := ethapi.SubmitTransaction(ctx, api.eth.APIBackend, finalTx)
	if addErr != nil {
		return &SettlementResponse{Success: false, Error: fmt.Sprintf("x402: submit to txpool failed: %v", addErr)}, nil
	}
	return &SettlementResponse{
		Success:   true,
		TxHash:    txHash,
		NetworkId: "splendor",
	}, nil
}

// Supported returns supported payment schemes and networks
func (api *X402API) Supported(ctx context.Context) (*SupportedResponse, error) {
	return &SupportedResponse{
		Kinds: []PaymentKind{
			{
				Scheme:  "exact",
				Network: "splendor",
			},
		},
	}, nil
}

// verifyPaymentSignature verifies the payment signature
func (api *X402API) verifyPaymentSignature(payload PaymentPayloadData) bool {
	// Strict production mode: only accept canonical v2 (with chainId) and EIP-191 prefix, checksum addresses, hex value
	chainIDStrict := api.eth.networkID
	if api.strictVerify {
		valHex := payload.Value.String()
		msg := fmt.Sprintf("x402-payment:%s:%s:%s:%d:%d:%s:%d",
			payload.From.Hex(),
			payload.To.Hex(),
			valHex,
			payload.ValidAfter,
			payload.ValidBefore,
			payload.Nonce.Hex(),
			chainIDStrict,
		)
		// Signature checks
		sig := make([]byte, len(payload.Signature))
		copy(sig, payload.Signature)
		if len(sig) != 65 {
			return false
		}
		if sig[64] >= 27 {
			sig[64] -= 27
		}
		hash := accounts.TextHash([]byte(msg))
		if pub, err := crypto.SigToPub(hash, sig); err == nil {
			return crypto.PubkeyToAddress(*pub) == payload.From
		}
		return false
	}

	// Be permissive: try address case variants (checksum/lower), value encodings (hex/dec),
	// message versions (v2 with chainId, v1 without), and both EIP-191 prefixed and raw hashes.
	chainID := api.eth.networkID

	// Prepare address strings (checksum and lowercase)
	fromChecksum := payload.From.Hex()
	toChecksum := payload.To.Hex()
	fromLower := strings.ToLower(fromChecksum)
	toLower := strings.ToLower(toChecksum)

	// Prepare value strings
	valHex := payload.Value.String() // hexutil.Big typically prints 0x...
	valDec := (*big.Int)(payload.Value).String()

	// Build candidate message strings
	var msgs []string
	addrPairs := [][2]string{
		{fromChecksum, toChecksum},
		{fromLower, toLower},
	}
	vals := []string{valHex, valDec}

	for _, ap := range addrPairs {
		for _, v := range vals {
			// v2 (with chainId)
			msgs = append(msgs, fmt.Sprintf("x402-payment:%s:%s:%s:%d:%d:%s:%d",
				ap[0], ap[1], v, payload.ValidAfter, payload.ValidBefore, payload.Nonce.Hex(), chainID))
			// v1 (without chainId)
			msgs = append(msgs, fmt.Sprintf("x402-payment:%s:%s:%s:%d:%d:%s",
				ap[0], ap[1], v, payload.ValidAfter, payload.ValidBefore, payload.Nonce.Hex()))
		}
	}

	// Signature copy and sanity
	sig := make([]byte, len(payload.Signature))
	copy(sig, payload.Signature)
	if len(sig) != 65 {
		log.Warn("X402: signature length invalid", "len", len(sig))
		return false
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	log.Info("X402: signature meta", "len", len(sig), "v", int(sig[64]))

	recoverAddr := func(hash []byte) (common.Address, bool) {
		var zero common.Address
		pub, err := crypto.SigToPub(hash, sig)
		if err != nil {
			log.Info("X402: SigToPub error", "err", err)
			return zero, false
		}
		addr := crypto.PubkeyToAddress(*pub)
		return addr, true
	}

	for _, m := range msgs {
		// EIP-191 prefixed
		hashPrefixed := accounts.TextHash([]byte(m))
		if rec, ok := recoverAddr(hashPrefixed); ok {
			if rec == payload.From {
				return true
			}
			log.Info("X402: recover mismatch (prefixed)", "msg", m, "recovered", rec, "expected", payload.From)
		}
		// Raw (eth_sign style)
		hashRaw := crypto.Keccak256([]byte(m))
		if rec, ok := recoverAddr(hashRaw); ok {
			if rec == payload.From {
				return true
			}
			log.Info("X402: recover mismatch (raw)", "msg", m, "recovered", rec, "expected", payload.From)
		}
	}
	log.Warn("X402: signature did not match any accepted message variants",
		"from", fromChecksum, "to", toChecksum, "valHex", valHex, "valDec", valDec, "chainId", chainID)
	return false
}


// executeNativeTransfer performs the actual token transfer
func (api *X402API) executeNativeTransfer(payload PaymentPayloadData) (common.Hash, error) {
	// Get current state
	state, err := api.eth.blockchain.State()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get blockchain state: %v", err)
	}

	// Check balance again
	balance := state.GetBalance(payload.From)
	amount := (*big.Int)(payload.Value)
	
	if balance.Cmp(amount) < 0 {
		return common.Hash{}, errors.New("insufficient balance for transfer")
	}

	// Update balances directly (bypassing normal transaction flow for x402)
	// This is much faster than creating actual transactions for micropayments
	// WARNING: Demo-mode settlement; not consensus-safe or persisted to chain. Replace with on-chain mechanism for production.
	log.Warn("X402: Demo-mode settlement path; not consensus-safe. Replace with on-chain mechanism for production")
	state.SubBalance(payload.From, amount)
	state.AddBalance(payload.To, amount)
	
	// Generate a pseudo transaction hash for tracking
	txHash := crypto.Keccak256Hash(
		payload.From.Bytes(),
		payload.To.Bytes(),
		amount.Bytes(),
		payload.Nonce.Bytes(),
	)

	log.Info("X402: Payment settled", "txHash", txHash.Hex(), "from", payload.From, "to", payload.To, "amount", amount)
	
	return txHash, nil
}

// GetPaymentHistory returns payment history for an address
func (api *X402API) GetPaymentHistory(ctx context.Context, address common.Address, limit int) ([]PaymentRecord, error) {
	// This would be implemented with proper storage in a production system
	return []PaymentRecord{}, nil
}

// PaymentRecord represents a historical payment record
type PaymentRecord struct {
	TxHash      common.Hash    `json:"txHash"`
	From        common.Address `json:"from"`
	To          common.Address `json:"to"`
	Amount      *hexutil.Big   `json:"amount"`
	Timestamp   uint64         `json:"timestamp"`
	Resource    string         `json:"resource"`
	Status      string         `json:"status"`
}

// GetPaymentStats returns payment statistics
func (api *X402API) GetPaymentStats(ctx context.Context) (*PaymentStats, error) {
	return &PaymentStats{
		TotalPayments:     0,
		TotalVolume:       (*hexutil.Big)(big.NewInt(0)),
		AveragePayment:    (*hexutil.Big)(big.NewInt(0)),
		ActiveUsers:       0,
		PaymentsToday:     0,
		VolumeToday:       (*hexutil.Big)(big.NewInt(0)),
	}, nil
}

// PaymentStats represents payment statistics
type PaymentStats struct {
	TotalPayments     uint64       `json:"totalPayments"`
	TotalVolume       *hexutil.Big `json:"totalVolume"`
	AveragePayment    *hexutil.Big `json:"averagePayment"`
	ActiveUsers       uint64       `json:"activeUsers"`
	PaymentsToday     uint64       `json:"paymentsToday"`
	VolumeToday       *hexutil.Big `json:"volumeToday"`
}

// processValidatorRevenue handles validator revenue sharing for x402 payments
func (api *X402API) processValidatorRevenue(payload PaymentPayloadData, txHash common.Hash) {
	// Get current block header to identify the validator
	currentBlock := api.eth.blockchain.CurrentBlock()
	if currentBlock == nil {
		log.Error("X402: Could not get current block for validator revenue")
		return
	}
	
	// The validator who processes this payment gets the revenue
	validator := currentBlock.Coinbase()
	amount := (*big.Int)(payload.Value)
	
	// REVENUE SPLIT (Zero-fee mode):
	// 100% to API provider (the "To" address)
	// 0% validator, 0% protocol treasury
	
	validatorFee := big.NewInt(0)
	protocolFee := big.NewInt(0)
	
	// Zero-fee mode: no protocol fee distribution
	log.Info("X402: Zero-fee mode - no protocol/validator fees applied")
	
	// Create x402 payment record
	x402Payment := X402Payment{
		TxHash:      txHash,
		From:        payload.From,
		To:          payload.To,
		Amount:      amount,
		Fee:         validatorFee,
		Validator:   validator,
		Timestamp:   uint64(time.Now().Unix()),
		Resource:    "x402-payment",
		BlockNumber: currentBlock.NumberU64(),
	}
	
	// Process through validator rewards system
	rewardsSystem := GetX402ValidatorRewards()
	if rewardsSystem != nil {
		if err := rewardsSystem.ProcessX402Payment(x402Payment); err != nil {
			log.Error("X402: Failed to process validator revenue", "error", err)
		}
	} else {
		log.Warn("X402: Validator rewards system not initialized")
	}
	
	log.Info("X402: Revenue distributed",
		"payment", amount,
		"apiProvider", payload.To,
		"validator", validator,
		"validatorFee", validatorFee,
		"protocolFee", protocolFee,
		"treasury", api.getProtocolTreasury(),
		"txHash", txHash,
	)
}

// GetValidatorX402Revenue returns x402 revenue for a validator (RPC method)
func (api *X402API) GetValidatorX402Revenue(ctx context.Context, validator common.Address) (*hexutil.Big, error) {
	rewardsSystem := GetX402ValidatorRewards()
	if rewardsSystem == nil {
		return (*hexutil.Big)(big.NewInt(0)), nil
	}
	
	return rewardsSystem.GetValidatorX402Revenue(ctx, validator)
}

// GetX402RevenueStats returns comprehensive x402 revenue statistics (RPC method)
func (api *X402API) GetX402RevenueStats(ctx context.Context) (*X402RevenueStats, error) {
	rewardsSystem := GetX402ValidatorRewards()
	if rewardsSystem == nil {
		return &X402RevenueStats{
			TotalRevenue:   (*hexutil.Big)(big.NewInt(0)),
			ValidatorCount: 0,
			TotalPayments:  0,
		}, nil
	}
	
	return rewardsSystem.GetX402RevenueStats(ctx)
}

// GetTopPerformingValidators returns AI-ranked validators by x402 performance (RPC method)
func (api *X402API) GetTopPerformingValidators(ctx context.Context, limit int) ([]ValidatorRanking, error) {
	rewardsSystem := GetX402ValidatorRewards()
	if rewardsSystem == nil {
		return []ValidatorRanking{}, nil
	}
	
	return rewardsSystem.GetTopPerformingValidators(ctx, limit)
}

// SetValidatorFeeShare sets the percentage of x402 payments that go to validators (RPC method)
func (api *X402API) SetValidatorFeeShare(ctx context.Context, percentage float64) error {
	rewardsSystem := GetX402ValidatorRewards()
	if rewardsSystem == nil {
		return fmt.Errorf("validator rewards system not initialized")
	}
	
	return rewardsSystem.SetValidatorFeeShare(ctx, percentage)
}

// SetDistributionMode sets how x402 revenue is distributed among validators (RPC method)
func (api *X402API) SetDistributionMode(ctx context.Context, mode string) error {
	rewardsSystem := GetX402ValidatorRewards()
	if rewardsSystem == nil {
		return fmt.Errorf("validator rewards system not initialized")
	}
	
	return rewardsSystem.SetDistributionMode(ctx, mode)
}
