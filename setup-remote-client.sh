#!/bin/bash
# Claude-Squad Remote Client Setup Script

set -e

echo "🚀 Claude-Squad Remote Client Setup"
echo "=================================="

# Get remote host details
read -p "Enter remote host (IP or hostname): " REMOTE_HOST
read -p "Enter SSH port (default 2222): " SSH_PORT
SSH_PORT=${SSH_PORT:-2222}
read -p "Enter remote username (default claudeuser): " REMOTE_USER
REMOTE_USER=${REMOTE_USER:-claudeuser}

# Generate SSH key if it doesn't exist
SSH_KEY="$HOME/.ssh/claude-squad-remote"
if [ ! -f "$SSH_KEY" ]; then
    echo "📝 Generating SSH key for claude-squad..."
    ssh-keygen -t ed25519 -f "$SSH_KEY" -C "claude-squad-remote" -N ""
else
    echo "✅ SSH key already exists at $SSH_KEY"
fi

# Create SSH config entry
SSH_CONFIG="$HOME/.ssh/config"
echo "📝 Adding SSH config entry..."

# Backup existing config
cp "$SSH_CONFIG" "$SSH_CONFIG.backup.$(date +%Y%m%d%H%M%S)" 2>/dev/null || true

# Check if entry already exists
if grep -q "Host claude-remote" "$SSH_CONFIG" 2>/dev/null; then
    echo "⚠️  SSH config entry 'claude-remote' already exists. Updating..."
    # Remove old entry
    sed -i '' '/Host claude-remote/,/^$/d' "$SSH_CONFIG" 2>/dev/null || \
    sed -i '/Host claude-remote/,/^$/d' "$SSH_CONFIG"
fi

# Add new entry
cat >> "$SSH_CONFIG" << EOF

Host claude-remote
    HostName $REMOTE_HOST
    Port $SSH_PORT
    User $REMOTE_USER
    IdentityFile $SSH_KEY
    ForwardAgent yes
    ServerAliveInterval 60
    ServerAliveCountMax 3
    TCPKeepAlive yes
    Compression yes
    ControlMaster auto
    ControlPath ~/.ssh/claude-squad-%r@%h:%p
    ControlPersist 10m
EOF

echo "✅ SSH config updated"

# Create shell aliases
echo "📝 Creating shell aliases..."

# Detect shell
if [ -n "$ZSH_VERSION" ]; then
    SHELL_RC="$HOME/.zshrc"
elif [ -n "$BASH_VERSION" ]; then
    SHELL_RC="$HOME/.bashrc"
else
    SHELL_RC="$HOME/.bashrc"
fi

# Check if aliases already exist
if grep -q "claude-squad remote aliases" "$SHELL_RC" 2>/dev/null; then
    echo "✅ Aliases already exist in $SHELL_RC"
else
    cat >> "$SHELL_RC" << 'EOF'

# claude-squad remote aliases
alias cs-remote='ssh -t claude-remote "cd repos && cs"'
alias cs-attach='ssh -t claude-remote "tmux attach"'
alias cs-list='ssh claude-remote "tmux list-sessions"'
alias cs-sessions='ssh claude-remote "cs list"'
alias cs-logs='ssh claude-remote "tail -f /tmp/claudesquad.log"'

# Advanced aliases
cs-session() {
    if [ -z "$1" ]; then
        echo "Usage: cs-session <session-name>"
        return 1
    fi
    ssh -t claude-remote "tmux attach -t $1"
}

cs-new() {
    ssh -t claude-remote "cd repos/${1:-.} && cs"
}

cs-kill() {
    if [ -z "$1" ]; then
        echo "Usage: cs-kill <session-name>"
        return 1
    fi
    ssh claude-remote "tmux kill-session -t $1"
}
EOF
    echo "✅ Aliases added to $SHELL_RC"
fi

# Display SSH public key
echo ""
echo "📋 SSH Public Key (send this to your backend developer):"
echo "======================================================="
cat "$SSH_KEY.pub"
echo "======================================================="
echo ""

# Create instructions for backend
cat > remote-setup-instructions.txt << EOF
Claude-Squad Remote Server Setup Instructions
============================================

Please add the following SSH public key to the Docker container:

$(cat "$SSH_KEY.pub")

The key should be added to:
/home/$REMOTE_USER/.ssh/authorized_keys

Make sure the permissions are correct:
chmod 700 /home/$REMOTE_USER/.ssh
chmod 600 /home/$REMOTE_USER/.ssh/authorized_keys
chown -R $REMOTE_USER:$REMOTE_USER /home/$REMOTE_USER/.ssh

The Docker container should expose SSH on port $SSH_PORT.

Please confirm when this is complete so we can test the connection.
EOF

echo "📄 Instructions saved to: remote-setup-instructions.txt"
echo ""
echo "🎯 Next Steps:"
echo "1. Send 'remote-setup-instructions.txt' to your backend developer"
echo "2. Once they confirm setup, test with: ssh claude-remote"
echo "3. Source your shell config: source $SHELL_RC"
echo "4. Use 'cs-remote' to start claude-squad on the remote server"
echo ""
echo "🛠️  Available commands after setup:"
echo "  cs-remote     - Start claude-squad on remote"
echo "  cs-attach     - Attach to remote tmux session"
echo "  cs-list       - List remote tmux sessions"
echo "  cs-sessions   - List claude-squad sessions"
echo "  cs-logs       - View remote logs"
echo "  cs-session <name> - Attach to specific session"
echo "  cs-new [dir]  - Start new session in specific repo"
echo "  cs-kill <name> - Kill specific session"