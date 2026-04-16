#!/bin/bash
set -euo pipefail

# Generate stateless node data from full node (witness) data.
#
# Usage:
#   ./scripts/gen-stateless-data.sh
#   ./scripts/gen-stateless-data.sh --src-dir /var/data_witness --dst-dir /var/data_stateless
#   SRC_DIR=/var/data_witness DST_DIR=/var/data_stateless ./scripts/gen-stateless-data.sh
#
# Prerequisites:
#   - Full node data available at SRC_DIR
#   - Binaries built: bin/exportblocks, bin/trimstate

SRC_DIR="${SRC_DIR:-/var/data_witness}"
DST_DIR="${DST_DIR:-/var/data_stateless}"
BIN_DIR="$(cd "$(dirname "$0")/.." && pwd)/bin"

usage() {
    cat <<EOF
Usage: $(basename "$0") [--src-dir PATH] [--dst-dir PATH]

Options:
  --src-dir PATH   Source witness/full-node data directory (default: $SRC_DIR)
  --dst-dir PATH   Destination stateless data directory (default: $DST_DIR)
  -h, --help       Show this help message

Environment overrides:
  SRC_DIR, DST_DIR
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --src-dir)
            [[ $# -ge 2 ]] || { echo "ERROR: --src-dir requires a value"; usage; exit 1; }
            SRC_DIR="$2"
            shift 2
            ;;
        --dst-dir)
            [[ $# -ge 2 ]] || { echo "ERROR: --dst-dir requires a value"; usage; exit 1; }
            DST_DIR="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "ERROR: unknown argument: $1"
            usage
            exit 1
            ;;
    esac
done

# Verify source directory
if [[ ! -d "$SRC_DIR" ]]; then
    echo "ERROR: source data directory $SRC_DIR does not exist"
    exit 1
fi

# Verify binaries
for bin in exportblocks trimstate; do
    if [[ ! -x "$BIN_DIR/$bin" ]]; then
        echo "ERROR: $BIN_DIR/$bin not found or not executable. Run 'make' first."
        exit 1
    fi
done

# Create destination directory
mkdir -p "$DST_DIR"

echo "=== Step 1: Copy blob.db, contractstaking.index.db, poll.db, trie.db.patch ==="
for f in blob.db contractstaking.index.db poll.db trie.db.patch; do
    src="$SRC_DIR/$f"
    if [[ -f "$src" ]]; then
        echo "  Copying $f ..."
        cp "$src" "$DST_DIR/$f"
    else
        echo "  WARNING: $src not found, skipping"
    fi
done

echo ""
echo "=== Step 2: Generate tip chain.db ==="
# Find the chain db file (e.g., chain-00000046.db)
CHAIN_SRC=$(ls "$SRC_DIR"/chain-*.db 2>/dev/null | head -1)
if [[ -z "$CHAIN_SRC" ]]; then
    echo "ERROR: no chain-*.db found in $SRC_DIR"
    exit 1
fi
echo "  Source chain DB: $CHAIN_SRC"
# Remove existing chain.db to avoid conflicts
rm -f "$DST_DIR/chain.db"
echo "  Running exportblocks ..."
"$BIN_DIR/exportblocks" -src-file "$CHAIN_SRC" -latest 1 -out "$DST_DIR/chain.db"
echo "  chain.db generated at $DST_DIR/chain.db"

echo ""
echo "=== Step 3: Copy and trim trie.db ==="
if [[ ! -e "$SRC_DIR/trie.db" ]]; then
    echo "ERROR: $SRC_DIR/trie.db not found"
    exit 1
fi
echo "  Copying trie.db (this may take a while) ..."
rm -rf "$DST_DIR/trie.db"
if [[ -d "$SRC_DIR/trie.db" ]]; then
    cp -r "$SRC_DIR/trie.db" "$DST_DIR/trie.db"
else
    cp "$SRC_DIR/trie.db" "$DST_DIR/trie.db"
fi
echo "  Running trimstate with compaction ..."
"$BIN_DIR/trimstate" -db "$DST_DIR/trie.db" -compact
echo "  trie.db trimmed at $DST_DIR/trie.db"

echo ""
echo "=== Done ==="
echo "Stateless node data is ready at $DST_DIR"
ls -lh "$DST_DIR"
