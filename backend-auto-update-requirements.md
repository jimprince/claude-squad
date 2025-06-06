# Claude-Squad Auto-Update Docker Setup

## Overview
Modify the Docker container to automatically clone, build, and update claude-squad from GitHub instead of copying a pre-built binary.

## Required Changes to Dockerfile

### 1. Add Build Dependencies
```dockerfile
# Add to existing package installation
RUN apt-get update && apt-get install -y \
    git \
    tmux \
    openssh-server \
    curl \
    sudo \
    golang-go \
    build-essential \
    cron \
    && rm -rf /var/lib/apt/lists/*
```

### 2. Clone and Build from Source
```dockerfile
# Replace the COPY cs /usr/local/bin/cs line with:
WORKDIR /opt
RUN git clone https://github.com/YOUR_USERNAME/claude-squad.git
WORKDIR /opt/claude-squad
RUN go build -o cs && \
    cp cs /usr/local/bin/cs && \
    chmod +x /usr/local/bin/cs
```

### 3. Add Update Script
```dockerfile
# Copy the update script
COPY update-claude-squad.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/update-claude-squad.sh

# Fix permissions for sudo
RUN echo "claudeuser ALL=(ALL) NOPASSWD: /usr/local/bin/update-claude-squad.sh, /bin/cp, /bin/chmod" >> /etc/sudoers
```

### 4. Enable Cron (Optional)
```dockerfile
# Add cron job for automatic updates
RUN echo "0 * * * * claudeuser /usr/local/bin/update-claude-squad.sh >> /var/log/claude-squad-updates.log 2>&1" >> /etc/crontab
```

## Files Needed

1. **update-claude-squad.sh** - Auto-update script (attached)
2. **Modified Dockerfile** - With build dependencies
3. **docker-compose.yml updates** - Add log volume

### Docker Compose Addition
```yaml
volumes:
  - ./logs:/var/log  # For update logs
```

## GitHub Repository Setup

You'll need to:
1. Push claude-squad to a public GitHub repository
2. Update the REPO_URL in update-claude-squad.sh
3. Consider using GitHub releases for stable versions

## Testing

After building:
1. SSH into container: `ssh claude-remote`
2. Test update: `sudo /usr/local/bin/update-claude-squad.sh`
3. Verify: `cs --version`

## Manual Update Command

Once set up, updates can be triggered via:
```bash
ssh claude-remote "sudo /usr/local/bin/update-claude-squad.sh"
```

## Benefits

- ✅ Always up-to-date with latest features
- ✅ Automatic security updates
- ✅ No need to manually copy binaries
- ✅ Version history and rollback capability
- ✅ Can be automated or triggered manually

Let me know when this is implemented and I'll test the auto-update functionality!