#!/bin/bash

# Auto-detect architecture and use appropriate docker-compose file
ARCH=$(uname -m)

case $ARCH in
    aarch64|arm64|armv7l|armv6l)
        echo "Detected ARM architecture: $ARCH"
        echo "Using ARM-specific Dockerfile..."
        export DOCKERFILE=Dockerfile.arm
        docker-compose -f docker-compose.arm.yml "$@"
        ;;
    x86_64|amd64)
        echo "Detected x86_64 architecture: $ARCH"
        echo "Using standard Dockerfile..."
        export DOCKERFILE=Dockerfile
        docker-compose "$@"
        ;;
    *)
        echo "Unknown architecture: $ARCH"
        echo "Falling back to standard Dockerfile..."
        export DOCKERFILE=Dockerfile
        docker-compose "$@"
        ;;
esac
