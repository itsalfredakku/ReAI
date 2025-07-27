#!/bin/bash

# ReAI Deployment Script with Persistent Storage
set -e

echo "🚀 ReAI Deployment Script"
echo "=========================="

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    aarch64|arm64|armv7l|armv6l)
        echo "📱 Detected ARM architecture: $ARCH"
        COMPOSE_FILE="docker-compose.arm.yml"
        ;;
    x86_64|amd64)
        echo "💻 Detected x86_64 architecture: $ARCH"
        COMPOSE_FILE="docker-compose.yml"
        export DOCKERFILE=Dockerfile
        ;;
    *)
        echo "❓ Unknown architecture: $ARCH, using standard setup"
        COMPOSE_FILE="docker-compose.yml"
        export DOCKERFILE=Dockerfile
        ;;
esac

echo "📁 Using compose file: $COMPOSE_FILE"

# Function to handle deployment
deploy() {
    echo "🔨 Building and starting ReAI container..."
    
    # Stop existing container if running
    sudo docker-compose -f $COMPOSE_FILE down --remove-orphans || true
    
    # Build and start with persistent volumes
    sudo docker-compose -f $COMPOSE_FILE up -d --build
    
    echo "✅ ReAI container started successfully!"
    echo "🌐 Server will be available at: http://localhost:50080"
    echo "📊 Health check: http://localhost:50080/health"
    echo ""
    echo "📝 To view logs:"
    echo "   sudo docker-compose -f $COMPOSE_FILE logs -f"
    echo ""
    echo "🔑 Authentication token will be preserved in the 'reai_data' volume"
}

# Function to show logs
logs() {
    echo "📄 Showing ReAI logs..."
    sudo docker-compose -f $COMPOSE_FILE logs -f
}

# Function to stop the service
stop() {
    echo "🛑 Stopping ReAI container..."
    sudo docker-compose -f $COMPOSE_FILE down
    echo "✅ ReAI container stopped"
}

# Function to restart the service
restart() {
    echo "🔄 Restarting ReAI container..."
    stop
    deploy
}

# Function to show status
status() {
    echo "📊 ReAI Container Status:"
    sudo docker-compose -f $COMPOSE_FILE ps
    echo ""
    echo "💾 Volume Status:"
    sudo docker volume ls | grep reai_data || echo "Volume not found"
}

# Parse command line arguments
case "${1:-deploy}" in
    deploy)
        deploy
        ;;
    logs)
        logs
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    *)
        echo "Usage: $0 {deploy|logs|stop|restart|status}"
        echo ""
        echo "Commands:"
        echo "  deploy  - Build and start the ReAI container (default)"
        echo "  logs    - Show container logs"
        echo "  stop    - Stop the container"
        echo "  restart - Restart the container"
        echo "  status  - Show container and volume status"
        exit 1
        ;;
esac
