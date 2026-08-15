#!/usr/bin/env bash
# Create a new contrib module with the standard structure.
# Usage: ./scripts/new-contrib.sh <module-name>
# Example: ./scripts/new-contrib.sh pack-foo
set -euo pipefail

NAME="${1:?Usage: $0 <module-name> (e.g., pack-foo, storage-memcached)}"
DIR="contrib/$NAME"
MODULE="go.klarlabs.de/agent/contrib/$NAME"

# Pin new modules to the latest published core tag (no local replace).
AGENT_VERSION="${AGENT_VERSION:-v0.15.0}"
GO_VERSION="${GO_VERSION:-1.26.2}"

# Derive Go package name from module name (remove prefix, replace hyphens)
PKG=$(echo "$NAME" | sed 's/^pack-//;s/^storage-//;s/^approval-//;s/-/_/g')

# Portable in-place sed (GNU and BSD).
sed_inplace() {
	local expr=$1
	local file=$2
	if sed --version >/dev/null 2>&1; then
		sed -i "$expr" "$file"
	else
		sed -i '' "$expr" "$file"
	fi
}

if [ -d "$DIR" ]; then
	echo "Error: $DIR already exists"
	exit 1
fi

echo "Creating contrib module: $NAME"
echo "  Directory: $DIR"
echo "  Package: $PKG"
echo "  Module: $MODULE"
echo "  Requires: go.klarlabs.de/agent $AGENT_VERSION"
echo

mkdir -p "$DIR"

cat > "$DIR/go.mod" <<GOMOD
module $MODULE

go $GO_VERSION

require go.klarlabs.de/agent $AGENT_VERSION
GOMOD

if [[ "$NAME" == pack-* ]]; then
	cat > "$DIR/$PKG.go" <<'GOSRC'
// Package PKGNAME provides tools for DESCRIPTION.
//
// Status: stub scaffold — add WithHandler implementations before use.
package PKGNAME

import (
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the tool pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("MODNAME").
		WithDescription("[STUB] DESCRIPTION tools").
		WithVersion("0.1.0").
		WithMetadata("status", "stub").
		AddTools(tools()...).
		Build()
}

func tools() []tool.Tool {
	return []tool.Tool{
		// Add tools here using tool.NewBuilder("tool_name").
		//     WithDescription("Does something").
		//     WithAnnotations(tool.Annotations{ReadOnly: true}).
		//     WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
		//         // Implementation
		//     }).
		//     MustBuild(),
	}
}
GOSRC
	sed_inplace "s/PKGNAME/$PKG/g;s/MODNAME/$NAME/g;s/DESCRIPTION/TODO/g" "$DIR/$PKG.go"

	cat > "$DIR/pack_test.go" <<'GOTEST'
package PKGNAME

import (
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestPack_RegistersTools(t *testing.T) {
	p := Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Metadata["status"] != "stub" {
		t.Fatalf("expected stub metadata, got %q", p.Metadata["status"])
	}
	if len(p.Tools) == 0 {
		t.Skip("no tools registered yet")
	}
}

func TestPack_ToolsImplementInterface(t *testing.T) {
	p := Pack()
	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
	}
}
GOTEST
	sed_inplace "s/PKGNAME/$PKG/g" "$DIR/pack_test.go"
else
	cat > "$DIR/$PKG.go" <<'GOSRC'
// Package PKGNAME provides DESCRIPTION.
package PKGNAME
GOSRC
	sed_inplace "s/PKGNAME/$PKG/g;s/DESCRIPTION/TODO/g" "$DIR/$PKG.go"

	cat > "$DIR/${PKG}_test.go" <<'GOTEST'
package PKGNAME

import "testing"

func TestPlaceholder(t *testing.T) {
	// Add tests here
}
GOTEST
	sed_inplace "s/PKGNAME/$PKG/g" "$DIR/${PKG}_test.go"
fi

# Add to go.work (portable insert before closing paren of use block)
if ! grep -q "./$DIR" go.work 2>/dev/null; then
	tmp=$(mktemp)
	awk -v entry="./$DIR" '
		/^)$/ && !done { print "\t" entry; done=1 }
		{ print }
	' go.work > "$tmp"
	mv "$tmp" go.work
	echo "Added ./$DIR to go.work"
fi

# Prefer workspace resolution during development.
(cd "$DIR" && go mod tidy 2>/dev/null || true)

echo
echo "Created $DIR/"
ls -la "$DIR/"
echo
echo "Next steps:"
echo "  1. Edit $DIR/$PKG.go — add handlers (remove stub metadata when ready)"
echo "  2. Run: cd $DIR && GOWORK=off go test ./..."
echo "  3. Run: go work sync"
