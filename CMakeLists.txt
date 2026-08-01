cmake_minimum_required(VERSION 3.24)
project(openinfer-studio-root LANGUAGES CXX)

# The Go backend is built by scripts/build.sh (go build ./apps/core).
# This root project builds the Qt desktop application.
add_subdirectory(apps/desktop)
