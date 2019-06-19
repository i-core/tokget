#!/usr/bin/env sh

chromium-browser                       \
    --headless                         \
    --disable-gpu                      \
    --disable-software-rasterizer      \
    --disable-dev-shm-usage            \
    --no-sandbox                       \
    --remote-debugging-address=0.0.0.0 \
    --remote-debugging-port=9222       \
    > /dev/null 2>&1 &
sleep 5
tokget --remote-chrome http://localhost:9222 $@