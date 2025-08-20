# Old-skool build tools.
#
# Targets (see each target for more information):
#   all: Build code.
#   build: Build code.
#   check: Run tests.
#   test: Run tests.
#   clean: Clean up.
#   release: Build release.

OUT_DIR = _output
OUT_PKG_DIR = Godeps/_workspace/pkg

export GOFLAGS

# Build code.
#
# Args:
#   GOFLAGS: Extra flags to pass to 'go' when building.
#
# Example:
#   make
#   make all
all build:
	hack/build-go.sh
.PHONY: all build

# Verify if code is properly organized.
#
# Example:
#   make verify
verify: build
	hack/verify-gofmt.sh
	hack/verify-golint.sh || true # ignored until existing lint errors are gone
	hack/verify-govet.sh
.PHONY: verify

# Install travis dependencies
#
# Example:
#   make install-travis
install-travis:
	hack/install-tools.sh
.PHONY: install-travis

# Build and run unit tests
#
# Args:
#   WHAT: Directory names to test.  All *_test.go files under these
#     directories will be run.  If not specified, "everything" will be tested.
#   TESTS: Same as WHAT.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#
# Example:
#   make check
#   make test
#   make check WHAT=pkg/build TESTFLAGS=-v
check:
	hack/verify-bash-completion.sh
	hack/test-go.sh $(WHAT) $(TESTS) $(TESTFLAGS)
.PHONY: check

# Remove all build artifacts.
#
# Example:
#   make clean
clean:
	rm -rf $(OUT_DIR) $(OUT_PKG_DIR)
.PHONY: clean

# Build the release.
#
# Example:
#   make release
release: clean
	hack/build-release.sh
	hack/extract-release.sh
.PHONY: release
