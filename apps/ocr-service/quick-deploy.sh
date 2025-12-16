#!/bin/bash

# Quick Deploy Script - One Command Deployment
# Usage: curl -sSL https://your-repo/quick-deploy.sh | sudo bash

set -e

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Study in Woods - OCR Service Quick Deploy            ║"
echo "║  For 4GB VPS Ubuntu 22.04                              ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Check root
if [ "$EUID" -ne 0 ]; then 
    echo "❌ Please run as root: sudo bash quick-deploy.sh"
    exit 1
fi

# Check Ubuntu version
if ! grep -q "Ubuntu 22.04" /etc/os-release; then
    echo "⚠️  Warning: This script is designed for Ubuntu 22.04"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo "📊 Current System Resources:"
free -h
df -h /
echo ""

# Step 1: System Update
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 Step 1/7: Updating system packages..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -qq

# Step 2: Install Dependencies
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 Step 2/7: Installing dependencies..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
apt-get install -y -qq \
    software-properties-common \
    build-essential \
    curl wget git vim \
    nginx \
    libpoppler-cpp-dev poppler-utils \
    tesseract-ocr libtesseract-dev \
    htop net-tools jq

# Install Python 3.11
add-apt-repository -y ppa:deadsnakes/ppa
apt-get update -qq
apt-get install -y -qq python3.11 python3.11-venv python3.11-dev python3-pip

echo "✅ Dependencies installed"

# Step 3: Setup Swap
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "💾 Step 3/7: Setting up swap file..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ ! -f /swapfile ]; then
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    sysctl vm.swappiness=60
    echo "vm.swappiness=60" >> /etc/sysctl.conf
    echo "✅ 2GB swap file created"
else
    echo "✅ Swap file already exists"
fi

# Step 4: OCR Service Deployment
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🤖 Step 4/7: Deploying OCR Service..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if we have the source files
if [ ! -f "main.py" ]; then
    echo "⚠️  OCR service files not found in current directory"
    echo "Please run this script from apps/ocr-service/ directory"
    echo ""
    read -p "Enter path to OCR service directory: " OCR_SOURCE
    if [ ! -d "$OCR_SOURCE" ]; then
        echo "❌ Directory not found: $OCR_SOURCE"
        exit 1
    fi
    cd "$OCR_SOURCE"
fi

# Run main deployment script
if [ -f "deploy-vps.sh" ]; then
    bash deploy-vps.sh
else
    echo "❌ deploy-vps.sh not found"
    exit 1
fi

# Step 5: Configure Firewall
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔒 Step 5/7: Configuring firewall..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if ! ufw status | grep -q "Status: active"; then
    ufw --force enable
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow OpenSSH
    ufw allow 'Nginx Full'
    echo "✅ Firewall configured"
else
    echo "✅ Firewall already configured"
fi

# Step 6: System Optimization
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "⚡ Step 6/7: Optimizing system..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Increase file descriptors
cat >> /etc/security/limits.conf << LIMITS
* soft nofile 65536
* hard nofile 65536
www-data soft nofile 65536
www-data hard nofile 65536
LIMITS

# Kernel network tuning
cat >> /etc/sysctl.conf << SYSCTL
# Network optimization
net.core.somaxconn = 1024
net.ipv4.tcp_max_syn_backlog = 2048
net.ipv4.tcp_fin_timeout = 30
net.ipv4.ip_local_port_range = 10000 65000
SYSCTL

sysctl -p > /dev/null 2>&1

echo "✅ System optimized"

# Step 7: Final Checks
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔍 Step 7/7: Running health checks..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

sleep 3

# Check service status
echo -n "OCR Service:   "
if systemctl is-active --quiet ocr-service; then
    echo "✅ Running"
else
    echo "❌ Not running"
fi

echo -n "Nginx:         "
if systemctl is-active --quiet nginx; then
    echo "✅ Running"
else
    echo "❌ Not running"
fi

# Check health endpoint
echo -n "Health Check:  "
if curl -sf http://127.0.0.1:8080/health > /dev/null; then
    echo "✅ Healthy"
else
    echo "❌ Failed"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Current Resource Usage:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
free -h | awk 'NR==1{print "             "$0} NR==2{print "System:      "$0} NR==3{print "Swap:        "$0}'
echo ""

echo "╔════════════════════════════════════════════════════════╗"
echo "║                  🎉 DEPLOYMENT COMPLETE!               ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "📝 Next Steps:"
echo ""
echo "1️⃣  Configure credentials:"
echo "   sudo nano /opt/ocr-service/.env"
echo ""
echo "2️⃣  Update Nginx domain:"
echo "   sudo nano /etc/nginx/sites-available/ocr-service"
echo ""
echo "3️⃣  Restart services:"
echo "   sudo systemctl restart ocr-service nginx"
echo ""
echo "4️⃣  Monitor resources:"
echo "   /opt/ocr-service/monitor-resources.sh"
echo ""
echo "5️⃣  View logs:"
echo "   sudo journalctl -u ocr-service -f"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📚 Documentation:"
echo "   • Deployment: /opt/ocr-service/VPS_DEPLOYMENT_GUIDE.md"
echo "   • Troubleshooting: /opt/ocr-service/TROUBLESHOOTING.md"
echo ""
echo "🔧 Quick Commands:"
echo "   • Test OCR: curl http://127.0.0.1:8080/health"
echo "   • Check status: systemctl status ocr-service"
echo "   • View logs: journalctl -u ocr-service -f"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
