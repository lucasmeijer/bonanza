#!/bin/sh -e

# Always output git information for container versioning
echo "BUILD_SCM_REVISION $(git rev-parse --short HEAD)"

# Use appropriate date command based on OS
if command -v gdate >/dev/null 2>&1; then
  # macOS with GNU coreutils installed
  echo "BUILD_SCM_TIMESTAMP $(TZ=UTC gdate --date "@$(git show -s --format=%ct HEAD)" +%Y%m%dT%H%M%SZ)"
else
  # Linux or if GNU date is available as 'date'
  echo "BUILD_SCM_TIMESTAMP $(TZ=UTC date --date "@$(git show -s --format=%ct HEAD)" +%Y%m%dT%H%M%SZ 2>/dev/null || date -u +%Y%m%dT%H%M%SZ)"
fi
