#! /bin/bash
echo "build wig"

# Inject the current commit hash, git tag (if any), and a dirty flag
# into the version package at link time. The variables are declared in
# version/version.go and exposed via `wig --version`.
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_VERSION=$(git tag --points-at HEAD | head -n 1)
DIRTY="false"
# Ignore untracked files (e.g. the `wig` binary, new files not yet added)
# so the dirty flag only triggers when tracked files are modified.
if [ -n "$(git status --porcelain -uno 2>/dev/null)" ]; then
    DIRTY="true"
fi

LDFLAGS="-X github.com/firstrow/wig/version.Version=${GIT_VERSION} -X github.com/firstrow/wig/version.CommitHash=${COMMIT_HASH} -X github.com/firstrow/wig/version.Dirty=${DIRTY}"

echo "  commit: ${COMMIT_HASH} version: ${GIT_VERSION:-none} (dirty=${DIRTY})"
GOTOOLCHAIN=go1.25.0 go build -ldflags "${LDFLAGS}" -o wig ./cmd
# go build -o wig cmd/main.go
cp wig ~/bin
