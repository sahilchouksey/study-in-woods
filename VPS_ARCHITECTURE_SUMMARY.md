# VPS Deployment Architecture Summary

## 📋 Overview

Complete production-ready architecture for deploying the Docling OCR FastAPI service alongside Go API and Next.js frontend on a single 4GB VPS.

## 🏗️ System Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Internet (Port 80/443)                        │
└─────────────────────────────────┬────────────────────────────────────┘
                                  │
                          ┌───────▼──────┐
                          │     Nginx    │ (Reverse Proxy)
                          │   Port 80    │ (SSL: 443)
                          └──┬────┬────┬─┘
                             │    │    │
              ┌──────────────┘    │    └──────────────┐
              │                   │                    │
       ┌──────▼────────┐  ┌──────▼────────┐  ┌───────▼────────┐
       │   Next.js     │  │    Go API     │  │  OCR Service   │
       │  Port 3000    │  │   Port 8000   │  │   Port 8080    │
       │  (Frontend)   │  │  (Backend)    │  │  (Internal)    │
       │               │  │               │  │                │
       │ PM2 Managed   │  │   systemd     │  │  Gunicorn +    │
       │ ~300MB RAM    │  │   ~100MB RAM  │  │  Uvicorn       │
       │               │  │               │  │  ~800MB RAM    │
       └───────────────┘  └───┬───────┬───┘  └────────────────┘
                              │       │              ▲
                              │       └──────────────┘
                              │    (Webhook callback)
                              │
                 ┌────────────┴────────────┐
                 │                         │
          ┌──────▼──────┐          ┌──────▼──────┐
          │ PostgreSQL  │          │    Redis    │
          │ Port 5432   │          │  Port 6379  │
          │ ~150MB RAM  │          │  ~50MB RAM  │
          └─────────────┘          └─────────────┘
                 │                         │
          ┌──────┴─────────────────────────┴──────┐
          │        Docker Network                  │
          └────────────────────────────────────────┘
```

## 💾 Resource Allocation (4GB RAM VPS)

| Component     | Min RAM | Max RAM | CPU Quota | Storage | Priority |
|--------------|---------|---------|-----------|---------|----------|
| OCR Service  | 600 MB  | 800 MB  | 50%       | 2 GB    | Medium   |
| Go API       | 80 MB   | 200 MB  | 30%       | 500 MB  | High     |
| Next.js      | 250 MB  | 400 MB  | 40%       | 1 GB    | High     |
| PostgreSQL   | 100 MB  | 200 MB  | 20%       | 5 GB    | High     |
| Redis        | 40 MB   | 100 MB  | 10%       | 100 MB  | Medium   |
| Nginx        | 10 MB   | 50 MB   | 5%        | 50 MB   | High     |
| **Subtotal** | **1.1 GB** | **1.75 GB** | **155%** | **8.6 GB** | - |
| System       | 300 MB  | -       | -         | 10 GB   | -        |
| Swap         | -       | 2 GB    | -         | 2 GB    | -        |
| **Total**    | **1.4 GB** | **3.75 GB** | **155%** | **20 GB** | - |

**Safety Margins:**
- Normal load: ~1.4GB RAM used (~35% of 4GB)
- Peak load: ~2.5GB RAM used (~62% of 4GB)
- Emergency buffer: 2GB swap
- Disk space: 20GB used of 80GB available

## 🔄 Service Communication Flow

### OCR Processing Workflow

```
┌─────────┐     ┌─────────┐     ┌──────────┐     ┌─────────┐
│  User   │────▶│ Next.js │────▶│  Go API  │────▶│   S3    │
│         │     │         │     │          │     │ Spaces  │
└─────────┘     └─────────┘     └────┬─────┘     └─────────┘
                                     │
                                     │ 1. Submit PDF
                                     ▼
                              ┌──────────────┐
                              │ OCR Service  │
                              │ (localhost)  │
                              └──────┬───────┘
                                     │ 2. Download PDF from S3
                                     │ 3. Process with Docling
                                     │ 4. Upload results to S3
                                     │
                                     │ 5. Webhook callback
                                     ▼
                              ┌──────────────┐
                              │   Go API     │
                              │  /webhooks   │
                              └──────┬───────┘
                                     │
                                     │ 6. Update database
                                     ▼
                              ┌──────────────┐
                              │ PostgreSQL   │
                              └──────────────┘
```

### Internal URLs

- **Go API → OCR:** `http://127.0.0.1:8080/process`
- **OCR → Go API:** `http://127.0.0.1:8000/api/webhooks/ocr`
- **Go API → PostgreSQL:** `localhost:5432`
- **Go API → Redis:** `localhost:6379`
- **External → Nginx:** `https://your-domain.com`

## 📦 File Structure

```
/opt/
├── ocr-service/                    # OCR Service (800MB)
│   ├── venv/                       # Python virtualenv
│   ├── main.py                     # FastAPI app
│   ├── processor.py                # Docling OCR logic
│   ├── queue.py                    # Job queue
│   ├── .env                        # Environment variables
│   └── gunicorn.conf.py            # Production config
│
├── go-api/                         # Go API Backend (100MB)
│   ├── api                         # Compiled binary
│   ├── .env                        # Environment variables
│   └── ...                         # Source files
│
├── nextjs-app/                     # Next.js Frontend (300MB)
│   ├── .next/                      # Production build
│   ├── node_modules/               # Dependencies
│   └── ecosystem.config.js         # PM2 config
│
└── scripts/
    └── backup-db.sh                # Database backup script

/var/log/
├── ocr-service/
│   ├── access.log                  # OCR access logs
│   └── error.log                   # OCR error logs
├── nginx/
│   ├── access.log                  # Nginx access logs
│   └── error.log                   # Nginx error logs
└── nextjs/
    ├── out.log                     # Next.js output
    └── error.log                   # Next.js errors

/etc/systemd/system/
├── ocr-service.service             # OCR systemd unit
└── go-api.service                  # Go API systemd unit

/etc/nginx/
└── sites-available/
    └── app                         # Main Nginx config
```

## 🚀 Deployment Workflow

### Quick Deployment (Recommended)

```bash
# 1. Clone repository on VPS
git clone <your-repo> study-in-woods
cd study-in-woods/apps/ocr-service

# 2. Run quick deploy script
sudo bash quick-deploy.sh

# 3. Configure credentials
sudo nano /opt/ocr-service/.env

# 4. Restart and verify
sudo systemctl restart ocr-service
curl http://127.0.0.1:8080/health
```

### Manual Deployment Steps

```bash
# 1. System preparation
sudo apt-get update && sudo apt-get upgrade -y
sudo fallocate -l 2G /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile

# 2. Install dependencies
sudo add-apt-repository ppa:deadsnakes/ppa
sudo apt-get install python3.11 python3.11-venv nginx -y

# 3. Deploy OCR service
cd /tmp/ocr-service
sudo bash deploy-vps.sh

# 4. Deploy Go API
cd /opt/go-api
go build -o api main.go
sudo systemctl enable go-api && sudo systemctl start go-api

# 5. Deploy Next.js
cd /opt/nextjs-app
npm install && npm run build
pm2 start ecosystem.config.js && pm2 save

# 6. Configure Nginx
sudo cp nginx-ocr.conf /etc/nginx/sites-available/app
sudo ln -s /etc/nginx/sites-available/app /etc/nginx/sites-enabled/
sudo systemctl reload nginx
```

## 🔐 Security Configuration

### Systemd Security Hardening

```ini
[Service]
# Resource limits
MemoryMax=800M
CPUQuota=50%

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/ocr-service

# User/Group
User=www-data
Group=www-data
```

### Nginx Security Headers

```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
```

### Firewall Rules

```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
ufw allow 'Nginx Full'
ufw enable
```

## 📊 Monitoring & Health Checks

### Service Status Commands

```bash
# Check all services
systemctl status ocr-service
systemctl status go-api
systemctl status nginx
pm2 status

# View logs
journalctl -u ocr-service -f
journalctl -u go-api -f
tail -f /var/log/nginx/error.log
pm2 logs
```

### Health Endpoints

```bash
# OCR Service
curl http://127.0.0.1:8080/health

# Go API
curl http://127.0.0.1:8000/api/health

# Next.js
curl http://127.0.0.1:3000

# Public (via Nginx)
curl https://your-domain.com/health/ocr
```

### Resource Monitoring Script

```bash
# Real-time monitoring
/opt/ocr-service/monitor-resources.sh

# Shows:
# - Memory usage per service
# - CPU usage per core
# - Disk I/O
# - Network connections
# - Service status
```

## 🛠️ Common Operations

### Restart Services

```bash
# OCR Service
sudo systemctl restart ocr-service

# Go API
sudo systemctl restart go-api

# Next.js
pm2 restart nextjs-app

# Nginx
sudo systemctl reload nginx

# All services
sudo systemctl restart ocr-service go-api nginx && pm2 restart all
```

### View Logs

```bash
# OCR Service (journalctl)
sudo journalctl -u ocr-service -f

# OCR Service (application logs)
sudo tail -f /var/log/ocr-service/error.log

# Go API
sudo journalctl -u go-api -f

# Next.js
pm2 logs nextjs-app

# Nginx
sudo tail -f /var/log/nginx/error.log
```

### Update Code

```bash
# OCR Service
sudo systemctl stop ocr-service
sudo cp -r /path/to/new/code/* /opt/ocr-service/
cd /opt/ocr-service && source venv/bin/activate
pip install -r requirements.txt --upgrade
sudo systemctl start ocr-service

# Go API
sudo systemctl stop go-api
cd /opt/go-api
go build -o api main.go
sudo systemctl start go-api

# Next.js
cd /opt/nextjs-app
git pull origin main
npm install
npm run build
pm2 restart nextjs-app
```

## 📈 Performance Optimization

### For Memory-Constrained Environments

```bash
# Reduce OCR workers to 1
--workers 1

# Lower memory limits
MemoryMax=600M

# Increase swap aggressiveness
sudo sysctl vm.swappiness=80
```

### For High-Throughput Workloads

```bash
# Increase OCR workers (needs more RAM)
--workers 2

# Adjust memory limits
MemoryMax=1.5G

# Optimize timeouts
--timeout 180
```

### Database Optimization

```sql
-- PostgreSQL settings for 4GB VPS
ALTER SYSTEM SET shared_buffers = '128MB';
ALTER SYSTEM SET work_mem = '4MB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
SELECT pg_reload_conf();
```

## 🐛 Troubleshooting Quick Reference

| Issue | Symptom | Solution |
|-------|---------|----------|
| Service won't start | `systemctl status` shows failed | Check logs: `journalctl -u ocr-service -n 50` |
| Out of memory | Slow performance, crashes | Reduce workers, check swap: `free -h` |
| Slow OCR | Processing takes >60s | Optimize threads: `OMP_NUM_THREADS=2` |
| Webhook fails | No callback received | Check URL: `http://127.0.0.1:8000/api/webhooks/ocr` |
| 502 Bad Gateway | Nginx error | Verify OCR service running: `systemctl status ocr-service` |
| High CPU | 100% CPU usage | Check processes: `htop`, adjust workers |

## 💰 Cost Analysis

### 4GB Droplet ($24/month)
✅ **Pros:**
- Cost-effective for small to medium workloads
- Sufficient for moderate OCR usage
- Good for development/staging

⚠️ **Cons:**
- Limited concurrent processing
- May need swap for heavy loads
- Single worker for OCR

### 8GB Droplet ($48/month)
✅ **Pros:**
- Better performance for OCR
- Can run 2-3 OCR workers
- More headroom for growth
- Better for production

⚠️ **Cons:**
- 2x cost
- May be overkill for small projects

**Recommendation:** Start with 4GB, upgrade to 8GB if you experience:
- Frequent out-of-memory issues
- Need for concurrent OCR processing
- Heavy traffic requiring more workers

## 📚 Documentation Files

| File | Purpose |
|------|---------|
| `README.md` | Quick start and API reference |
| `VPS_DEPLOYMENT_GUIDE.md` | Complete deployment walkthrough |
| `TROUBLESHOOTING.md` | Common issues and solutions |
| `deploy-vps.sh` | Main deployment script |
| `quick-deploy.sh` | One-command deployment |
| `monitor-resources.sh` | Resource monitoring tool |
| `ocr-service.service` | systemd service file |
| `nginx-ocr.conf` | Nginx configuration |
| `gunicorn.conf.py` | Production server config |

## ✅ Deployment Checklist

### Pre-Deployment
- [ ] VPS provisioned (4GB RAM minimum)
- [ ] Domain DNS configured (optional)
- [ ] DigitalOcean Spaces credentials ready
- [ ] SSH access configured

### Deployment
- [ ] System updated: `apt-get update && upgrade`
- [ ] Swap file created: 2GB
- [ ] OCR service deployed
- [ ] Go API deployed
- [ ] Next.js deployed
- [ ] PostgreSQL and Redis running
- [ ] Nginx configured

### Configuration
- [ ] `.env` files updated with credentials
- [ ] Webhook URL configured: `http://127.0.0.1:8000/api/webhooks/ocr`
- [ ] Nginx domain updated
- [ ] SSL certificate installed (if using domain)
- [ ] Firewall configured

### Verification
- [ ] All services running: `systemctl status`
- [ ] Health checks passing: `curl localhost:8080/health`
- [ ] OCR test successful
- [ ] Webhook callback working
- [ ] External access working: `curl https://your-domain.com`
- [ ] Resource usage acceptable: `free -h`

### Post-Deployment
- [ ] Database backup configured
- [ ] Log rotation configured
- [ ] Monitoring setup
- [ ] Documentation reviewed
- [ ] Emergency recovery tested

## 🎯 Next Steps After Deployment

1. **Test OCR Processing:**
   ```bash
   curl -X POST http://127.0.0.1:8080/process \
     -H "Content-Type: application/json" \
     -d '{"pdf_key": "test.pdf", "document_id": "test123"}'
   ```

2. **Setup Monitoring:**
   - Configure health check cron job
   - Setup alerts for service failures
   - Monitor disk space and memory

3. **Optimize Performance:**
   - Tune worker counts based on load
   - Adjust memory limits as needed
   - Configure CDN for static assets

4. **Backup Strategy:**
   - Configure automated database backups
   - Test restore procedure
   - Document recovery process

## 📞 Support Resources

- **Troubleshooting:** See `TROUBLESHOOTING.md`
- **Deployment Guide:** See `VPS_DEPLOYMENT_GUIDE.md`
- **Health Check:** `curl http://127.0.0.1:8080/health`
- **Logs:** `journalctl -u ocr-service -f`
- **Resources:** `/opt/ocr-service/monitor-resources.sh`

---

**Created:** December 2024  
**Version:** 2.0.0  
**Platform:** Ubuntu 22.04 LTS on 4GB VPS
