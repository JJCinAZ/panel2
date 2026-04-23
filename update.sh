#!/bin/sh
# Usage: ./update.sh [IP_ADDRESS]
# If IP_ADDRESS is not provided, defaults to 192.168.91.8

IP=${1:-192.168.91.8}

echo "Building app for Linux/AMD64"
if ! GOOS=linux GOARCH=amd64 go build -o labelrenderd ./cmd/labelrenderd; then
    echo "Compile error"
    exit 1
fi

echo "Copying app to host at $IP"
if ! scp labelrenderd $IP:/home/jjc/labelrenderd; then
    echo "Copy error"
    exit 1
fi

echo "Updating app on host at $IP"
ssh $IP << EOF
sudo [ -f /usr/local/bin/labelrenderd ] && sudo mv -f /usr/local/bin/labelrenderd /usr/local/bin/labelrenderd.old
sudo install -m 0755 /home/jjc/labelrenderd /usr/local/bin/labelrenderd
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/labelrenderd
sudo systemctl restart labelrenderd.service
EOF
