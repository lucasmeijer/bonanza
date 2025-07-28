# Bonanza Container Build Analysis

This document provides an analysis of how the Bonanza project builds container images for its different components.

## Overview

Bonanza is a Bazel-based build system that consists of multiple microservices. The project uses Bazel rules to build and containerize Go binaries for deployment.

## Build System

The project uses Bazel as its primary build system with the following key characteristics:

- **Build Tool**: Bazel (MODULE.bazel configuration)
- **Language**: Go (primary language for all services)
- **Container Rules**: Uses `@com_github_buildbarn_bb_storage//tools:container.bzl` for container image creation
- **Container Registry**: Configured to use `ghcr.io/lucasmeijer` by default

## Container Build Process

### 1. Build Rules Structure

Each component follows a consistent pattern in its BUILD.bazel file:

1. **Go Library**: Defines the Go library with dependencies
2. **Go Binary**: Creates the executable from the library
3. **Container Image**: Uses `multiarch_go_image` to create multi-architecture container images
4. **Container Push**: Uses `container_push_official` to push images to registry

### 2. Key Build Functions

- **`multiarch_go_image`**: Creates container images that support multiple architectures
- **`container_push_official`**: Handles pushing images to the container registry with proper naming

### 3. Container Build Command

Container images are built and pushed using:
```bash
bazel run //cmd/{component}:{component}_container_push -- --repository {registry}/{repo-name}
```

## Components

The following components are containerized:

1. **bonanza_builder** (cmd/bonanza_builder/)
   - Main build orchestrator
   - Handles build requests and manages the build process
   - Connects to scheduler and storage services

2. **bonanza_browser** (cmd/bonanza_browser/)
   - Web UI service
   - Includes Tailwind CSS compilation during build
   - Provides browser interface for the build system

3. **bonanza_scheduler** (cmd/bonanza_scheduler/)
   - Job scheduling service
   - Manages work distribution to workers
   - Handles client and worker connections

4. **bonanza_storage_frontend** (cmd/bonanza_storage_frontend/)
   - Storage API frontend
   - Provides unified interface to storage shards
   - Handles object and tag operations

5. **bonanza_storage_shard** (cmd/bonanza_storage_shard/)
   - Storage backend service
   - Manages actual data storage
   - Handles persistence and caching

6. **bonanza_worker** (cmd/bonanza_worker/)
   - Build execution worker
   - Runs actual build commands
   - Manages virtual filesystem for builds

## Container Image Details

### Base Images
- The project uses Go binaries compiled with Bazel
- No explicit Dockerfiles are used - containers are built directly from Bazel rules
- Currently uses `gcr.io/distroless/static` as the base image (as defined in bb-storage's container.bzl)
- Images are minimal, containing only the Go binary and necessary runtime dependencies

### Multi-Architecture Support
- All images support multiple architectures through `multiarch_go_image`
- Supports linux/amd64 and linux/arm64 platforms
- This enables deployment on different CPU architectures

### Naming Convention
- Component names use underscores in code (e.g., `bonanza_builder`)
- Container images use hyphens (e.g., `bonanza-builder`)
- The `push_all_containers.sh` script handles this transformation

## Base Image Switching Options

### Current Implementation (container.bzl)

The `multiarch_go_image` function in bb-storage's container.bzl:
```starlark
oci_image(
    name = image_target,
    base = Label("@distroless_static"),
    entrypoint = ["/app/{}".format(native.package_relative_label(binary).name)],
    tars = [tar_target],
)
```

The base image is pulled in bb-storage's MODULE.bazel:
```starlark
oci.pull(
    name = "distroless_static",
    digest = "sha256:3f2b64ef97bd285e36132c684e6b2ae8f2723293d09aae046196cca64251acac",
    image = "gcr.io/distroless/static",
)
```

## Push Script

The `push_all_containers.sh` script automates pushing all container images:
- Iterates through all components
- Transforms naming convention (underscore to hyphen)
- Pushes to configured registry
- Supports additional bazel run arguments
