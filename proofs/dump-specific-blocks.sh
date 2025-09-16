#!/bin/bash

# === CONFIG ===
OUT_DIR="/root/splendor-blockchain-v4/proofs"
mkdir -p "$OUT_DIR"

# List of blocks you want to dump (excluding 21018 - already have good data)
BLOCKS=(21019 21020)

for B in "${BLOCKS[@]}"; do
    echo "📦 Dumping block $B..."
    /root/splendor-blockchain-v4/Core-Blockchain/node_src/build/bin/geth attach --datadir /root/splendor-blockchain-v4/Core-Blockchain/chaindata/node1 --exec "eth.getBlock($B, true)" > "$OUT_DIR/block-$B.json"
    echo "✅ Saved to $OUT_DIR/block-$B.json"
done

echo "🎉 All blocks dumped successfully!"
echo "Files created:"
for B in "${BLOCKS[@]}"; do
    echo "  - $OUT_DIR/block-$B.json"
done
