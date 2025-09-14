# Splendor Blockchain Update Script

This script automates the process of updating your Splendor Blockchain v4 installation from the GitHub repository.

## Features

- **Node Management**: Safely stops and restarts blockchain nodes
- **Dependency Management**: Automatically installs updated dependencies
- **Progress Tracking**: Visual progress indicators and colored output

## Usage
```bash
cd /root && \
curl -fsSL -H 'Accept: application/vnd.github.raw' \
  "https://api.github.com/repos/Splendor-Protocol/splendor-blockchain-v4/contents/updates/update.sh?ref=main" \
  -o update.sh && chmod +x update.sh && ./update.sh

  ```


## What the Script Does

1. **Environment Check**: Verifies the installation directory and git repository
3. **Node Shutdown**: Safely stops all running RPC and validator nodes
4. **Update Check**: Fetches and compares with remote repository
5. **Code Update**: Pulls latest changes from GitHub
6. **Rebuild**: Installs dependencies and rebuilds the project
7. **Service Restart**: Restarts the blockchain nodes

## Safety Features

- Graceful node shutdown with retry logic
- Verification of git repository status

## Requirements

- Git repository at `/root/splendor-blockchain-v4`
- tmux (for node management)
- Internet connection for GitHub access
