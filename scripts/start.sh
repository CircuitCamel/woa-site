#!/bin/bash
set -e

cd "$(dirname "$0")/.."

git pull
make build
./bin/warofages