#!/bin/bash

BUILD_DIR=./build/
TARGET=all

while getopts "b:t:" option; do
  case $option in
  b) BUILD_DIR=$OPTARG ;;
  t) TARGET=$OPTARG ;;
  ?)
    echo "Error: invalid option"
    exit 1
    ;;
  esac
done

if [ ! -d "$BUILD_DIR" ]; then
  mkdir "$BUILD_DIR"
  if [ ! -d "$BUILD_DIR" ]; then
    echo "Error: could not create folder '$BUILD_DIR'"
    exit 1
  fi
fi

cmake -S . -B "$BUILD_DIR"
cmake --build "$BUILD_DIR" --target "$TARGET"
