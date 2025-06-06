# Claude-Squad Remote Docker Container Requirements

## Overview
I need a Docker container that runs claude-squad sessions persistently on a remote server. I'll connect via SSH from my laptop to manage sessions that continue running even when my laptop is closed.

## Container Requirements

### 1. Base Software
- Ubuntu 22.04 base image
- OpenSSH Server
- Tmux (version 3.0+)
- Git (with git-lfs support)
- Claude CLI (from https://storage.googleapis.com/anthropic-sdk/claude-cli/install.sh)
- claude-squad binary (I'll provide this - filename: `cs`)

### 2. User Setup
- Create user: `claudeuser` with home directory `/home/claudeuser`
- User should have sudo access (for package installation if needed)
- SSH access via key authentication (NO password auth)

### 3. Directory Structure
```
/home/claudeuser/
├── .ssh/
│   └── authorized_keys  (for SSH key authentication)
├── repos/               (git repositories)
├── .claude/             (Claude CLI data)
└── .claude-squad/       (claude-squad worktrees and state)
```

### 4. SSH Configuration
- SSH port: 2222 (or your preference)
- Disable password authentication
- Enable key authentication only
- Configure for persistent connections

### 5. Volumes Needed
```yaml
volumes:
  - ./repos:/home/claudeuser/repos
  - claude-data:/home/claudeuser/.claude  
  - claude-squad-data:/home/claudeuser/.claude-squad
  - ./authorized_keys:/home/claudeuser/.ssh/authorized_keys:ro
```

### 6. Environment Variables
- `ANTHROPIC_API_KEY` - Will be provided via environment
- Standard git config (user.name, user.email)

### 7. Container Features
- Auto-restart on failure
- Health check via SSH connectivity
- Log rotation for claude-squad logs
- Persistent tmux sessions that survive container restart

## Deliverables Needed

### 1. Dockerfile
Complete Dockerfile with all dependencies and configuration

### 2. docker-compose.yml
With proper volume mounts, port exposure, and restart policies

### 3. Setup Instructions
- How to build and start the container
- How to add SSH keys to authorized_keys
- How to provide the ANTHROPIC_API_KEY securely
- How to add the claude-squad binary

### 4. Maintenance Procedures
- How to backup/restore session state
- How to view container logs
- How to update the claude-squad binary
- Recommended backup schedule

## SSH Key Setup
After you create the container, I'll run a setup script that generates an SSH key. I'll send you the public key to add to the container's authorized_keys file.

## Testing
Once setup is complete, I should be able to:
1. `ssh -p 2222 claudeuser@your-server` - Connect without password
2. `tmux list-sessions` - See active sessions
3. `cs` - Run claude-squad

## Questions?
Please let me know if you need:
- The claude-squad binary
- Clarification on any requirements
- Different port numbers or user names
- Additional software packages