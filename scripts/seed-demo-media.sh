#!/bin/sh
# Deliberately local-only: no cloud endpoint, credential, or bucket overrides.
# curl supplies SigV4; this script does not implement a bespoke S3 signer.
set -eu
endpoint=http://object-store:9000
bucket=ocp-demo-media
credentials=ocp_local_media:ocp_local_media_only_123
work=$(mktemp -d)
trap 'rm -f "$work/response" "$work/download" "$work/headers"; rmdir "$work"' EXIT

signed() {
  curl --silent --show-error --connect-timeout 5 --max-time 30 \
    --aws-sigv4 aws:amz:us-east-1:s3 --user "$credentials" "$@"
}

curl --fail --silent --show-error --retry 10 --retry-connrefused --retry-delay 1 \
  --connect-timeout 2 --max-time 30 "$endpoint/minio/health/ready" >/dev/null
status=$(signed -o "$work/response" -w '%{http_code}' "$endpoint/$bucket")
case "$status" in
  200) ;;
  404) signed --fail -X PUT "$endpoint/$bucket" >/dev/null ;;
  *) printf 'Unexpected local bucket response: %s\n' "$status" >&2; exit 1 ;;
esac

cors_md5=$(openssl dgst -md5 -binary /workspace/deploy/local/media-cors.xml | openssl base64 -A)
signed --fail -X PUT --data-binary @/workspace/deploy/local/media-cors.xml -H "Content-MD5: $cors_md5" \
  "$endpoint/$bucket?cors" >/dev/null
# Actual image bytes use the client-side presigned URL path, not a provider upload.
OCP_TEST_LOCAL_MEDIA=1 go test -race -count=1 -v ./internal/media -run TestLocalPresigned
status=$(curl --silent --show-error --max-time 30 -o "$work/response" -w '%{http_code}' "$endpoint/$bucket")
test "$status" = 403
printf 'Verified anonymous bucket listing denied; development menus unchanged\n'
