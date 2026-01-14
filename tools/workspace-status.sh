#!/bin/sh -e

if test "${GITHUB_ACTIONS}" = "true" || test "${BONANZA_ENABLE_STAMP}" = "true"; then
  echo "BUILD_SCM_REVISION $(git rev-parse --short HEAD)"
  echo "BUILD_SCM_TIMESTAMP $(TZ=UTC git show -s --format=%cd --date=format:%Y%m%dT%H%M%SZ HEAD)"
fi
