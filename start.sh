#!/usr/bin/env bash
# Start the S3 photo uploader locally with working AWS credentials.
#
# Credential strategy (best first):
#   1. App user key from the Terragrunt stack -> assume the bucket-scoped role
#   2. Fall back to your AWS CLI session exported as env vars
#
# Usage: ./start.sh
set -euo pipefail

cd "$(dirname "$0")"

ROLE_ARN_NAME="dev-s3-photo-uploader"
# Locate the s3-uploader Terragrunt stack wherever it lives under live/
STACK_DIR="$(find "$HOME/Documents/gitops_iac/live-infra-repo/live" \
  -name terragrunt.hcl -path '*/s3-uploader/*' -not -path '*/.terragrunt-cache/*' \
  2>/dev/null | head -1 | xargs dirname 2>/dev/null || true)"
PORT="$(grep -E '^PORT=' .env 2>/dev/null | cut -d= -f2 || true)"
PORT="${PORT:-8080}"

# ── 1. Make sure the CLI session is alive ───────────────────────────────────
if ! aws sts get-caller-identity >/dev/null 2>&1; then
  echo "AWS session expired — opening login..."
  aws login
fi

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_ARN_NAME}"

# The Go SDK can't read `aws login` sessions; export them as env vars.
eval "$(aws configure export-credentials --format env)"

# ── 2. Prefer the least-privilege role via the app user ────────────────────
if command -v terragrunt >/dev/null && [ -n "$STACK_DIR" ] && [ -d "$STACK_DIR" ]; then
  AKID="$(cd "$STACK_DIR" && terragrunt output -raw app_access_key_id 2>/dev/null || true)"
  SECRET="$(cd "$STACK_DIR" && terragrunt output -raw app_secret_access_key 2>/dev/null || true)"
  if [ -n "$AKID" ] && [ -n "$SECRET" ]; then
    CREDS="$(AWS_ACCESS_KEY_ID="$AKID" AWS_SECRET_ACCESS_KEY="$SECRET" AWS_SESSION_TOKEN= \
      aws sts assume-role --role-arn "$ROLE_ARN" --role-session-name s3uploader-local \
      --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' --output text 2>/dev/null || true)"
    if [ -n "$CREDS" ]; then
      export AWS_ACCESS_KEY_ID="$(echo "$CREDS" | cut -f1)"
      export AWS_SECRET_ACCESS_KEY="$(echo "$CREDS" | cut -f2)"
      export AWS_SESSION_TOKEN="$(echo "$CREDS" | cut -f3)"
      echo "Running under least-privilege role: $ROLE_ARN"
    else
      echo "WARN: could not assume $ROLE_ARN — falling back to your CLI session creds"
    fi
  else
    echo "WARN: app user key not found in Terragrunt outputs — using CLI session creds"
  fi
else
  echo "WARN: terragrunt or stack dir not found — using CLI session creds"
fi
unset AWS_PROFILE

# ── 3. Free the port if a previous instance is still running ───────────────
if lsof -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "Port $PORT busy — stopping previous instance..."
  lsof -nP -iTCP:"$PORT" -sTCP:LISTEN | awk 'NR>1 {print $2}' | xargs kill
  sleep 1
fi

# ── 4. Go ───────────────────────────────────────────────────────────────────
( sleep 2 && open "http://localhost:$PORT" ) &
exec go run .
