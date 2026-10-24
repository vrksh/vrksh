#!/usr/bin/env bash
# Serves site/ on localhost:8099 in the background. expect.sh stops it.
python3 -m http.server 8099 --bind 127.0.0.1 --directory site >/dev/null 2>&1 &
echo $! > server.pid
sleep 1
