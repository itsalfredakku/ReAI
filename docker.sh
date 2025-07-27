#!/bin/bash

# ReAI Docker Management Scripts

set -e

CONTAINER_NAME="reai"
IMAGE_NAME="reai"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Help function
show_help() {
    echo "ReAI Docker Management"
    echo ""
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  build       Build the Docker image"
    echo "  start       Start the application using docker-compose"
    echo "  stop        Stop the application"
    echo "  restart     Restart the application"
    echo "  logs        Show application logs"
    echo "  shell       Open a shell in the running container"
    echo "  clean       Remove containers and images"
    echo "  status      Show container status"
    echo "  health      Check application health"
    echo "  help        Show this help message"
    echo ""
}

# Build the Docker image
build() {
    print_status "Building Docker image..."
    docker-compose build
    print_success "Docker image built successfully!"
}

# Start the application
start() {
    print_status "Starting ReAI..."
    docker-compose up -d
    print_success "ReAI started!"
    print_status "Application will be available at http://localhost:8080"
    print_status "Use './docker.sh logs' to view logs"
}

# Stop the application
stop() {
    print_status "Stopping ReAI..."
    docker-compose down
    print_success "ReAI stopped!"
}

# Restart the application
restart() {
    print_status "Restarting ReAI..."
    docker-compose restart
    print_success "ReAI restarted!"
}

# Show logs
logs() {
    print_status "Showing ReAI logs (press Ctrl+C to exit)..."
    docker-compose logs -f reai
}

# Open shell in container
shell() {
    print_status "Opening shell in ReAI container..."
    docker-compose exec reai /bin/sh
}

# Clean up containers and images
clean() {
    print_warning "This will remove all containers, images, and volumes for ReAI"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_status "Stopping and removing containers..."
        docker-compose down -v --rmi all
        print_success "Cleanup completed!"
    else
        print_status "Cleanup cancelled"
    fi
}

# Show container status
status() {
    print_status "Container status:"
    docker-compose ps
}

# Health check
health() {
    print_status "Checking application health..."
    if curl -s http://localhost:8080/health > /dev/null; then
        print_success "Application is healthy!"
        echo "Available endpoints:"
        echo "  - Health check: http://localhost:8080/health"
        echo "  - Models: http://localhost:8080/v1/models"
        echo "  - Completions: http://localhost:8080/v1/completions"
        echo "  - Chat: http://localhost:8080/v1/chat/completions"
    else
        print_error "Application is not responding or not running"
        print_status "Try running: $0 start"
        exit 1
    fi
}

# Main script logic
case "$1" in
    build)
        build
        ;;
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    logs)
        logs
        ;;
    shell)
        shell
        ;;
    clean)
        clean
        ;;
    status)
        status
        ;;
    health)
        health
        ;;
    help|--help|-h)
        show_help
        ;;
    "")
        print_error "No command specified"
        show_help
        exit 1
        ;;
    *)
        print_error "Unknown command: $1"
        show_help
        exit 1
        ;;
esac
