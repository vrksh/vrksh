#!/usr/bin/env bash
# Stand-in for a real deploy script: noisy output on stdout and stderr.
echo "INFO starting deploy of web-7"
echo "INFO api_key=sk-proj-FAKE0000example0000notreal0000000 loaded"
echo "WARN health check slow" >&2
echo "ERROR migration 0042 failed" >&2
exit 1
