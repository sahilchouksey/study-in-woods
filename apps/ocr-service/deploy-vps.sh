#!/bin/bash
set -e

# VPS Deployment Script for Docling OCR Service
# Usage: sudo bash deploy-vps.sh

echo "=========================================="
echo "  Docling OCR Service VPS Deployment"
echo "=========================================="

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "❌ Please run as root (use sudo)"
    exit 1
fi

# Configuration
OCR_DIR="/opt/ocr-service"
LOG_DIR="/var/log/ocr-service"
SERVICE_USER="www-data"
PYTHON_VERSION="3.11"

echo ""
echo "📦 Step 1: Installing system dependencies..."
apt-get update
apt-get install -y \
    python3.11 \
    python3.11-venv \
    python3.11-dev \
    python3-pip \
    build-essential \
    nginx \
    curl \
    git \
    libpoppler-cpp-dev \
    poppler-utils \
    tesseract-ocr \
    libtesseract-dev

echo ""
echo "📁 Step 2: Creating directories..."
mkdir -p $OCR_DIR
mkdir -p $LOG_DIR
chown -R $SERVICE_USER:$SERVICE_USER $LOG_DIR

echo ""
echo "📥 Step 3: Copying application files..."
# Assumes script is run from apps/ocr-service directory
if [ ! -f "main.py" ]; then
    echo "❌ Error: main.py not found. Run this script from apps/ocr-service/"
    exit 1
fi

cp -r . $OCR_DIR/
cd $OCR_DIR

echo ""
echo "🐍 Step 4: Setting up Python virtual environment..."
python3.11 -m venv venv
source venv/bin/activate

echo ""
echo "📦 Step 5: Installing Python dependencies..."
pip install --upgrade pip setuptools wheel
pip install -r requirements.txt

# Pre-download Docling models (optional but recommended)
echo ""
echo "📥 Step 6: Pre-downloading Docling models (this may take a few minutes)..."
python3 -c "from docling.document_converter import DocumentConverter; DocumentConverter()" || true

echo ""
echo "🔐 Step 7: Setting up environment file..."
if [ ! -f "$OCR_DIR/.env" ]; then
    cat > $OCR_DIR/.env << 'ENVEOF'
# DigitalOcean Spaces credentials
SPACES_KEY=your-spaces-key
SPACES_SECRET=your-spaces-secret
SPACES_BUCKET=study-in-woods
SPACES_REGION=blr1
SPACES_ENDPOINT=https://blr1.digitaloceanspaces.com

# Service configuration
OCR_API_KEY=
WEBHOOK_SECRET=
WEBHOOK_URL=http://127.0.0.1:8000/api/webhooks/ocr
OCR_DEVICE=cpu
PORT=8080
ENVEOF
    echo "⚠️  Created .env file - PLEASE UPDATE WITH YOUR CREDENTIALS!"
else
    echo "✅ .env file already exists, skipping..."
fi

echo ""
echo "🔧 Step 8: Setting permissions..."
chown -R $SERVICE_USER:$SERVICE_USER $OCR_DIR

echo ""
echo "📋 Step 9: Installing systemd service..."
cp ocr-service.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable ocr-service
systemctl start ocr-service

echo ""
echo "⏳ Waiting for service to start..."
sleep 5

echo ""
echo "🔍 Step 10: Checking service status..."
systemctl status ocr-service --no-pager || true

echo ""
echo "🌐 Step 11: Configuring Nginx..."
cp nginx-ocr.conf /etc/nginx/sites-available/ocr-service
ln -sf /etc/nginx/sites-available/ocr-service /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx

echo ""
echo "📊 Step 12: Setting up log rotation..."
cat > /etc/logrotate.d/ocr-service << 'LOGROTATE'
/var/log/ocr-service/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 www-data www-data
    sharedscripts
    postrotate
        systemctl reload ocr-service > /dev/null 2>&1 || true
    endscript
}
LOGROTATE

echo ""
echo "💾 Step 13: Creating swap file (2GB for stability)..."
if [ ! -f /swapfile ]; then
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    echo "✅ Swap file created"
else
    echo "✅ Swap file already exists"
fi

echo ""
echo "🧪 Step 14: Testing OCR service..."
sleep 3
curl -s http://127.0.0.1:8080/health | python3 -m json.tool || echo "❌ Health check failed"

echo ""
echo "=========================================="
echo "  ✅ Deployment Complete!"
echo "=========================================="
echo ""
echo "📝 Next Steps:"
echo "1. Update .env file: nano /opt/ocr-service/.env"
echo "2. Update Nginx config: nano /etc/nginx/sites-available/ocr-service"
echo "3. Restart service: systemctl restart ocr-service"
echo ""
echo "🔧 Useful Commands:"
echo "  • View logs:    journalctl -u ocr-service -f"
echo "  • Check status: systemctl status ocr-service"
echo "  • Restart:      systemctl restart ocr-service"
echo "  • Test health:  curl http://127.0.0.1:8080/health"
echo ""
echo "📊 Resource Usage:"
free -h
echo ""
