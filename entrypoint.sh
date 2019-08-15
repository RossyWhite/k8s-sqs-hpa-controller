#!/usr/bin/env bash
set -e

case "$1" in
  run)
    exec /usr/src/app/k8s-sqs-hpa-controller
    ;;
  *)
    exec "$@"
    ;;
esac
