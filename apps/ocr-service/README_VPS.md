# Docling OCR Service - VPS Deployment

Production-ready FastAPI service for OCR processing using IBM Docling, optimized for 4GB VPS deployment.

## 🚀 Quick Start

### One-Command Deployment

```bash
# On your 4GB VPS (Ubuntu 22.04)
cd /tmp
git clone <your-repo> study-in-woods
cd study-in-woods/apps/ocr-service
sudo bash quick-deploy.sh
```

That's it! The script will:
- Install all dependencies (Python 3.11, Nginx, etc.)
- Setup virtual environment
- Configure systemd service
- Setup Nginx reverse proxy
- Configure swap file
- Start OCR service

### Manual Deployment

If you prefer step-by-step control:

```bash
sudo bash deploy-vps.sh
```

## 📋 Prerequisites

- **VPS:** 4GB RAM minimum (DigitalOcean, Linode, Vultr, etc.)
- **OS:** Ubuntu 22.04 LTS
- **Credentials:** DigitalOcean Spaces access keys

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│         4GB RAM VPS                                  │
├─────────────────────────────────────────────────────┤
│  Nginx (Port 80/443)                                 │
│    ├─> :3000  → Next.js Frontend                    │
│    ├─> :8000  → Go API Backend                      │
│    └─> :8080  → OCR Service (Internal)              │
├─────────────────────────────────────────────────────┤
│  Internal Communication:                             │
│    Go API → http://127.0.0.1:8080/process          │
│    OCR → http://127.0.0.1:8000/api/webhooks/ocr    │
└─────────────────────────────────────────────────────┘
```

## 📊 Resource Allocation

| Service      | RAM Usage | CPU Quota | Priority |
|--------------|-----------|-----------|----------|
| OCR Service  | 600-800MB | 50%       | Medium   |
| Go API       | 80-120MB  | 30%       | High     |
| Next.js      | 250-350MB | 40%       | High     |
| PostgreSQL   | 100-150MB | 20%       | High     |
| Redis        | 40-60MB   | 10%       | Medium   |
| **Total**    | **~1.4GB**| **150%**  | -        |

## 🔧 Configuration

### 1. Update Environment Variables

```bash
sudo nano /opt/ocr-service/.env
```

Required variables:
```env
SPACES_KEY=your-digitalocean-spaces-key
SPACES_SECRET=your-digitalocean-spaces-secret
SPACES_BUCKET=study-in-woods
SPACES_REGION=blr1
WEBHOOK_URL=http://127.0.0.1:8000/api/webhooks/ocr
```

### 2. Restart Service

```bash
sudo systemctl restart ocr-service
```

## 🔍 Verification

```bash
# Check service status
systemctl status ocr-service

# Test health endpoint
curl http://127.0.0.1:8080/health | jq

# View logs
journalctl -u ocr-service -f

# Monitor resources
/opt/ocr-service/monitor-resources.sh
```

## 📡 API Usage

### Submit Document for Processing

```bash
curl -X POST http://127.0.0.1:8080/process \
  -H "Content-Type: application/json" \
  -d '{
    "pdf_key": "documents/sample.pdf",
    "document_id": "doc_123",
    "callback_url": "http://127.0.0.1:8000/api/webhooks/ocr"
  }'
```

Response:
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "message": "Document queued for OCR processing"
}
```

### Check Job Status

```bash
curl http://127.0.0.1:8080/status/{job_id} | jq
```

### Health Check

```bash
curl http://127.0.0.1:8080/health | jq
```

## 🔗 Integration with Go API

Add to your Go API (see `services/ocr_client.go`):

```go
import "your-module/services"

// Create OCR client
ocrClient := services.NewOCRClient()

// Submit document
response, err := ocrClient.SubmitDocument(ctx, services.OCRRequest{
    PDFKey:      "documents/sample.pdf",
    DocumentID:  "doc_123",
    CallbackURL: "http://127.0.0.1:8000/api/webhooks/ocr",
})

if err != nil {
    log.Error("OCR submission failed:", err)
    return err
}

log.Info("OCR job submitted:", response.JobID)
```

## 📁 Project Structure

```
apps/ocr-service/
├── main.py                    # FastAPI application
├── processor.py               # OCR processing logic
├── queue.py                   # Job queue management
├── config.py                  # Configuration
├── requirements.txt           # Python dependencies
├── gunicorn.conf.py          # Production server config
├── ocr-service.service       # systemd service file
├── nginx-ocr.conf            # Nginx configuration
├── deploy-vps.sh             # Main deployment script
├── quick-deploy.sh           # One-command deployment
├── monitor-resources.sh      # Resource monitoring
├── VPS_DEPLOYMENT_GUIDE.md   # Complete deployment guide
├── TROUBLESHOOTING.md        # Troubleshooting guide
└── README.md                 # This file
```

## 🛠️ Maintenance

### View Logs

```bash
# Service logs
sudo journalctl -u ocr-service -f

# Application logs
sudo tail -f /var/log/ocr-service/error.log
sudo tail -f /var/log/ocr-service/access.log
```

### Restart Service

```bash
sudo systemctl restart ocr-service
```

### Monitor Resources

```bash
# Real-time monitoring
/opt/ocr-service/monitor-resources.sh

# Quick check
free -h
systemctl status ocr-service
```

### Update Code

```bash
# 1. Stop service
sudo systemctl stop ocr-service

# 2. Backup
sudo cp -r /opt/ocr-service /opt/ocr-service.backup

# 3. Update files (upload new code)
sudo cp -r /path/to/new/code/* /opt/ocr-service/

# 4. Update dependencies if needed
cd /opt/ocr-service
source venv/bin/activate
pip install -r requirements.txt --upgrade

# 5. Restart
sudo systemctl start ocr-service
```

## 🐛 Troubleshooting

See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for detailed solutions.

**Quick Fixes:**

```bash
# Service won't start
sudo journalctl -u ocr-service -n 50

# Out of memory
free -h
sudo systemctl restart ocr-service

# High CPU usage
htop

# Permission issues
sudo chown -R www-data:www-data /opt/ocr-service
```

## 📚 Documentation

- **[VPS_DEPLOYMENT_GUIDE.md](./VPS_DEPLOYMENT_GUIDE.md)** - Complete deployment guide
- **[TROUBLESHOOTING.md](./TROUBLESHOOTING.md)** - Common issues and solutions
- **API Docs:** http://your-vps-ip/ocr/docs (when running)

## 🔒 Security

### Hardening Checklist

- ✅ Service runs as `www-data` (non-root)
- ✅ OCR endpoint accessible only via localhost
- ✅ Resource limits enforced (systemd)
- ✅ Firewall configured (UFW)
- ✅ HTTPS via Nginx (with Certbot)
- ✅ Private tmp directory
- ✅ Read-only file system protection

### API Key (Optional)

For internal networks, API keys are optional. To enable:

```bash
# Generate random key
openssl rand -hex 32

# Add to .env
echo "OCR_API_KEY=your-generated-key" >> /opt/ocr-service/.env

# Restart
sudo systemctl restart ocr-service
```

## 📈 Performance Tuning

### For Large PDFs

```bash
# Edit systemd service
sudo nano /etc/systemd/system/ocr-service.service

# Increase timeout
--timeout 300

# Allow more memory
MemoryMax=1G

sudo systemctl daemon-reload
sudo systemctl restart ocr-service
```

### For High Throughput

If you have RAM available:

```bash
# Increase workers (needs more RAM)
--workers 2

# Adjust memory limit
MemoryMax=1.5G
```

## 🚨 Emergency Recovery

### Complete Reset

```bash
# Stop everything
sudo systemctl stop ocr-service nginx

# Kill stuck processes
sudo pkill -9 gunicorn

# Clear logs
sudo truncate -s 0 /var/log/ocr-service/*.log

# Restart
sudo systemctl start ocr-service nginx
```

### Rollback

```bash
# Stop service
sudo systemctl stop ocr-service

# Restore backup
sudo rm -rf /opt/ocr-service
sudo mv /opt/ocr-service.backup /opt/ocr-service

# Start
sudo systemctl start ocr-service
```

## 💰 Cost Estimate

**DigitalOcean 4GB Droplet:** $24/month
- 2 vCPUs, 4GB RAM, 80GB SSD
- Sufficient for moderate OCR workload

**Recommended: 8GB Droplet:** $48/month
- Better performance for concurrent processing
- More headroom for growth

## 🤝 Support

1. Check [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
2. View logs: `journalctl -u ocr-service -f`
3. Monitor resources: `./monitor-resources.sh`

## 📝 License

MIT License - See main project LICENSE file

## 🎯 Features

- ✅ Async job processing with background tasks
- ✅ Webhook callbacks on completion
- ✅ Job status tracking
- ✅ S3-compatible storage (DigitalOcean Spaces)
- ✅ Production-ready with Gunicorn
- ✅ Resource limits and monitoring
- ✅ Automatic log rotation
- ✅ Health check endpoints
- ✅ Graceful shutdown
- ✅ Memory-optimized for VPS

## 🔄 Version

**Current Version:** 2.0.0
- Production-ready VPS deployment
- Memory-optimized configuration
- Complete monitoring and troubleshooting
