package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

func usage() {
	fmt.Fprintf(os.Stderr, `x402sign - minimal signer for x402 canonical messages (strict mode)

Usage:
  # Print address from a hex private key
  x402sign -key 0x<privatekey> addr

  # Sign a canonical x402 message string (text) with EIP-191 prefix
  x402sign -key 0x<privatekey> sign "x402-payment:{from}:{to}:{value}:{validAfter}:{validBefore}:{nonce}:{chainId}"

Notes:
  - The -key must be a 32-byte hex string (with or without 0x).
  - The "sign" command expects the exact message text the server reconstructs.
  - Output signature is 0x-prefixed hex, 65 bytes (r||s||v) with v in {27,28}.

`)
	os.Exit(2)
}

func loadPriv(hexkey string) (*ecdsa.PrivateKey, error) {
	k := strings.TrimPrefix(strings.TrimSpace(hexkey), "0x")
	b, err := hex.DecodeString(k)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("want 32-byte privkey, got %d", len(b))
	}
	return crypto.ToECDSA(b)
}

func main() {
	log.SetFlags(0)
	key := flag.String("key", "", "hex private key (0x...)")
	flag.Usage = usage
	flag.Parse()

	if *key == "" || flag.NArg() < 1 {
		usage()
	}

	priv, err := loadPriv(*key)
	if err != nil {
		log.Fatalf("load key: %v", err)
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)

	cmd := flag.Arg(0)
	switch cmd {
	case "addr":
		fmt.Println(addr.Hex())
	case "sign":
		if flag.NArg() < 2 {
			log.Fatalf("missing message to sign")
		}
		msg := flag.Arg(1)
		hash := accounts.TextHash([]byte(msg))
		sig, err := crypto.Sign(hash, priv)
		if err != nil {
			log.Fatalf("sign: %v", err)
		}
		// Normalize v to {27,28}
		if sig[64] < 27 {
			sig[64] += 27
		}
		fmt.Printf("0x%x\n", sig)
	default:
		usage()
	}
}
