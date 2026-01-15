# Quick Start Guide - DevOps Setup

This is a streamlined guide to get your AI Reimbursement System deployed to AWS with GitHub Actions CI/CD in under 30 minutes.

## Prerequisites Checklist

- [ ] GitHub account
- [ ] AWS account with admin access
- [ ] Lark Open Platform credentials
- [ ] OpenAI API key
- [ ] Excel template ready

## Step 1: Push Code to GitHub (5 minutes)

```bash
cd /Users/garyjia/workshop/AI_Reimbursement

# Initialize git repository
git init
git add .
git commit -m "Initial commit: AI Reimbursement System"

# Create GitHub repository at: https://github.com/new
# Then link and push:
git remote add origin https://github.com/YOUR_USERNAME/AI_Reimbursement.git
git branch -M main
git push -u origin main
```

## Step 2: Configure GitHub Secrets (5 minutes)

Go to your repository → **Settings** → **Secrets and variables** → **Actions** → **New repository secret**

Add these 10 secrets:

| Secret Name | Where to Get It |
|-------------|-----------------|
| `AWS_ACCESS_KEY_ID` | AWS IAM Console → Users → Security credentials |
| `AWS_SECRET_ACCESS_KEY` | AWS IAM Console → Users → Security credentials |
| `LARK_APP_ID` | Lark Open Platform Console → Your App |
| `LARK_APP_SECRET` | Lark Open Platform Console → Your App |
| `LARK_VERIFY_TOKEN` | Lark Open Platform Console → Events & Callbacks |
| `LARK_ENCRYPT_KEY` | Lark Open Platform Console → Events & Callbacks (if enabled) |
| `OPENAI_API_KEY` | OpenAI Platform → API keys |
| `ACCOUNTANT_EMAIL` | Your external accountant's email |
| `COMPANY_NAME` | Your company name |
| `COMPANY_TAX_ID` | Your Chinese tax ID (18 digits) |

## Step 3: Set Up AWS Infrastructure (10 minutes)

### Option A: Using Script (Easiest)

```bash
# Run the automated setup script
./scripts/setup-aws-secrets.sh

# This will:
# 1. Create AWS Secrets Manager entries
# 2. Prompt you for all credentials
# 3. Store them securely in AWS
```

### Option B: Using Terraform (Recommended for Production)

```bash
cd aws/terraform

# Initialize Terraform
terraform init

# Review the plan
terraform plan

# Apply the configuration
terraform apply

# Note down the outputs (ECR URL, ECS cluster name, etc.)
```

### Option C: Manual Setup (Most Control)

Follow the detailed steps in `docs/DEVOPS_SETUP.md`

## Step 4: Create ECS Task Definition (5 minutes)

Update `aws/task-definition.json` with your AWS account ID and resource ARNs:

```bash
# Get your AWS account ID
aws sts get-caller-identity --query Account --output text

# Get your EFS file system ID (if using Terraform, it's in the outputs)
terraform output efs_file_system_id
```

Then register the task definition:

```bash
cd aws
aws ecs register-task-definition --cli-input-json file://task-definition.json
```

## Step 5: Trigger First Deployment (2 minutes)

```bash
# Make a small change to trigger the pipeline
echo "# Deployment triggered" >> README.md
git add README.md
git commit -m "Trigger first deployment"
git push origin main
```

Go to GitHub → **Actions** tab to watch the deployment progress.

## Step 6: Configure Lark Webhook (3 minutes)

1. Get your AWS Application Load Balancer URL or EC2 public IP
2. Go to Lark Open Platform Console
3. Navigate to **Events & Callbacks**
4. Set Request URL: `https://your-aws-endpoint.com/webhook/approval`
5. Subscribe to events:
   - `approval.approval_instance.created`
   - `approval.approval_instance.approved`
   - `approval.approval_instance.rejected`
   - `approval.approval_instance.status_changed`

## Step 7: Verify Deployment

```bash
# Check if the service is healthy
curl https://your-aws-endpoint.com/health

# Expected response:
# {"status":"healthy","service":"ai-reimbursement","time":"2026-01-14T10:30:00Z"}

# View logs
aws logs tail /ecs/ai-reimbursement --follow

# Check ECS service status
aws ecs describe-services \
    --cluster ai-reimbursement-cluster \
    --services ai-reimbursement-service
```

## Step 8: Test End-to-End Flow

1. Create a test reimbursement approval in Lark
2. Check CloudWatch Logs for webhook receipt
3. Monitor database for new entry
4. Wait for approval
5. Check `generated_vouchers/` in EFS for generated Excel file
6. Verify email sent to accountant

## Troubleshooting

### GitHub Actions fails

```bash
# Check the Actions tab for error messages
# Common issues:
# - Missing GitHub Secrets → Add them in Settings
# - Wrong AWS credentials → Verify IAM permissions
# - ECR repository doesn't exist → Run Terraform or create manually
```

### ECS service won't start

```bash
# Check ECS task logs
aws logs tail /ecs/ai-reimbursement --follow

# Common issues:
# - Missing AWS Secrets → Run setup-aws-secrets.sh
# - Wrong task definition → Update and re-register
# - EFS mount issues → Check security groups
```

### Webhook not receiving events

```bash
# Check Lark webhook configuration
# Verify the URL is correct and accessible
# Check security groups allow inbound HTTPS

# Test webhook manually:
curl -X POST https://your-endpoint.com/webhook/approval \
  -H "Content-Type: application/json" \
  -d '{"type":"url_verification","challenge":"test","token":"your_verify_token"}'
```

## Environment Variables Quick Reference

### Local Development (.env)
```bash
LARK_APP_ID=cli_xxxxx
LARK_APP_SECRET=xxxxx
OPENAI_API_KEY=sk-proj-xxxxx
ACCOUNTANT_EMAIL=accountant@company.com
```

### GitHub Actions (GitHub Secrets)
All secrets stored in repository settings

### AWS Production (Secrets Manager)
All secrets stored in AWS Secrets Manager, injected via ECS task definition

## Next Steps

- [ ] Set up monitoring alerts (CloudWatch Alarms)
- [ ] Configure backup strategy for EFS
- [ ] Set up log aggregation (CloudWatch Insights)
- [ ] Implement blue-green deployment
- [ ] Add staging environment
- [ ] Set up cost monitoring

## Architecture Diagram

```
┌─────────────────────────────────────────────────────┐
│              GitHub Repository                      │
│  (Code + Secrets)                                   │
└────────────────┬────────────────────────────────────┘
                 │
                 │ Push to main
                 ▼
┌─────────────────────────────────────────────────────┐
│           GitHub Actions CI/CD                      │
│  • Run tests                                        │
│  • Build Docker image                               │
│  • Push to AWS ECR                                  │
│  • Update ECS service                               │
└────────────────┬────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────┐
│              AWS Infrastructure                     │
│  ┌──────────────┐    ┌──────────────┐             │
│  │     ECR      │───▶│  ECS Fargate │             │
│  │  (Images)    │    │  (Container) │             │
│  └──────────────┘    └──────┬───────┘             │
│                              │                      │
│  ┌──────────────┐    ┌──────▼───────┐             │
│  │  Secrets     │───▶│  Application │             │
│  │  Manager     │    │   Running    │             │
│  └──────────────┘    └──────┬───────┘             │
│                              │                      │
│  ┌──────────────┐    ┌──────▼───────┐             │
│  │     EFS      │◀───│   SQLite +   │             │
│  │ (Persistent) │    │   Vouchers   │             │
│  └──────────────┘    └──────────────┘             │
└─────────────────────────────────────────────────────┘
                 ▲                 │
                 │                 ▼
         ┌───────┴────┐    ┌──────────────┐
         │    Lark    │    │  Accountant  │
         │  Webhooks  │    │    Email     │
         └────────────┘    └──────────────┘
```

## Support

- 📖 Full documentation: `docs/DEVOPS_SETUP.md`
- 🏗️ Architecture details: `docs/ARCHITECTURE.md`
- 🔐 Security guide: `docs/SECURITY.md`
- 🚀 Deployment guide: `docs/DEPLOYMENT.md`

**Need help?** Check the logs first, then review the documentation above.
