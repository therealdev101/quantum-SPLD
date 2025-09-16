package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// X402TxType is the EIP-2718 typed transaction ID for x402 settlement envelopes.
// Pick a high, unused value to avoid collisions with upstream types.
const X402TxType = 0x50

// X402Tx is a typed transaction envelope that carries an x402 settlement payload.
// It implements TxData so it can be propagated via the txpool and included in blocks.
// Notes:
// - Gas fields are kept for EIP-1559 compatibility but x402 consensus execution ignores fees (zero-fee policy).
// - The actual settlement logic (signature verification, balance moves, nonce registry) is executed in consensus
//   (e.g., in the Congress engine ApplySysTx path), based on the payload bytes contained in Data.
type X402Tx struct {
	// EIP-155 chain ID
	ChainID *big.Int

	// Standard transaction envelope fields (ignored for x402 economics)
	Nonce     uint64
	To        *common.Address
	Value     *big.Int
	Gas       uint64
	GasPrice  *big.Int
	GasFeeCap *big.Int
	GasTipCap *big.Int

	// Encoded x402 payload bytes (e.g., RLP or ABI-encoded PaymentPayloadData)
	// Consensus code will parse and validate these bytes during block processing.
	Input []byte

	// Signature values for the envelope (optional; consensus uses inner x402 signature)
	V *big.Int
	R *big.Int
	S *big.Int
}

// Ensure X402Tx implements TxData.
var _ TxData = (*X402Tx)(nil)

// txType returns the typed transaction discriminator.
func (x *X402Tx) txType() byte { return X402TxType }

// copy returns a deep copy of the X402Tx.
func (x *X402Tx) copy() TxData {
	if x == nil {
		return (*X402Tx)(nil)
	}
	cpy := &X402Tx{
		Nonce:     x.Nonce,
		Gas:       x.Gas,
		To:        copyAddressPtr(x.To),
		Input:     append([]byte(nil), x.Input...),
	}
	if x.ChainID != nil {
		cpy.ChainID = new(big.Int).Set(x.ChainID)
	}
	if x.Value != nil {
		cpy.Value = new(big.Int).Set(x.Value)
	}
	if x.GasPrice != nil {
		cpy.GasPrice = new(big.Int).Set(x.GasPrice)
	}
	if x.GasFeeCap != nil {
		cpy.GasFeeCap = new(big.Int).Set(x.GasFeeCap)
	}
	if x.GasTipCap != nil {
		cpy.GasTipCap = new(big.Int).Set(x.GasTipCap)
	}
	if x.V != nil {
		cpy.V = new(big.Int).Set(x.V)
	}
	if x.R != nil {
		cpy.R = new(big.Int).Set(x.R)
	}
	if x.S != nil {
		cpy.S = new(big.Int).Set(x.S)
	}
	return cpy
}

// chainID returns the EIP-155 chain ID.
func (x *X402Tx) chainID() *big.Int {
	if x.ChainID == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(x.ChainID)
}

// accessList returns nil for x402 envelopes (no ACL usage).
func (x *X402Tx) accessList() AccessList { return nil }

// data returns the input payload.
func (x *X402Tx) data() []byte { return x.Input }

// gas returns the gas limit (ignored by x402 settlement economics).
func (x *X402Tx) gas() uint64 { return x.Gas }

// gasPrice returns gas price (ignored by x402 settlement economics; may be zero).
func (x *X402Tx) gasPrice() *big.Int {
	if x.GasPrice == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(x.GasPrice)
}

// gasTipCap returns tip cap (ignored for x402).
func (x *X402Tx) gasTipCap() *big.Int {
	if x.GasTipCap == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(x.GasTipCap)
}

// gasFeeCap returns fee cap (ignored for x402).
func (x *X402Tx) gasFeeCap() *big.Int {
	if x.GasFeeCap == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(x.GasFeeCap)
}

// value returns the value field (not used by x402; amount is inside the x402 payload).
func (x *X402Tx) value() *big.Int {
	if x.Value == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(x.Value)
}

// nonce returns the nonce (envelope nonce; x402 has its own replay nonce inside payload).
func (x *X402Tx) nonce() uint64 { return x.Nonce }

// to returns the recipient (optional; not used by x402).
func (x *X402Tx) to() *common.Address { return x.To }

// rawSignatureValues returns the envelope signature (optional).
func (x *X402Tx) rawSignatureValues() (v, r, s *big.Int) {
	return x.V, x.R, x.S
}

// setSignatureValues sets the envelope signature (optional).
func (x *X402Tx) setSignatureValues(chainID, v, r, s *big.Int) {
	if chainID != nil {
		x.ChainID = new(big.Int).Set(chainID)
	}
	x.V = new(big.Int).Set(v)
	x.R = new(big.Int).Set(r)
	x.S = new(big.Int).Set(s)
}

// NewX402Tx constructs an x402 typed transaction envelope.
// gas/gasPrice/fee fields can be zero for system settlement (gasless UX).
func NewX402Tx(chainID *big.Int, nonce uint64, to *common.Address, payload []byte) *Transaction {
	inner := &X402Tx{
		ChainID:  new(big.Int).Set(chainID),
		Nonce:    nonce,
		To:       copyAddressPtr(to),
		Value:    new(big.Int), // x402 amount is in payload, not here
		Gas:      0,
		GasPrice: new(big.Int),
		GasFeeCap: new(big.Int),
		GasTipCap: new(big.Int),
		Input:    append([]byte(nil), payload...),
		V:        new(big.Int),
		R:        new(big.Int),
		S:        new(big.Int),
	}
	return NewTx(inner)
}
