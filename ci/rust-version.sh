#!/usr/bin/env bash

# Prints Rust version

while read -r name equals value _; do
  if [[ $name = "channel" && $equals = = ]]; then
    echo "${value//\"/}"
  fi
done < <(cat $(dirname "${BASH_SOURCE[0]}")/../rust-toolchain.toml)
