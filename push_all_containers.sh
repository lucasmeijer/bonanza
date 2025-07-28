#!/bin/bash
set -euo pipefail

# Configuration
REGISTRY="${REGISTRY:-ghcr.io/lucasmeijer}"

# Components to push
COMPONENTS=(
    "bonanza_builder"
    "bonanza_browser"
    "bonanza_scheduler"
    "bonanza_storage_frontend"
    "bonanza_storage_shard"
    "bonanza_worker"
)

# Parse command line arguments
DEBUG_BUILD=false
EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_BUILD=true
            shift
            ;;
        *)
            EXTRA_ARGS+=("$1")
            shift
            ;;
    esac
done

# Set build flags based on debug option
if [ "$DEBUG_BUILD" = true ]; then
    echo "Building with debug symbols..."
    BUILD_FLAGS="--strip=never --compilation_mode=dbg"
else
    echo "Building in release mode..."
    BUILD_FLAGS=""
fi

# Create mac_dist directory if it doesn't exist
MAC_DIST_DIR="mac_dist"
mkdir -p "$MAC_DIST_DIR"
rm -f "$MAC_DIST_DIR/*"
echo "Building Mac arm64 binaries..."
echo

# Build bonanza_worker for Mac arm64
echo "Building bonanza_worker..."
bazel build $BUILD_FLAGS --cpu=darwin_arm64 //cmd/bonanza_worker:bonanza_worker

# Build bb_runner for Mac arm64
echo "Building bb_runner..."
bazel build $BUILD_FLAGS --cpu=darwin_arm64 @com_github_buildbarn_bb_remote_execution//cmd/bb_runner:bb_runner

# Copy binaries to mac_dist
echo
echo "Copying binaries to $MAC_DIST_DIR..."
cp bazel-bin/cmd/bonanza_worker/bonanza_worker_/bonanza_worker "$MAC_DIST_DIR/"
cp bazel-bin/external/com_github_buildbarn_bb_remote_execution+/cmd/bb_runner/bb_runner_/bb_runner "$MAC_DIST_DIR/"

# Fix permissions to make them writable
chmod 755 "$MAC_DIST_DIR/bonanza_worker" "$MAC_DIST_DIR/bb_runner"

echo "✓ Mac binaries copied to $MAC_DIST_DIR"

# Check binary sizes and architecture
echo
echo "Mac binary info:"
file "$MAC_DIST_DIR/bonanza_worker" "$MAC_DIST_DIR/bb_runner"
ls -lh "$MAC_DIST_DIR/bonanza_worker" "$MAC_DIST_DIR/bb_runner"

echo
echo "Pushing all Bonanza container images to ${REGISTRY}..."
echo

# Array to store container:tag pairs
declare -a PUSHED_IMAGES

for component in "${COMPONENTS[@]}"; do
    echo "Pushing ${component}..."
    repo_name="${component//_/-}"  # Replace all underscores with dashes
    
    # Capture output to extract the tag
    OUTPUT=$(bazel run $BUILD_FLAGS "//cmd/${component}:${component}_container_push" -- \
        --repository "${REGISTRY}/${repo_name}" \
        ${EXTRA_ARGS[@]:-} 2>&1)
    
    # Print the output for visibility
    echo "$OUTPUT"
    
    # Extract the tag from the output (looking for the pattern like "20250728T144343Z-17f1823")
    TAG=$(echo "$OUTPUT" | grep -oE '[0-9]{8}T[0-9]{6}Z-[a-f0-9]+' | tail -1)
    
    if [ -n "$TAG" ]; then
        PUSHED_IMAGES+=("${REGISTRY}/${repo_name}:${TAG}")
        echo "✓ ${component} pushed successfully with tag: ${TAG}"
    else
        echo "✓ ${component} pushed successfully"
    fi
    echo
done

# Push bb_runner_installer from external repository
echo "Pushing bb_runner_installer..."
OUTPUT=$(bazel run $BUILD_FLAGS "@com_github_buildbarn_bb_remote_execution//cmd/bb_runner:bb_runner_installer_container_push" -- \
    --repository "${REGISTRY}/bb-runner-installer" \
    ${EXTRA_ARGS[@]:-} 2>&1)

# Print the output for visibility
echo "$OUTPUT"

# Extract the tag
TAG=$(echo "$OUTPUT" | grep -oE '[0-9]{8}T[0-9]{6}Z-[a-f0-9]+' | tail -1)

if [ -n "$TAG" ]; then
    PUSHED_IMAGES+=("${REGISTRY}/bb-runner-installer:${TAG}")
    echo "✓ bb_runner_installer pushed successfully with tag: ${TAG}"
else
    echo "✓ bb_runner_installer pushed successfully"
fi
echo

echo "All container images pushed successfully!"
echo
echo "Pushed images with tags:"
echo "========================"
for image in "${PUSHED_IMAGES[@]}"; do
    echo "$image"
done
