# VPS Deployment Guide - Complete Stack

Complete guide to deploy Go API, Next.js frontend, and OCR service on a single 4GB VPS.

## Architecture Overview

```
Internet → Nginx (Port 80/443)
           ├─> Next.js (Port 3000) - Frontend
           ├─> Go API (Port 8000) - Backend
           └─> OCR Service (Port 8080) - Internal only
```

**Internal Communication:**
- Go API calls OCR: `http://127.0.0.1:8080/process`
- OCR webhooks to Go API: `http://127.0.0.1:8000/api/webhooks/ocr`

## Prerequisites

### 1. VPS Requirements
- **RAM:** 4GB minimum
- **Storage:** 25GB SSD
- **OS:** Ubuntu 22.04 LTS
- **Provider:** DigitalOcean, Linode, Vultr, etc.

### 2. Domain Setup (Optional)
- Point your domain to VPS IP
- Wait for DNS propagation

### 3. Initial VPS Setup

```bash
# SSH into your VPS
ssh root@your-vps-ip

# Update system
apt-get update && apt-get upgrade -y

# Create swap file (important for 4GB RAM)
fallocate -l 2G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab

# Install base utilities
apt-get install -y curl git vim htop ufw

# Setup firewall
ufw allow OpenSSH
ufw allow 'Nginx Full'
ufw enable
```

## Resource Allocation Plan

| Service      | RAM Target | Max RAM | CPU Quota | Priority |
|--------------|-----------|---------|-----------|----------|
| OCR Service  | 600-800MB | 800MB   | 50%       | Medium   |
| Go API       | 80-120MB  | 200MB   | 30%       | High     |
| Next.js      | 250-350MB | 400MB   | 40%       | High     |
| PostgreSQL   | 100-150MB | 200MB   | 20%       | High     |
| Redis        | 40-60MB   | 100MB   | 10%       | Medium   |
| Nginx        | 10-20MB   | 50MB    | 5%        | High     |
| **Total**    | **~1.4GB**| **~2GB**| **155%**  | -        |

**Safety Margins:**
- Base system: ~300MB
- Available for processes: ~3.7GB
- Swap usage: Emergency buffer
- Peak usage: ~2GB (normal), ~3GB (heavy load)

## Deployment Steps

### Step 1: Deploy PostgreSQL & Redis

```bash
# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh

# Create network
docker network create app-network

# Deploy PostgreSQL
docker run -d \
  --name postgres \
  --network app-network \
  --restart unless-stopped \
  -e POSTGRES_DB=study_in_woods \
  -e POSTGRES_USER=dbuser \
  -e POSTGRES_PASSWORD=your_secure_password \
  -p 127.0.0.1:5432:5432 \
  -v postgres-data:/var/lib/postgresql/data \
  --memory="200m" \
  --cpus="0.5" \
  postgres:15-alpine

# Deploy Redis
docker run -d \
  --name redis \
  --network app-network \
  --restart unless-stopped \
  -p 127.0.0.1:6379:6379 \
  -v redis-data:/data \
  --memory="100m" \
  --cpus="0.3" \
  redis:7-alpine redis-server --maxmemory 80mb --maxmemory-policy allkeys-lru
```

### Step 2: Deploy Go API

```bash
# Install Go 1.21
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
source /etc/profile

# Create app directory
mkdir -p /opt/go-api
cd /opt/go-api

# Upload your Go API code (from local machine)
# scp -r apps/api/* root@vps:/opt/go-api/

# Build
go build -o api main.go

# Create .env file
nano /opt/go-api/.env
# Add database, Redis, Spaces credentials

# Create systemd service
cat > /etc/systemd/system/go-api.service << 'SERVICE'
[Unit]
Description=Go API Service
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/go-api
EnvironmentFile=/opt/go-api/.env
Environment="PORT=8000"
ExecStart=/opt/go-api/api
Restart=on-failure
RestartSec=5s

# Resource limits
MemoryMax=200M
CPUQuota=30%

[Install]
WantedBy=multi-user.target
SERVICE

# Fix permissions
chown -R www-data:www-data /opt/go-api

# Start service
systemctl daemon-reload
systemctl enable go-api
systemctl start go-api
systemctl status go-api
```

### Step 3: Deploy OCR Service

```bash
# Navigate to OCR directory on your local machine
cd apps/ocr-service

# Upload files to VPS
scp -r . root@vps:/tmp/ocr-service/

# On VPS, run deployment script
ssh root@vps
cd /tmp/ocr-service
bash deploy-vps.sh

# Edit environment file with your credentials
nano /opt/ocr-service/.env

# Update these values:
# SPACES_KEY=your-actual-key
# SPACES_SECRET=your-actual-secret
# WEBHOOK_URL=http://127.0.0.1:8000/api/webhooks/ocr

# Restart service
systemctl restart ocr-service
systemctl status ocr-service

# Test
curl http://127.0.0.1:8080/health | jq
```

### Step 4: Deploy Next.js Frontend

```bash
# Install Node.js 20 LTS
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt-get install -y nodejs

# Install PM2 globally
npm install -g pm2

# Create app directory
mkdir -p /opt/nextjs-app
cd /opt/nextjs-app

# Upload Next.js code (from local machine)
# scp -r apps/web/* root@vps:/opt/nextjs-app/

# Install dependencies
npm install --production

# Build production bundle
npm run build

# Create ecosystem file for PM2
cat > ecosystem.config.js << 'ECOSYSTEM'
module.exports = {
  apps: [{
    name: 'nextjs-app',
    script: 'npm',
    args: 'start',
    instances: 1,
    exec_mode: 'cluster',
    env: {
      NODE_ENV: 'production',
      PORT: 3000
    },
    max_memory_restart: '400M',
    error_file: '/var/log/nextjs/error.log',
    out_file: '/var/log/nextjs/out.log',
    log_date_format: 'YYYY-MM-DD HH:mm:ss Z'
  }]
}
ECOSYSTEM

# Create log directory
mkdir -p /var/log/nextjs

# Start with PM2
pm2 start ecosystem.config.js
pm2 save
pm2 startup systemd

# Verify
pm2 status
curl http://127.0.0.1:3000
```

### Step 5: Configure Nginx

```bash
# Install Nginx
apt-get install -y nginx

# Copy OCR Nginx config
cp /opt/ocr-service/nginx-ocr.conf /etc/nginx/sites-available/app

# Update server_name with your domain
nano /etc/nginx/sites-available/app
# Change: server_name your-domain.com;

# Enable site
ln -s /etc/nginx/sites-available/app /etc/nginx/sites-enabled/
rm /etc/nginx/sites-enabled/default

# Test configuration
nginx -t

# Reload Nginx
systemctl reload nginx

# Test from outside
curl http://your-vps-ip
```

### Step 6: Setup SSL (Optional but Recommended)

```bash
# Install Certbot
apt-get install -y certbot python3-certbot-nginx

# Get certificate (replace with your domain)
certbot --nginx -d your-domain.com -d www.your-domain.com

# Auto-renewal is configured automatically
# Test renewal
certbot renew --dry-run
```

## Verification Checklist

After deployment, verify everything works:

```bash
# 1. Check all services are running
systemctl status go-api
systemctl status ocr-service
systemctl status nginx
pm2 status
docker ps

# 2. Check memory usage
free -h
# Should have at least 1GB free

# 3. Test internal communication
# From Go API to OCR
curl -X POST http://127.0.0.1:8080/process \
  -H "Content-Type: application/json" \
  -d '{"pdf_key": "test.pdf", "document_id": "test"}'

# 4. Test external access
curl http://your-domain.com
curl http://your-domain.com/api/health
curl http://your-domain.com/health/ocr

# 5. Monitor resources
./monitor-resources.sh
```

## Post-Deployment Configuration

### Update Go API to Use OCR Service

Add to `/opt/go-api/.env`:
```env
OCR_SERVICE_URL=http://127.0.0.1:8080
OCR_API_KEY=
```

Update your Go code to use the OCR client:
```go
ocrClient := services.NewOCRClient()
response, err := ocrClient.SubmitDocument(ctx, services.OCRRequest{
    PDFKey:      "documents/sample.pdf",
    DocumentID:  "doc_123",
    CallbackURL: "http://127.0.0.1:8000/api/webhooks/ocr",
})
```

Restart Go API:
```bash
systemctl restart go-api
```

## Monitoring & Maintenance

### Daily Checks

```bash
# Quick health check
curl -s http://127.0.0.1:8080/health | jq
curl -s http://127.0.0.1:8000/api/health | jq

# Check disk space
df -h

# Check memory
free -h
```

### Weekly Maintenance

```bash
# Update system packages
apt-get update && apt-get upgrade -y

# Check logs for errors
journalctl -u ocr-service --since "7 days ago" | grep -i error
journalctl -u go-api --since "7 days ago" | grep -i error

# Clean up old logs
find /var/log -name "*.log.*" -mtime +30 -delete
```

### Monthly Cleanup

```bash
# Clean Docker
docker system prune -af

# Clean package cache
apt-get autoremove -y
apt-get clean

# Check and rotate logs
logrotate -f /etc/logrotate.conf
```

## Performance Optimization

### If You Have Memory Issues

1. **Reduce Next.js instances:**
```bash
pm2 scale nextjs-app 0  # Use 0 instances (minimal)
```

2. **Optimize PostgreSQL:**
```sql
-- Connect to PostgreSQL
psql -U dbuser -d study_in_woods

-- Reduce memory settings
ALTER SYSTEM SET shared_buffers = '64MB';
ALTER SYSTEM SET work_mem = '2MB';
SELECT pg_reload_conf();
```

3. **Reduce Redis memory:**
```bash
docker exec redis redis-cli CONFIG SET maxmemory 60mb
```

### If You Need Better OCR Performance

Upgrade to a larger VPS (8GB RAM):
- Increase OCR workers to 2
- Allocate 1.5GB to OCR service
- Better concurrent processing

## Backup Strategy

### Automated Database Backup

```bash
# Create backup script
cat > /opt/scripts/backup-db.sh << 'BACKUP'
#!/bin/bash
BACKUP_DIR="/opt/backups"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

# Backup PostgreSQL
docker exec postgres pg_dump -U dbuser study_in_woods | gzip > "$BACKUP_DIR/db_$DATE.sql.gz"

# Keep only last 7 days
find $BACKUP_DIR -name "db_*.sql.gz" -mtime +7 -delete

# Upload to Spaces (optional)
# s3cmd put "$BACKUP_DIR/db_$DATE.sql.gz" s3://study-in-woods/backups/
BACKUP

chmod +x /opt/scripts/backup-db.sh

# Setup cron (daily at 2 AM)
crontab -e
# Add: 0 2 * * * /opt/scripts/backup-db.sh
```

## Disaster Recovery

### Complete System Restore

```bash
# 1. Fresh VPS setup
# 2. Restore from backups
# 3. Redeploy services using this guide
# 4. Restore database:
docker exec -i postgres psql -U dbuser study_in_woods < backup.sql
```

## Cost Breakdown

**DigitalOcean 4GB Droplet:** $24/month
- CPU: 2 vCPUs
- RAM: 4GB
- Storage: 80GB SSD
- Transfer: 4TB

**Alternative: 8GB Droplet:** $48/month
- Better performance for OCR
- More headroom for growth

## Troubleshooting

See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for detailed solutions to common issues.

## Security Hardening

```bash
# 1. Disable root login
nano /etc/ssh/sshd_config
# Set: PermitRootLogin no

# 2. Create deployment user
adduser deploy
usermod -aG sudo deploy

# 3. Setup SSH keys (from local machine)
ssh-copy-id deploy@vps-ip

# 4. Restart SSH
systemctl restart sshd

# 5. Configure fail2ban
apt-get install -y fail2ban
systemctl enable fail2ban
```

## Support

For issues:
1. Check [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
2. Review service logs: `journalctl -u ocr-service -f`
3. Check resource usage: `./monitor-resources.sh`
