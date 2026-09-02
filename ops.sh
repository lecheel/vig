#! /bin/bash
echo "build wig"

# Inject the current commit hash, git tag (if any), and a dirty flag
# into the version package at link time. The variables are declared in
# version/version.go and exposed via `wig --version`.
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Use `git describe` to find the nearest tag reachable from HEAD.
# This ensures we get "1.0" even if we are a few commits ahead of the tag.
GIT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
DIRTY="false"
# Ignore untracked files (e.g. the `wig` binary, new files not yet added)
# so the dirty flag only triggers when tracked files are modified.
if [ -n "$(git status --porcelain -uno 2>/dev/null)" ]; then
    DIRTY="true"
fi

# go test -v -run "TestCmdKillBufferWorkspaceIsolation|TestWorkspaceCacheStaleCleanup|TestWorkspaceCacheCaptureAndRestore"

LDFLAGS="-X github.com/firstrow/wig/version.Version=${GIT_VERSION} -X github.com/firstrow/wig/version.CommitHash=${COMMIT_HASH} -X github.com/firstrow/wig/version.Dirty=${DIRTY}"

echo "  commit: ${COMMIT_HASH} version: ${GIT_VERSION:-none} (dirty=${DIRTY})"
# GOTOOLCHAIN=go1.25.0  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o wig.exe ./cmd
GOTOOLCHAIN=go1.25.0 go build -ldflags "${LDFLAGS}" -o wig ./cmd
# go build -o wig cmd/main.go
cp wig ~/bin
