#!/bin/bash
# Claude-Squad Auto-Update Script

set -e

REPO_URL="https://github.com/jimprince/claude-squad.git"
BUILD_DIR="/opt/claude-squad"
BINARY_PATH="/usr/local/bin/cs"
BACKUP_PATH="/usr/local/bin/cs.backup"

echo "🔄 Checking for claude-squad updates..."

# Navigate to repo directory
cd "$BUILD_DIR"

# Fetch latest changes
git fetch origin

# Get current and latest versions
CURRENT_VERSION=$(git rev-parse HEAD)
LATEST_VERSION=$(git rev-parse origin/main)

if [ "$CURRENT_VERSION" = "$LATEST_VERSION" ]; then
    echo "✅ Claude-squad is up to date ($(git log -1 --format='%h - %s'))"
    exit 0
fi

echo "📦 New version available!"
echo "Current: $(git log -1 --format='%h - %s' $CURRENT_VERSION)"
echo "Latest:  $(git log -1 --format='%h - %s' $LATEST_VERSION)"

# Backup current binary
if [ -f "$BINARY_PATH" ]; then
    echo "💾 Backing up current binary..."
    cp "$BINARY_PATH" "$BACKUP_PATH"
fi

# Pull latest changes
echo "⬇️  Pulling latest changes..."
git pull origin main

# Build new binary
echo "🔨 Building new binary..."
go build -o cs

# Test the new binary
echo "🧪 Testing new binary..."
if ./cs --version > /dev/null 2>&1; then
    echo "✅ New binary works!"
    
    # Install new binary
    echo "📦 Installing new binary..."
    sudo cp cs "$BINARY_PATH"
    sudo chmod +x "$BINARY_PATH"
    
    echo "🎉 Claude-squad updated successfully!"
    echo "New version: $(git log -1 --format='%h - %s')"
else
    echo "❌ New binary failed tests!"
    if [ -f "$BACKUP_PATH" ]; then
        echo "🔄 Restoring backup..."
        sudo cp "$BACKUP_PATH" "$BINARY_PATH"
    fi
    exit 1
fi

# Cleanup
rm -f "$BACKUP_PATH"

# Optional: Restart any running claude-squad sessions
echo "🔄 Checking for running sessions..."
if tmux list-sessions 2>/dev/null | grep -q .; then
    echo "⚠️  Active tmux sessions detected. Consider restarting them to use the new version."
    echo "   Use: tmux kill-server (to restart all sessions)"
    echo "   Or restart individual sessions as needed."
fi

echo "✅ Update complete!"