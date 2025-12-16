# OCR Service VPS Troubleshooting Guide

## Quick Health Checks

### 1. Check Service Status
```bash
# OCR Service
systemctl status ocr-service

# View recent logs
journalctl -u ocr-service -n 50 --no-pager

# Follow logs in real-time
journalctl -u ocr-service -f
```

### 2. Test Endpoints
```bash
# Health check
curl http://127.0.0.1:8080/health | jq

# Service info
curl http://127.0.0.1:8080/ | jq

# From Go API (simulate internal call)
curl -X POST http://127.0.0.1:8080/process \
  -H "Content-Type: application/json" \
  -d '{
    "pdf_key": "test/sample.pdf",
    "document_id": "test123",
    "callback_url": "http://127.0.0.1:8000/api/webhooks/ocr"
  }'
```

### 3. Check Resource Usage
```bash
# Quick memory check
free -h

# Service-specific memory
ps aux | grep gunicorn | awk '{sum+=$4} END {print "OCR Memory: " sum "%"}'

# Monitor in real-time
./monitor-resources.sh

# Check systemd resource limits
systemctl show ocr-service | grep -i memory
```

## Common Issues

### Issue 1: Service Won't Start

**Symptoms:**
```
● ocr-service.service - Docling OCR Service
   Loaded: loaded
   Active: failed (Result: exit-code)
```

**Solutions:**

1. Check logs for specific error:
```bash
journalctl -u ocr-service -n 100 --no-pager
```

2. Common causes:
```bash
# Missing dependencies
sudo -u www-data /opt/ocr-service/venv/bin/pip list | grep docling

# Permission issues
ls -la /opt/ocr-service
sudo chown -R www-data:www-data /opt/ocr-service

# Port already in use
netstat -tlnp | grep 8080
# Kill conflicting process: sudo kill -9 <PID>

# Invalid .env file
sudo cat /opt/ocr-service/.env
# Check for missing quotes, special characters
```

3. Test manually:
```bash
sudo -u www-data bash
cd /opt/ocr-service
source venv/bin/activate
python3 main.py
# Check for Python errors
```

### Issue 2: Out of Memory (OOM)

**Symptoms:**
- Service randomly crashes
- `dmesg | grep -i kill` shows OOM killer
- Slow performance, high swap usage

**Solutions:**

1. Check current memory:
```bash
free -h
dmesg | grep -i "out of memory"
```

2. Reduce OCR workers:
```bash
# Edit systemd service
sudo nano /etc/systemd/system/ocr-service.service

# Change workers to 1 (if not already)
--workers 1

# Lower memory limit if needed
MemoryMax=600M

sudo systemctl daemon-reload
sudo systemctl restart ocr-service
```

3. Enable aggressive swapping:
```bash
# Check swap
swapon --show

# Adjust swappiness (higher = more swap usage)
sudo sysctl vm.swappiness=80
echo "vm.swappiness=80" | sudo tee -a /etc/sysctl.conf
```

4. Clear cache:
```bash
# Drop caches (safe, just clears cache)
sudo sync && sudo sysctl -w vm.drop_caches=3
```

### Issue 3: Slow OCR Processing

**Symptoms:**
- Requests timeout
- Processing takes >60 seconds
- CPU at 100%

**Solutions:**

1. Check CPU usage:
```bash
top -bn1 | grep gunicorn
mpstat 1 5
```

2. Optimize Docling:
```bash
# Edit .env
sudo nano /opt/ocr-service/.env

# Add/update:
OMP_NUM_THREADS=2
DOCLING_MAX_WORKERS=1

sudo systemctl restart ocr-service
```

3. Increase timeout:
```bash
# Edit systemd service
sudo nano /etc/systemd/system/ocr-service.service

# Update timeout
--timeout 180

sudo systemctl daemon-reload
sudo systemctl restart ocr-service
```

### Issue 4: Webhook Callbacks Failing

**Symptoms:**
- OCR completes but Go API doesn't receive callback
- Callback errors in logs

**Solutions:**

1. Verify webhook URL:
```bash
# Check .env
sudo cat /opt/ocr-service/.env | grep WEBHOOK_URL

# Should be: http://127.0.0.1:8000/api/webhooks/ocr
```

2. Test Go API webhook endpoint:
```bash
curl -X POST http://127.0.0.1:8000/api/webhooks/ocr \
  -H "Content-Type: application/json" \
  -d '{
    "job_id": "test-123",
    "status": "completed",
    "document_id": "doc-456"
  }'
```

3. Check firewall (localhost should be open):
```bash
sudo iptables -L -n | grep 8000
# If blocked: sudo iptables -I INPUT -i lo -j ACCEPT
```

### Issue 5: Nginx Returns 502 Bad Gateway

**Symptoms:**
- External requests fail with 502
- Nginx error log shows connection refused

**Solutions:**

1. Check OCR service is running:
```bash
systemctl status ocr-service
curl http://127.0.0.1:8080/health
```

2. Check Nginx config:
```bash
sudo nginx -t
sudo systemctl status nginx
```

3. Check Nginx error logs:
```bash
sudo tail -f /var/log/nginx/error.log
```

4. Verify upstream connection:
```bash
# Test from Nginx's perspective
sudo -u www-data curl http://127.0.0.1:8080/health
```

### Issue 6: Python Dependencies Missing

**Symptoms:**
```
ModuleNotFoundError: No module named 'docling'
```

**Solutions:**

1. Reinstall dependencies:
```bash
cd /opt/ocr-service
source venv/bin/activate
pip install -r requirements.txt --upgrade --force-reinstall
```

2. Check virtual environment:
```bash
which python3
# Should show: /opt/ocr-service/venv/bin/python3

/opt/ocr-service/venv/bin/pip list | grep docling
```

## Maintenance Tasks

### Update OCR Service Code

```bash
# 1. Stop service
sudo systemctl stop ocr-service

# 2. Backup current version
sudo cp -r /opt/ocr-service /opt/ocr-service.backup.$(date +%Y%m%d)

# 3. Upload new code
scp -r apps/ocr-service/* user@vps:/tmp/ocr-update/

# 4. Copy files
sudo cp -r /tmp/ocr-update/* /opt/ocr-service/

# 5. Update dependencies if changed
cd /opt/ocr-service
source venv/bin/activate
pip install -r requirements.txt --upgrade

# 6. Fix permissions
sudo chown -R www-data:www-data /opt/ocr-service

# 7. Restart service
sudo systemctl start ocr-service

# 8. Verify
curl http://127.0.0.1:8080/health
```

### Rotate Logs Manually

```bash
# Current size
du -sh /var/log/ocr-service/

# Compress old logs
sudo gzip /var/log/ocr-service/*.log.1

# Delete logs older than 30 days
sudo find /var/log/ocr-service/ -name "*.gz" -mtime +30 -delete

# Trigger logrotate
sudo logrotate -f /etc/logrotate.d/ocr-service
```

### Clean Up Old Jobs (if using persistent storage)

```bash
# Check job storage (if applicable)
# This depends on your implementation of JobQueue
# Add cleanup logic if jobs are stored in Redis/DB
```

## Performance Tuning

### Optimize for Large PDFs

```bash
# Edit systemd service
sudo nano /etc/systemd/system/ocr-service.service

# Increase timeout for large files
--timeout 300

# Allow more memory temporarily
MemoryMax=1G

sudo systemctl daemon-reload
sudo systemctl restart ocr-service
```

### Optimize for High Throughput

```bash
# If you have RAM available, increase workers
# Edit systemd service
--workers 2

# Adjust memory limits accordingly
MemoryMax=1.5G

sudo systemctl daemon-reload
sudo systemctl restart ocr-service
```

## Monitoring & Alerts

### Set Up Basic Monitoring

```bash
# Create a simple health check cron
sudo crontab -e

# Add this line to check every 5 minutes
*/5 * * * * curl -f http://127.0.0.1:8080/health || systemctl restart ocr-service
```

### Monitor Disk Space

```bash
# Check disk usage
df -h

# Clean up if needed
sudo apt-get autoremove
sudo apt-get clean
docker system prune -f
```

## Emergency Recovery

### Complete Service Reset

```bash
# 1. Stop everything
sudo systemctl stop ocr-service
sudo systemctl stop nginx

# 2. Kill any stuck processes
sudo pkill -9 gunicorn
sudo pkill -9 uvicorn

# 3. Clear logs if needed
sudo truncate -s 0 /var/log/ocr-service/*.log

# 4. Restart
sudo systemctl start ocr-service
sudo systemctl start nginx

# 5. Verify
systemctl status ocr-service
curl http://127.0.0.1:8080/health
```

### Rollback to Previous Version

```bash
# 1. Stop service
sudo systemctl stop ocr-service

# 2. Restore backup
sudo rm -rf /opt/ocr-service
sudo mv /opt/ocr-service.backup.YYYYMMDD /opt/ocr-service

# 3. Fix permissions
sudo chown -R www-data:www-data /opt/ocr-service

# 4. Start service
sudo systemctl start ocr-service
```

## Useful Commands Reference

```bash
# Service management
systemctl status ocr-service
systemctl start ocr-service
systemctl stop ocr-service
systemctl restart ocr-service
systemctl reload ocr-service

# Logs
journalctl -u ocr-service -f
journalctl -u ocr-service --since "1 hour ago"
journalctl -u ocr-service --since "2024-01-01" --until "2024-01-02"

# Resource usage
htop
free -h
df -h
./monitor-resources.sh

# Network
netstat -tlnp | grep 8080
ss -tlnp | grep 8080

# Test endpoints
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/

# Check environment
sudo cat /opt/ocr-service/.env
systemctl show ocr-service --property=Environment
```

## Getting Help

If issues persist:

1. Collect diagnostic info:
```bash
# Create diagnostic report
cat > /tmp/ocr-diagnostic.txt << DIAG
=== System Info ===
$(uname -a)
$(free -h)
$(df -h)

=== Service Status ===
$(systemctl status ocr-service --no-pager)

=== Recent Logs ===
$(journalctl -u ocr-service -n 100 --no-pager)

=== Resource Usage ===
$(ps aux | grep gunicorn)

=== Network ===
$(netstat -tlnp | grep 8080)

=== Environment ===
$(sudo systemctl show ocr-service --property=Environment)
DIAG

cat /tmp/ocr-diagnostic.txt
```

2. Share the diagnostic report for troubleshooting
