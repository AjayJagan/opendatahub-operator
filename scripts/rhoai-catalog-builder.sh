#!/bin/bash

set -euo pipefail

#######################################
# RHOAI Catalog Builder
#
# Builds a custom OLM catalog containing two RHOAI operator bundle versions.
# Handles both pre-published bundles from registry.redhat.io and locally-built bundles.
#
# Features:
# - Checks registry.redhat.io first for existing bundles (uses podman manifest inspect)
# - Builds bundles locally only if not found in registry
# - Auto-stashes uncommitted changes and restores them at the end
# - Uses correct rhods-operator package name for RHOAI catalogs
# - Supports dry-run mode for testing
#######################################

#######################################
# Example commands:
# Test 2.25.2 → 3.3.0 upgrade
# ./scripts/rhoai-catalog-builder.sh \
#   --custom-bundles "quay.io/rhoai/odh-operator-bundle:rhoai-2.25,quay.io/ajaganat/opendatahub-operator-bundle:v3.3.0" \
#   --registry quay.io/ajaganat \
#   --catalog-tag v2.25.2-3.3.0

# # Test multiple version chain: 2.25 → 3.2 → 3.3
# ./scripts/rhoai-catalog-builder.sh \
#   --custom-bundles "quay.io/rhoai/odh-operator-bundle:rhoai-2.25,quay.io/rhoai/odh-operator-bundle:rhoai-3.2,quay.io/ajaganat/bundle:v3.3.0" \
#   --registry quay.io/ajaganat

# Special path where quay image is used and local build is also done
# ./scripts/rhoai-catalog-builder.sh \
#   --version1 2.25.2 \
#   --version2 3.3.0 \
#   --registry quay.io/ajaganat \
#   --catalog-tag v2.25.2-3.3.0
#######################################


# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Global variables
VERSION1=""
VERSION2=""
REGISTRY=""
CATALOG_TAG=""
BUNDLE_REGISTRY="quay.io/rhoai/odh-operator-bundle"
CUSTOM_BUNDLES=""
DRY_RUN=false
ORIGINAL_BRANCH=""
BUNDLE_IMG_1=""
BUNDLE_IMG_2=""
STASHED_CHANGES=false
OPM_BIN=""

#######################################
# Logging functions
#######################################
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

#######################################
# Usage
#######################################
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Builds a custom RHOAI catalog image containing operator bundle versions.

Mode 1: Two-version catalog (default):
  Required:
    --version1 <VER>      First bundle version (e.g., 2.25.2)
    --version2 <VER>      Second bundle version (e.g., 3.3.0)
    --registry <REG>      Push registry for locally-built images (e.g., quay.io/ajaganat)

Mode 2: Custom bundle list:
  Required:
    --custom-bundles <LIST>  Comma-separated list of bundle images
                             (e.g., "quay.io/rhoai/odh-operator-bundle:rhoai-2.25,quay.io/user/bundle:v3.3.0")
    --registry <REG>         Push registry for catalog image

Optional:
  --catalog-tag <TAG>   Tag for catalog image (default: v<version1>-<version2> or custom)
  --bundle-registry <R> Registry to check for existing bundles
                        (default: quay.io/rhoai/odh-operator-bundle)
  --dry-run             Print commands without executing
  --help                Show this help message

Examples:
  # Build catalog with two versions from quay.io/rhoai
  $0 --version1 2.25.2 --version2 3.3.0 --registry quay.io/myuser

  # Build catalog with custom bundle list
  $0 --custom-bundles "quay.io/rhoai/odh-operator-bundle:rhoai-2.25,quay.io/myuser/bundle:v3.3.0" \\
     --registry quay.io/myuser --catalog-tag my-custom-catalog

  # Dry run to see what would happen
  $0 --version1 2.25.2 --version2 3.3.0 --registry quay.io/myuser --dry-run

EOF
    exit 0
}

#######################################
# Execute command (respects dry-run)
#######################################
execute() {
    if [[ "$DRY_RUN" == true ]]; then
        echo -e "${YELLOW}[DRY-RUN]${NC} $*" >&2
    else
        log_info "Executing: $*"
        eval "$@"
    fi
}

#######################################
# Cleanup function
#######################################
cleanup() {
    if [[ "$DRY_RUN" == false ]]; then
        # Return to original branch if we switched
        if [[ -n "$ORIGINAL_BRANCH" ]]; then
            local current_branch
            current_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
            if [[ -n "$current_branch" ]] && [[ "$current_branch" != "$ORIGINAL_BRANCH" ]]; then
                log_info "Returning to original branch: $ORIGINAL_BRANCH"
                git checkout "$ORIGINAL_BRANCH" 2>/dev/null || true
            fi
        fi

        # Restore stashed changes if we created a stash
        if [[ "$STASHED_CHANGES" == true ]]; then
            log_info "Restoring stashed changes..."
            if git stash pop &>/dev/null; then
                log_success "Stashed changes restored"
            else
                log_warn "Could not restore stash automatically. Run: git stash pop"
            fi
        fi
    fi
}

trap cleanup EXIT ERR INT TERM

#######################################
# Argument parsing
#######################################
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version1)
                VERSION1="$2"
                shift 2
                ;;
            --version2)
                VERSION2="$2"
                shift 2
                ;;
            --registry)
                REGISTRY="$2"
                shift 2
                ;;
            --catalog-tag)
                CATALOG_TAG="$2"
                shift 2
                ;;
            --bundle-registry)
                BUNDLE_REGISTRY="$2"
                shift 2
                ;;
            --custom-bundles)
                CUSTOM_BUNDLES="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --help)
                usage
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                ;;
        esac
    done

    # Validate required arguments based on mode
    if [[ -n "$CUSTOM_BUNDLES" ]]; then
        # Custom bundles mode
        if [[ -n "$VERSION1" ]] || [[ -n "$VERSION2" ]]; then
            log_error "Cannot use --version1/--version2 with --custom-bundles"
            exit 1
        fi
        if [[ -z "$REGISTRY" ]]; then
            log_error "Missing required argument: --registry"
            exit 1
        fi
    else
        # Traditional two-version mode
        if [[ -z "$VERSION1" ]]; then
            log_error "Missing required argument: --version1 (or use --custom-bundles)"
            exit 1
        fi
        if [[ -z "$VERSION2" ]]; then
            log_error "Missing required argument: --version2 (or use --custom-bundles)"
            exit 1
        fi
        if [[ -z "$REGISTRY" ]]; then
            log_error "Missing required argument: --registry"
            exit 1
        fi

        # Validate versions are different
        if [[ "$VERSION1" == "$VERSION2" ]]; then
            log_error "Versions must be different (both are $VERSION1)"
            exit 1
        fi

        # Validate version format (basic semver check)
        if ! [[ "$VERSION1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            log_error "Invalid version format for --version1: $VERSION1 (expected X.Y.Z)"
            exit 1
        fi
        if ! [[ "$VERSION2" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            log_error "Invalid version format for --version2: $VERSION2 (expected X.Y.Z)"
            exit 1
        fi
    fi

    # Remove trailing slash from registry if present
    REGISTRY="${REGISTRY%/}"

    # Set default catalog tag if not provided
    if [[ -z "$CATALOG_TAG" ]]; then
        CATALOG_TAG="v${VERSION1}-${VERSION2}"
    fi

    log_info "Configuration:"
    if [[ -n "$CUSTOM_BUNDLES" ]]; then
        log_info "  Custom Bundles: $CUSTOM_BUNDLES"
    else
        log_info "  Version 1: $VERSION1"
        log_info "  Version 2: $VERSION2"
    fi
    log_info "  Push Registry: $REGISTRY"
    log_info "  Bundle Registry: $BUNDLE_REGISTRY"
    log_info "  Catalog Tag: $CATALOG_TAG"
    log_info "  Dry Run: $DRY_RUN"
}

#######################################
# Check prerequisites
#######################################
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check for required commands
    local required_commands=("podman" "make" "git" "yq")
    for cmd in "${required_commands[@]}"; do
        if ! command -v "$cmd" &> /dev/null; then
            log_error "Required command not found: $cmd"
            exit 1
        fi
    done


    # Check we're in the repo root
    if [[ ! -f "Makefile" ]] || [[ ! -f "get_all_manifests.sh" ]]; then
        log_error "This script must be run from the opendatahub-operator repository root"
        exit 1
    fi

    # Save original branch
    ORIGINAL_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    log_info "Current branch: $ORIGINAL_BRANCH"

    # Stash uncommitted changes if any exist (and not in dry-run)
    if [[ "$DRY_RUN" == false ]]; then
        if [[ -n "$(git status --porcelain)" ]]; then
            log_warn "Working directory has uncommitted changes"
            log_info "Stashing changes temporarily..."
            if git stash push -u -m "rhoai-catalog-builder auto-stash $(date +%Y%m%d-%H%M%S)" &>/dev/null; then
                STASHED_CHANGES=true
                log_success "Changes stashed (will be restored at the end)"
            else
                log_error "Failed to stash changes. Please commit or stash them manually."
                git status --short
                exit 1
            fi
        fi
    fi

    # Check podman login for push registry
    if [[ "$DRY_RUN" == false ]]; then
        local registry_domain
        registry_domain=$(echo "$REGISTRY" | cut -d'/' -f1)
        if ! podman login --get-login "$registry_domain" &>/dev/null; then
            log_error "Not logged into $registry_domain. Please run: podman login $registry_domain"
            exit 1
        fi
    fi

    # Detect opm binary location
    if [[ -f "./bin/opm" ]]; then
        OPM_BIN="./bin/opm"
    elif command -v opm &>/dev/null; then
        OPM_BIN="opm"
    else
        log_info "opm not found, downloading..."
        execute "make opm"
        OPM_BIN="./bin/opm"
    fi

    log_info "Using opm at: $OPM_BIN"
    log_success "All prerequisites met"
}


#######################################
# Check if bundle exists in registry
#######################################
check_bundle_available() {
    local version=$1
    # For quay.io/rhoai bundles, use tag format like "rhoai-2.25" instead of "v2.25.2"
    local major minor
    major=$(echo "$version" | cut -d'.' -f1)
    minor=$(echo "$version" | cut -d'.' -f2)
    local bundle_img="${BUNDLE_REGISTRY}:rhoai-${major}.${minor}"

    log_info "Checking if bundle exists in registry: $bundle_img"

    # Use podman manifest inspect to check if image exists (doesn't download layers)
    # This is lightweight, so we run it even in dry-run mode
    if podman manifest inspect "$bundle_img" &>/dev/null; then
        log_success "Bundle exists in registry: $bundle_img"
        echo "$bundle_img"
        return 0
    else
        log_warn "Bundle not found in registry: $bundle_img"
        if [[ "$DRY_RUN" == false ]]; then
            log_info "Try logging in: podman login quay.io"
        fi
        return 1
    fi
}

#######################################
# Convert version to branch name
#######################################
version_to_branch() {
    local version=$1
    local major minor
    major=$(echo "$version" | cut -d'.' -f1)
    minor=$(echo "$version" | cut -d'.' -f2)
    local branch="rhoai-${major}.${minor}"

    # Debug output in dry-run mode
    if [[ "$DRY_RUN" == true ]]; then
        log_info "Looking for branch: $branch" >&2
    fi

    # Check if branch exists (match exact branch name with word boundaries)
    # Temporarily disable pipefail because grep -q exits early and breaks the pipe
    set +o pipefail
    git branch -a | grep -qw "$branch"
    local found=$?
    set -o pipefail

    if [[ $found -ne 0 ]]; then
        log_error "Branch not found: $branch" >&2
        log_error "Available rhoai branches:" >&2
        git branch -a | grep rhoai >&2 || echo "  (none found)" >&2
        return 1
    fi

    echo "$branch"
    return 0
}

#######################################
# Build and push operator and bundle for a version
#######################################
build_and_push_version() {
    local version=$1
    local branch
    branch=$(version_to_branch "$version") || exit 1

    log_info "Building version $version from branch $branch"

    # Prompt user for confirmation
    if [[ "$DRY_RUN" == false ]]; then
        echo ""
        read -p "About to checkout branch $branch and build version $version. Continue? [y/N] " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_error "Build cancelled by user"
            exit 1
        fi
    fi

    # Checkout target branch
    execute "git checkout $branch"

    # Clean opt directory to ensure fresh manifest generation
    # This prevents stale symlinks from previous builds (especially on macOS)
    log_info "Cleaning opt directory..."
    execute "rm -rf opt"

    # Generate code and manifests from API types
    # This ensures deepcopy methods and CRDs match the current branch's API types
    # Critical when switching between branches with different components (e.g., SparkOperator)
    log_info "Generating code and manifests..."
    execute "make generate manifests ODH_PLATFORM_TYPE=rhoai"

    # Pull manifests
    log_info "Pulling manifests for RHOAI..."
    execute "ODH_PLATFORM_TYPE=rhoai ./get_all_manifests.sh"

    # Check for and remove broken symlinks in opt directory
    # Symlinks can break when building on macOS and deploying on Linux
    # Broken symlinks typically point to components that don't exist in this version
    log_info "Checking for broken symlinks in opt directory..."
    if [[ "$DRY_RUN" == false ]]; then
        local broken_symlinks
        broken_symlinks=$(find opt -type l ! -exec test -e {} \; -print 2>/dev/null || true)
        if [[ -n "$broken_symlinks" ]]; then
            log_warn "Found broken symlinks in opt directory:"
            echo "$broken_symlinks" >&2
            log_warn "Removing broken symlinks automatically..."

            # Remove each broken symlink
            while IFS= read -r symlink; do
                if [[ -n "$symlink" ]]; then
                    log_info "  Removing: $symlink"
                    rm -f "$symlink"
                fi
            done <<< "$broken_symlinks"

            log_success "Broken symlinks removed"
        else
            log_success "No broken symlinks found"
        fi
    fi

    # Build operator image
    log_info "Building operator image..."
    execute "make image-build \
        ODH_PLATFORM_TYPE=rhoai \
        IMAGE_TAG_BASE=${REGISTRY}/opendatahub-operator \
        IMG_TAG=v${version}"

    # Push operator image
    log_info "Pushing operator image..."
    execute "make image-push IMG=${REGISTRY}/opendatahub-operator:v${version}"

    # Build bundle image
    log_info "Building bundle image..."
    execute "make bundle-build \
        ODH_PLATFORM_TYPE=rhoai \
        VERSION=${version} \
        IMAGE_TAG_BASE=${REGISTRY}/opendatahub-operator \
        IMG_TAG=v${version}"

    # Push bundle image
    local bundle_img="${REGISTRY}/opendatahub-operator-bundle:v${version}"
    log_info "Pushing bundle image..."
    execute "make bundle-push BUNDLE_IMG=${bundle_img}"

    # Return to original branch
    execute "git checkout $ORIGINAL_BRANCH"

    log_success "Built and pushed version $version"
    echo "$bundle_img"
}

#######################################
# Build catalog
#######################################
build_catalog() {
    local bundle_imgs="$1"
    local catalog_img="${REGISTRY}/opendatahub-operator-catalog:${CATALOG_TAG}"

    log_info "Building catalog with bundles: $bundle_imgs"

    # Clean and create catalog directory
    execute "rm -rf catalog"
    execute "mkdir -p catalog"

    if [[ "$DRY_RUN" == false ]]; then
        log_info "Creating file-based catalog using opm render..."

        # Split bundle images
        IFS=',' read -ra images <<< "$bundle_imgs"

        # Collect bundle names for channel creation
        local bundle_names=()
        local bundle_files=()

        # Render each bundle
        local idx=0
        for img in "${images[@]}"; do
            idx=$((idx + 1))
            local bundle_file="catalog/bundle${idx}.json"

            log_info "Rendering bundle ${idx}: $img"

            # Render bundle to JSON
            if ! $OPM_BIN render "$img" > "$bundle_file" 2>&1; then
                log_error "Failed to render bundle: $img"
                cat "$bundle_file" >&2
                exit 1
            fi

            # Extract bundle name
            local bundle_name
            bundle_name=$(jq -r 'select(.schema == "olm.bundle") | .name' "$bundle_file" 2>/dev/null)

            if [[ -z "$bundle_name" ]]; then
                log_error "Failed to extract bundle name from $img"
                exit 1
            fi

            log_info "  Bundle name: $bundle_name"
            bundle_names+=("$bundle_name")
            bundle_files+=("$bundle_file")
        done

        # Create catalog.yaml with package declaration
        log_info "Creating catalog structure..."
        cat > catalog/catalog.yaml << EOF
---
schema: olm.package
name: rhods-operator
defaultChannel: fast
EOF

        # Append each bundle (in JSON format from opm render)
        for bundle_file in "${bundle_files[@]}"; do
            echo "---" >> catalog/catalog.yaml
            cat "$bundle_file" >> catalog/catalog.yaml
        done

        # Create channel entry with upgrade path
        log_info "Creating upgrade channel..."
        echo "---" >> catalog/catalog.yaml
        cat >> catalog/catalog.yaml << EOF
schema: olm.channel
package: rhods-operator
name: fast
entries:
EOF

        # Add channel entries - oldest to newest (OLM requirement)
        if [[ ${#bundle_names[@]} -eq 2 ]]; then
            # Assume bundle1 is older (2.25.2) and bundle2 is newer (3.3.0)
            cat >> catalog/catalog.yaml << EOF
  - name: ${bundle_names[0]}
  - name: ${bundle_names[1]}
    replaces: ${bundle_names[0]}
EOF
        elif [[ ${#bundle_names[@]} -eq 1 ]]; then
            cat >> catalog/catalog.yaml << EOF
  - name: ${bundle_names[0]}
EOF
        else
            log_error "Unexpected number of bundles: ${#bundle_names[@]}"
            exit 1
        fi

        # Clean up temporary bundle files
        rm -f catalog/bundle*.json

        # Validate catalog
        log_info "Validating catalog..."
        if $OPM_BIN validate catalog 2>&1 | tee /tmp/opm-validate.log; then
            log_success "Catalog validation passed"
        else
            # Check if it's just warnings (common for file-based catalogs)
            if grep -qi "error" /tmp/opm-validate.log; then
                log_error "Catalog validation failed"
                cat /tmp/opm-validate.log
                exit 1
            else
                log_warn "Catalog validation had warnings (this is common)"
            fi
        fi
        rm -f /tmp/opm-validate.log

        # Show catalog summary
        log_info "Catalog contents:"
        local bundle_count
        bundle_count=$(grep -c '"schema": "olm.bundle"' catalog/catalog.yaml || echo 0)
        log_info "  Package: rhods-operator"
        log_info "  Channel: fast"
        log_info "  Bundles: $bundle_count"
        for name in "${bundle_names[@]}"; do
            log_info "    - $name"
        done
    else
        log_warn "[DRY-RUN] Skipping actual catalog preparation"
    fi

    # Build catalog image using file-based catalog Dockerfile
    log_info "Building catalog image..."

    # Use opm to serve the file-based catalog
    execute "podman build --no-cache --load \
        -f - \
        --platform linux/amd64 \
        -t ${catalog_img} . <<'EOF'
FROM registry.redhat.io/openshift4/ose-operator-registry:v4.14

# Copy the catalog directory
COPY catalog /configs

# Expose gRPC port
EXPOSE 50051

# Set entrypoint to serve the file-based catalog
ENTRYPOINT [\"/bin/opm\"]
CMD [\"serve\", \"/configs\"]
EOF"

    # Push catalog image
    log_info "Pushing catalog image..."
    execute "podman push ${catalog_img}"

    log_success "Catalog built and pushed: $catalog_img"
    echo "$catalog_img"
}

#######################################
# Print summary
#######################################
print_summary() {
    local catalog_img=$1

    echo ""
    echo "======================================"
    log_success "RHOAI Catalog Build Complete!"
    echo "======================================"
    echo ""
    echo "Bundle Images:"

    if [[ -n "$CUSTOM_BUNDLES" ]]; then
        # Custom bundles mode
        local idx=1
        IFS=',' read -ra bundles <<< "$CUSTOM_BUNDLES"
        for bundle in "${bundles[@]}"; do
            echo "  ${idx}. $bundle"
            ((idx++))
        done
    else
        # Traditional two-version mode
        # Show source for bundle 1
        if [[ "$BUNDLE_IMG_1" =~ ^quay\.io/rhoai ]]; then
            echo "  1. $BUNDLE_IMG_1 (from quay.io/rhoai)"
        else
            echo "  1. $BUNDLE_IMG_1 (built from source)"
        fi

        # Show source for bundle 2
        if [[ "$BUNDLE_IMG_2" =~ ^quay\.io/rhoai ]]; then
            echo "  2. $BUNDLE_IMG_2 (from quay.io/rhoai)"
        else
            echo "  2. $BUNDLE_IMG_2 (built from source)"
        fi
    fi

    echo ""
    echo "Catalog Image:"
    echo "  $catalog_img"
    echo ""
    echo "To use this catalog in OpenShift, create a CatalogSource:"
    echo ""
    cat << EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: rhoai-custom-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: ${catalog_img}
  displayName: "RHOAI Custom Catalog"
  publisher: "Custom"
  updateStrategy:
    registryPoll:
      interval: 10m
EOF
    echo ""
    echo "After applying the CatalogSource, the operator will appear in OperatorHub."
    echo ""

    # Show usage note
    log_info "Note: This catalog uses bundles from quay.io/rhoai (public registry)"
    echo ""
}

#######################################
# Main
#######################################
main() {
    log_info "RHOAI Catalog Builder"
    echo ""

    parse_args "$@"
    check_prerequisites

    local bundle_images=""

    if [[ -n "$CUSTOM_BUNDLES" ]]; then
        # Custom bundles mode - use provided bundle list directly
        log_info "Using custom bundle list"
        bundle_images="$CUSTOM_BUNDLES"

        # Validate bundle images exist
        IFS=',' read -ra bundles <<< "$bundle_images"
        for bundle in "${bundles[@]}"; do
            log_info "Verifying bundle: $bundle"
            # Try manifest inspect first (for multi-arch images), fallback to image inspect
            if podman manifest inspect "$bundle" &>/dev/null || podman image inspect "$bundle" &>/dev/null; then
                log_success "Bundle verified: $bundle"
            else
                log_error "Bundle image not accessible: $bundle"
                log_info "Make sure you can pull this image: podman pull $bundle"
                exit 1
            fi
        done
    else
        # Traditional two-version mode
        # Check bundle availability and build if necessary
        log_info "Processing version $VERSION1..."
        log_info "Checking if $VERSION1 is available..."

        # Temporarily disable exit-on-error for the check
        set +e
        BUNDLE_IMG_1=$(check_bundle_available "$VERSION1")
        local check1_status=$?
        set -e

        if [[ $check1_status -eq 0 ]] && [[ -n "$BUNDLE_IMG_1" ]]; then
            log_success "Using existing bundle: $BUNDLE_IMG_1"
        else
            log_warn "Bundle not available, will build locally"
            BUNDLE_IMG_1=$(build_and_push_version "$VERSION1")
        fi

        log_info "Processing version $VERSION2..."
        log_info "Checking if $VERSION2 is available..."

        # Temporarily disable exit-on-error for the check
        set +e
        BUNDLE_IMG_2=$(check_bundle_available "$VERSION2")
        local check2_status=$?
        set -e

        if [[ $check2_status -eq 0 ]] && [[ -n "$BUNDLE_IMG_2" ]]; then
            log_success "Using existing bundle: $BUNDLE_IMG_2"
        else
            log_warn "Bundle not available, will build locally"
            BUNDLE_IMG_2=$(build_and_push_version "$VERSION2")
        fi

        bundle_images="${BUNDLE_IMG_1},${BUNDLE_IMG_2}"
    fi

    # Build catalog
    local catalog_img
    catalog_img=$(build_catalog "$bundle_images")

    # Print summary
    print_summary "$catalog_img"
}

main "$@"
