#!/usr/bin/env bash
# Removes the secrets created by seed-secrets.sh. These are real cloud
# resources, so clean up when you are done testing.

set -euo pipefail

PROJECT="${GCP_SECRETMANAGER_PROJECT:?set GCP_SECRETMANAGER_PROJECT to your sandbox project ID}"
TEAM="${CONCOURSE_TEST_TEAM:-main}"
PIPELINE="${CONCOURSE_TEST_PIPELINE:-gcp-creds-test}"

ids=(
  "__concourse-health-check"
  "concourse--shared-cred"
  "concourse--${TEAM}--team-cred"
  "concourse--${TEAM}--${PIPELINE}--pipeline-cred"
  "concourse--${TEAM}--${PIPELINE}--json-cred"
  "concourse--precedence-cred"
  "concourse--${TEAM}--precedence-cred"
  "concourse--${TEAM}--${PIPELINE}--precedence-cred"
)

for id in "${ids[@]}"; do
  if gcloud secrets describe "$id" --project="$PROJECT" >/dev/null 2>&1; then
    gcloud secrets delete "$id" --project="$PROJECT" --quiet
    echo "deleted  $id"
  else
    echo "absent   $id"
  fi
done
