# Deployment Guide

## Prerequisites

- Go 1.22 or higher
- SQLite 3.42+
- Lark Open Platform account with approval workflow access
- OpenAI API key
- Server with HTTPS support (for webhook endpoint)

## Environment Variables

Set the following environment variables before deployment:

```bash
# Lark Credentials
export LARK_APP_ID="cli_xxxxxxxxxxxxx"
export LARK_APP_SECRET="xxxxxxxxxxxxxxxxxxxx"
export LARK_VERIFY_TOKEN="xxxxxxxxxxxxxxxx"
export LARK_ENCRYPT_KEY="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# OpenAI Credentials
export OPENAI_API_KEY="sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Email Configuration
export ACCOUNTANT_EMAIL="accountant@company.com"
```

## Configuration

1. Copy the example configuration:
```bash
cp configs/config.example.yaml configs/config.yaml
```

2. Edit `configs/config.yaml` with your specific settings:
   - Update company name and tax ID
   - Adjust database path if needed
   - Configure server port
   - Set logging preferences

3. Place your Excel template in `templates/reimbursement_form.xlsx`

## Building

```bash
# Download dependencies
go mod download

# Build the application
go build -o bin/server cmd/server/main.go
```

## Database Setup

The application automatically runs migrations on startup. To manually run migrations:

```bash
./bin/server migrate
```

## Running the Application

### Development Mode

```bash
# Run directly with Go
go run cmd/server/main.go
```

### Production Mode

```bash
# Run the compiled binary
./bin/server
```

### Using Docker (Recommended)

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o server cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite-libs
WORKDIR /root/
COPY --from=builder /app/server .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/templates ./templates
EXPOSE 8080
CMD ["./server"]
```

Build and run:
```bash
docker build -t ai-reimbursement .
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/root/data \
  -v $(pwd)/generated_vouchers:/root/generated_vouchers \
  -e LARK_APP_ID=$LARK_APP_ID \
  -e LARK_APP_SECRET=$LARK_APP_SECRET \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -e ACCOUNTANT_EMAIL=$ACCOUNTANT_EMAIL \
  --name ai-reimbursement \
  ai-reimbursement
```

## Lark Webhook Configuration

1. Log in to Lark Open Platform Console
2. Navigate to your app's settings
3. Configure webhook URL:
   ```
   https://your-domain.com/webhook/approval
   ```
4. Subscribe to events:
   - `approval_instance.created`
   - `approval_instance.approved`
   - `approval_instance.rejected`
   - `approval_instance.status_changed`

## Security Hardening

### 1. HTTPS/TLS

Always run behind a reverse proxy with TLS:

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 2. File Permissions

```bash
chmod 700 data/
chmod 600 configs/config.yaml
chmod 700 generated_vouchers/
```

### 3. Database Backups

Set up automated backups:

```bash
#!/bin/bash
# backup.sh
DATE=$(date +%Y%m%d_%H%M%S)
sqlite3 data/reimbursement.db ".backup data/backup_${DATE}.db"
# Upload to cloud storage
```

Add to crontab:
```
0 */6 * * * /path/to/backup.sh
```

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

### Log Monitoring

Logs are output to stdout in JSON format. Use a log aggregator like:
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Grafana Loki
- CloudWatch Logs

### Metrics

Monitor these key metrics:
- Webhook processing latency
- AI API call latency and failures
- Database connection pool usage
- Voucher generation success rate
- Email delivery success rate

## Troubleshooting

### Webhook not receiving events

1. Check Lark webhook configuration
2. Verify firewall rules allow incoming connections
3. Check webhook signature verification
4. Review application logs for errors

### AI API failures

1. Verify OpenAI API key is valid
2. Check API rate limits
3. Review prompt responses in logs
4. Implement fallback to manual review

### Database issues

1. Check disk space
2. Verify file permissions
3. Review connection pool settings
4. Check for lock contention

### Email delivery failures

1. Verify Lark message API permissions
2. Check accountant email configuration
3. Review Lark admin settings for external email
4. Check attachment file sizes

## Scaling Considerations

### Horizontal Scaling

SQLite is single-writer, so for high concurrency:
- Use connection pooling (already configured)
- Consider migrating to PostgreSQL
- Implement request queuing

### Vertical Scaling

- Increase server resources
- Optimize database queries
- Cache frequently accessed data
- Use CDN for static assets

## Maintenance

### Regular Tasks

1. **Daily**: Monitor logs and error rates
2. **Weekly**: Review AI audit accuracy
3. **Monthly**: Database vacuum and optimize
4. **Quarterly**: Security audit and dependency updates

### Updates

```bash
# Backup database first
./backup.sh

# Pull latest code
git pull

# Update dependencies
go mod tidy

# Rebuild
go build -o bin/server cmd/server/main.go

# Restart service
systemctl restart ai-reimbursement
```

## Support

For issues and questions:
- Check logs in `logs/` directory
- Review architecture documentation
- Contact development team
