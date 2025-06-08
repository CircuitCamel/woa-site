#!/bin/bash
set -e

git pull
make build
./bin/warofages