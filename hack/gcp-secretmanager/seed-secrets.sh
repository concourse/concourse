#!/usr/bin/env bash
# Seeds the test secrets used to exercise the gcpsecretmanager credential
# manager against the docker-compose stack. Idempotent: adds a new version if
# the secret already exists.
#
#   GCP_SECRETMANAGER_PROJECT=my-sandbox ./hack/gcp-secretmanager/seed-secrets.sh
#
# Tear down with ./hack/gcp-secretmanager/destroy-secrets.sh

set -euo pipefail

PROJECT="${GCP_SECRETMANAGER_PROJECT:?set GCP_SECRETMANAGER_PROJECT to your sandbox project ID}"
TEAM="${CONCOURSE_TEST_TEAM:-main}"
PIPELINE="${CONCOURSE_TEST_PIPELINE:-gcp-creds-test}"

put() {
  local id="$1" value="$2"

  # printf '%s', not echo: a trailing newline would corrupt the payload.
  if gcloud secrets describe "$id" --project="$PROJECT" >/dev/null 2>&1; then
    printf '%s' "$value" \
      | gcloud secrets versions add "$id" --project="$PROJECT" --data-file=- >/dev/null
    echo "updated  $id"
  else
    printf '%s' "$value" \
      | gcloud secrets create "$id" --project="$PROJECT" \
          --replication-policy=automatic --data-file=- >/dev/null
    echo "created  $id"
  fi
}

# Healthcheck
put "__concourse-health-check" "health-check-ok"

# Shared scope -- concourse--{{.Secret}}
put "concourse--shared-cred" "shared-value"

# Team scope -- concourse--{{.Team}}--{{.Secret}}
put "concourse--${TEAM}--team-cred" "team-value"

# Pipeline scope -- concourse--{{.Team}}--{{.Pipeline}}--{{.Secret}}
put "concourse--${TEAM}--${PIPELINE}--pipeline-cred" "pipeline-value"

# JSON object payload: resolves as a map so ((json-cred.username)) works.
put "concourse--${TEAM}--${PIPELINE}--json-cred" '{"username":"neo","password":"trinity"}'

# Same name at three scopes, to prove pipeline > team > shared precedence.
put "concourse--precedence-cred" "wrong-shared"
put "concourse--${TEAM}--precedence-cred" "wrong-team"
put "concourse--${TEAM}--${PIPELINE}--precedence-cred" "right-pipeline"

echo
echo "Seeded into project ${PROJECT} for team=${TEAM} pipeline=${PIPELINE}"
