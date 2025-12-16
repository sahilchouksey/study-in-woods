# OCR Service VPS Deployment - Complete File Index

All files needed for production VPS deployment of Docling OCR service on 4GB RAM.

## 📋 Quick Reference

**One-command deployment:** `sudo bash quick-deploy.sh`  
**Documentation:** Start with `README.md` → `VPS_DEPLOYMENT_GUIDE.md`  
**Troubleshooting:** See `TROUBLESHOOTING.md`

## 📁 File Structure

```
apps/ocr-service/
├── Core Application Files
│   ├── main.py                          # FastAPI application (264 lines)
│   ├── processor.py                     # OCR processing logic
│   ├── queue.py                         # Job queue management
│   ├── config.py                        # Configuration management (30 lines)
│   └── requirements.txt                 # Python dependencies (16 lines)
│
├── Production Configuration
│   ├── gunicorn.conf.py                 # Production server config (NEW)
│   ├── ocr-service.service              # systemd service file (NEW)
│   ├── nginx-ocr.conf                   # Nginx reverse proxy (NEW)
│   ├── .env.vps.example                 # Environment template (NEW)
│   └── .env.8gb.example                 # 8GB VPS config (NEW)
│
├── Deployment Scripts
│   ├── deploy-vps.sh                    # Main deployment script (NEW)
│   ├── quick-deploy.sh                  # One-command deployment (NEW)
│   └── monitor-resources.sh             # Resource monitoring (NEW)
│
├── Documentation
│   ├── README.md                        # Quick start guide (NEW)
│   ├── VPS_DEPLOYMENT_GUIDE.md          # Complete deployment walkthrough (NEW)
│   ├── TROUBLESHOOTING.md               # Common issues & solutions (NEW)
│   ├── UPGRADE_TO_8GB.md                # 8GB upgrade guide (NEW)
│   └── DEPLOYMENT_FILES_INDEX.md        # This file (NEW)
│
└── Go API Integration
    └── services/ocr_client.go           # Go client for OCR service (NEW)

Project Root:
└── VPS_ARCHITECTURE_SUMMARY.md          # Architecture overview (NEW)
```

## 📦 Core Application Files

### 1. main.py
**Purpose:** FastAPI application with async job processing  
**Key features:**
- Health check endpoint (`/health`)
- Async job submission (`/process`)
- Job status tracking (`/status/{job_id}`)
- Sync processing endpoint (`/process/sync`)
- Webhook callbacks
- Global exception handling

**No changes needed** - Already production-ready

### 2. processor.py
**Purpose:** OCR processing logic using Docling  
**Status:** Existing file (no changes)

### 3. queue.py
**Purpose:** In-memory job queue management  
**Status:** Existing file (no changes)

### 4. config.py
**Purpose:** Environment variable management  
**Status:** Existing file (no changes)

### 5. requirements.txt
**Purpose:** Python dependencies  
**Status:** Existing file (no changes)

```txt
fastapi==0.109.0
uvicorn[standard]==0.27.0
pydantic==2.6.0
pydantic-settings==2.1.0
aiohttp==3.9.1
docling==2.64.1
boto3==1.35.0
gunicorn==21.2.0
```

## 🔧 Production Configuration Files

### 6. gunicorn.conf.py ⭐ NEW
**Purpose:** Production WSGI server configuration  
**Key settings:**
- 1 worker for 4GB VPS (memory optimized)
- Uvicorn worker class for async support
- 120s timeout for OCR processing
- Auto-restart workers every 500 requests
- Comprehensive logging

**Usage:** Loaded automatically by systemd service

### 7. ocr-service.service ⭐ NEW
**Purpose:** systemd service unit file  
**Key features:**
- Memory limit: 800MB max
- CPU quota: 50%
- Runs as www-data user
- Auto-restart on failure
- Security hardening (NoNewPrivileges, PrivateTmp, etc.)
- Proper environment loading

**Installation:** Copied to `/etc/systemd/system/` by deploy script

### 8. nginx-ocr.conf ⭐ NEW
**Purpose:** Nginx reverse proxy configuration  
**Features:**
- Routes for Next.js, Go API, OCR service
- Rate limiting for OCR endpoint
- Internal-only access for OCR (localhost)
- Security headers
- Extended timeout for OCR processing (300s)
- Public health check endpoint

**Installation:** Copied to `/etc/nginx/sites-available/` by deploy script

### 9. .env.vps.example ⭐ NEW
**Purpose:** Environment variable template for 4GB VPS  
**Variables:**
```env
SPACES_KEY=your-do-spaces-access-key
SPACES_SECRET=your-do-spaces-secret-key
SPACES_BUCKET=study-in-woods
SPACES_REGION=blr1
SPACES_ENDPOINT=https://blr1.digitaloceanspaces.com
WEBHOOK_URL=http://127.0.0.1:8000/api/webhooks/ocr
OCR_DEVICE=cpu
PORT=8080
```

**Usage:** Copy to `/opt/ocr-service/.env` and update credentials

### 10. .env.8gb.example ⭐ NEW
**Purpose:** Optimized configuration for 8GB VPS  
**Differences from 4GB:**
- `DOCLING_MAX_WORKERS=2` (vs 1)
- `OMP_NUM_THREADS=4` (vs 2)
- Use with 2 Gunicorn workers

**Usage:** Use when upgrading to 8GB VPS

## 🚀 Deployment Scripts

### 11. deploy-vps.sh ⭐ NEW
**Purpose:** Main deployment automation script  
**What it does:**
1. Installs system dependencies (Python 3.11, Nginx, etc.)
2. Creates directory structure
3. Sets up Python virtual environment
4. Installs Python packages
5. Pre-downloads Docling models
6. Creates .env file
7. Installs systemd service
8. Configures Nginx
9. Sets up log rotation
10. Creates 2GB swap file
11. Runs health checks

**Usage:**
```bash
cd apps/ocr-service
sudo bash deploy-vps.sh
```

**Duration:** ~10-15 minutes (includes model download)

### 12. quick-deploy.sh ⭐ NEW
**Purpose:** One-command complete VPS setup  
**What it does:**
- System update & upgrade
- Dependency installation
- Swap file setup
- Runs deploy-vps.sh
- Firewall configuration
- System optimization
- Final health checks

**Usage:**
```bash
sudo bash quick-deploy.sh
```

**Duration:** ~15-20 minutes

**Best for:** Fresh VPS setup

### 13. monitor-resources.sh ⭐ NEW
**Purpose:** Real-time resource monitoring  
**Displays:**
- Memory usage (system + per service)
- CPU usage per core
- Disk I/O statistics
- Service status
- Active network connections

**Usage:**
```bash
./monitor-resources.sh [interval_seconds]
# Example: ./monitor-resources.sh 5
```

**Requirements:** `sysstat` package (installed by deploy script)

## 📚 Documentation Files

### 14. README.md ⭐ NEW
**Purpose:** Quick start guide and API reference  
**Sections:**
- Quick deployment instructions
- Architecture diagram
- Resource allocation
- Configuration guide
- API usage examples
- Maintenance tasks
- Troubleshooting quick reference

**Target audience:** Developers deploying for the first time

### 15. VPS_DEPLOYMENT_GUIDE.md ⭐ NEW
**Purpose:** Complete step-by-step deployment walkthrough  
**Sections:**
- Prerequisites and VPS setup
- Resource allocation plan
- Step-by-step deployment (PostgreSQL, Go API, OCR, Next.js, Nginx)
- SSL setup with Certbot
- Verification checklist
- Post-deployment configuration
- Monitoring and maintenance
- Backup strategy
- Disaster recovery

**Target audience:** DevOps engineers, detailed deployment

### 16. TROUBLESHOOTING.md ⭐ NEW
**Purpose:** Comprehensive troubleshooting guide  
**Covers:**
- Quick health checks
- 6 common issues with solutions:
  1. Service won't start
  2. Out of memory (OOM)
  3. Slow OCR processing
  4. Webhook callbacks failing
  5. Nginx 502 errors
  6. Python dependencies missing
- Maintenance tasks
- Performance tuning
- Emergency recovery procedures
- Useful commands reference

**Target audience:** Operations teams, on-call engineers

### 17. UPGRADE_TO_8GB.md ⭐ NEW
**Purpose:** Guide for upgrading from 4GB to 8GB VPS  
**Sections:**
- Benefits comparison (4GB vs 8GB)
- When to upgrade
- Step-by-step upgrade process
- Updated configuration files
- Performance benchmarks
- Rollback procedure
- Cost-benefit analysis

**Target audience:** Teams scaling up

### 18. DEPLOYMENT_FILES_INDEX.md ⭐ NEW
**Purpose:** This file - complete file reference  
**Content:** Overview of all deployment files and their purpose

## 🔌 Go API Integration

### 19. services/ocr_client.go ⭐ NEW
**Purpose:** Go client library for OCR service communication  
**Location:** `apps/api/services/ocr_client.go`

**Key components:**
```go
// Client initialization
func NewOCRClient() *OCRClient

// Submit async job
func (c *OCRClient) SubmitDocument(ctx, req) (*OCRResponse, error)

// Check job status
func (c *OCRClient) GetJobStatus(ctx, jobID) (*OCRJobStatus, error)

// Process sync (small PDFs)
func (c *OCRClient) ProcessDocumentSync(ctx, req) (result, error)

// Health check
func (c *OCRClient) HealthCheck(ctx) error

// Wait for completion (polling)
func (c *OCRClient) WaitForCompletion(ctx, jobID, ...) (*OCRJobStatus, error)
```

**Usage in Go API:**
```go
ocrClient := services.NewOCRClient()
response, err := ocrClient.SubmitDocument(ctx, services.OCRRequest{
    PDFKey:      "documents/sample.pdf",
    DocumentID:  "doc_123",
    CallbackURL: "http://127.0.0.1:8000/api/webhooks/ocr",
})
```

**Default URL:** `http://127.0.0.1:8080` (configurable via `OCR_SERVICE_URL` env var)

## 📊 Project Root Documentation

### 20. VPS_ARCHITECTURE_SUMMARY.md ⭐ NEW
**Purpose:** High-level architecture overview  
**Location:** Project root  
**Sections:**
- System architecture diagram
- Resource allocation table
- Service communication flow
- File structure overview
- Deployment workflow
- Security configuration
- Monitoring guide
- Performance optimization
- Cost analysis

**Target audience:** Project managers, architects, stakeholders

## 🎯 Deployment Workflow

### Quick Deployment (Recommended)
```bash
# 1. Fresh Ubuntu 22.04 VPS
ssh root@vps-ip

# 2. Upload OCR service files
scp -r apps/ocr-service/* root@vps:/tmp/ocr-service/

# 3. Run quick deploy
cd /tmp/ocr-service
sudo bash quick-deploy.sh

# 4. Configure credentials
sudo nano /opt/ocr-service/.env

# 5. Restart and verify
sudo systemctl restart ocr-service
curl http://127.0.0.1:8080/health
```

**Time:** 15-20 minutes  
**Difficulty:** Easy

### Manual Deployment
```bash
# Follow VPS_DEPLOYMENT_GUIDE.md step-by-step
# More control, same result
```

**Time:** 30-45 minutes  
**Difficulty:** Intermediate

## ✅ Pre-Deployment Checklist

Before deployment, ensure you have:

- [ ] 4GB RAM VPS (Ubuntu 22.04 LTS)
- [ ] Root SSH access
- [ ] DigitalOcean Spaces credentials
  - [ ] Access key
  - [ ] Secret key
  - [ ] Bucket name
  - [ ] Region
- [ ] Domain name (optional but recommended)
- [ ] Basic Linux knowledge
- [ ] 20-30 minutes of time

## 📝 Post-Deployment Checklist

After deployment, verify:

- [ ] All services running (`systemctl status ocr-service`)
- [ ] Health checks passing (`curl localhost:8080/health`)
- [ ] OCR test successful (submit test PDF)
- [ ] Webhook callback working
- [ ] Resource usage acceptable (`free -h`)
- [ ] Logs rotating properly
- [ ] Nginx serving requests
- [ ] SSL certificate installed (if using domain)
- [ ] Firewall configured (`ufw status`)
- [ ] Database backup configured

## 🔧 Configuration Files Summary

| File | Location After Deploy | Purpose |
|------|----------------------|---------|
| `.env` | `/opt/ocr-service/.env` | Environment variables |
| `ocr-service.service` | `/etc/systemd/system/` | systemd unit |
| `nginx-ocr.conf` | `/etc/nginx/sites-available/app` | Nginx config |
| `gunicorn.conf.py` | `/opt/ocr-service/` | Gunicorn settings |
| `logrotate` | `/etc/logrotate.d/ocr-service` | Log rotation |

## 📞 Getting Help

1. **Quick issues:** Check `TROUBLESHOOTING.md`
2. **Deployment help:** See `VPS_DEPLOYMENT_GUIDE.md`
3. **Architecture questions:** See `VPS_ARCHITECTURE_SUMMARY.md`
4. **Service logs:** `journalctl -u ocr-service -f`
5. **Resource monitoring:** `./monitor-resources.sh`

## 🎓 Learning Path

**For first-time deployers:**
1. Read `README.md` (10 min)
2. Skim `VPS_DEPLOYMENT_GUIDE.md` (15 min)
3. Run `quick-deploy.sh` (20 min)
4. Bookmark `TROUBLESHOOTING.md` for later

**For experienced DevOps:**
1. Review `VPS_ARCHITECTURE_SUMMARY.md` (5 min)
2. Customize `deploy-vps.sh` if needed
3. Deploy manually for full control
4. Setup monitoring and alerts

## 📊 File Statistics

**Total new files created:** 12  
**Total lines of code/config:** ~2,500  
**Documentation:** ~1,500 lines  
**Scripts:** ~600 lines  
**Configuration:** ~400 lines  

## 🚀 Quick Commands Reference

```bash
# Deploy
sudo bash quick-deploy.sh

# Configure
sudo nano /opt/ocr-service/.env

# Restart
sudo systemctl restart ocr-service

# Logs
journalctl -u ocr-service -f

# Monitor
./monitor-resources.sh

# Test
curl http://127.0.0.1:8080/health

# Status
systemctl status ocr-service
```

## 💡 Pro Tips

1. **Always backup before updates:** `cp -r /opt/ocr-service /opt/ocr-service.backup`
2. **Monitor resources regularly:** Run `monitor-resources.sh` weekly
3. **Check logs proactively:** Set up log monitoring alerts
4. **Test before scaling:** Verify on 4GB before upgrading to 8GB
5. **Document changes:** Keep notes of customizations

## 📅 Maintenance Schedule

**Daily:** Health checks  
**Weekly:** Log review, resource monitoring  
**Monthly:** System updates, log cleanup  
**Quarterly:** Backup verification, disaster recovery test

---

**Created:** December 2024  
**Version:** 2.0.0  
**Total Files:** 20 (12 new + 8 existing)  
**Target Platform:** Ubuntu 22.04 LTS on 4GB VPS
