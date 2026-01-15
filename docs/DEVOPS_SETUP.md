# DevOps Setup Guide - GitHub Actions + AWS Deployment

This guide walks you through setting up a complete CI/CD pipeline using GitHub Actions to deploy the AI Reimbursement System to AWS.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [GitHub Repository Setup](#github-repository-setup)
3. [GitHub Secrets Configuration](#github-secrets-configuration)
4. [AWS Infrastructure Setup](#aws-infrastructure-setup)
5. [Environment Variables Management](#environment-variables-management)
6. [Deployment Workflow](#deployment-workflow)
7. [Monitoring and Troubleshooting](#monitoring-and-troubleshooting)

## Prerequisites

### Required Accounts
- ✅ GitHub account with repository access
- ✅ AWS account with appropriate permissions
- ✅ Lark Open Platform account
- ✅ OpenAI API account

### Required Tools (Local Development)
```bash
# Install AWS CLI
curl "https://awscli.amazonaws.com/AWSCLIV2.pkg" -o "AWSCLIV2.pkg"
sudo installer -pkg AWSCLIV2.pkg -target /

# Install Docker
# Download from https://www.docker.com/products/docker-desktop

# Verify installations
aws --version
docker --version
```

## GitHub Repository Setup

### 1. Initialize Git Repository

```bash
cd /Users/garyjia/workshop/AI_Reimbursement

# Initialize git (if not already done)
git init

# Create .gitignore (already exists)
# Make sure it excludes sensitive files
cat >> .gitignore << 'EOF'
# Secrets
.env
.env.local
.env.*.local
configs/config.yaml

# Data
data/*.db
data/*.db-*
generated_vouchers/*
!generated_vouchers/.gitkeep
logs/*
!logs/.gitkeep

# IDE
.vscode/
.idea/
EOF

# Add remote repository
git remote add origin https://github.com/YOUR_USERNAME/AI_Reimbursement.git

# Commit all files
git add .
git commit -m "Initial commit: AI Reimbursement Workflow System"
git push -u origin main
```

### 2. Create Required Branches

```bash
# Create develop branch
git checkout -b develop
git push -u origin develop

# Back to main
git checkout main
```

## GitHub Secrets Configuration

### 1. Navigate to GitHub Secrets

1. Go to your repository on GitHub
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**

### 2. Add AWS Credentials

Create these secrets:

#### AWS_ACCESS_KEY_ID
```
# Value: Your AWS IAM access key
# Example: AKIAIOSFODNN7EXAMPLE
```

#### AWS_SECRET_ACCESS_KEY
```
# Value: Your AWS IAM secret key
# Example: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### 3. Add Lark Credentials

#### LARK_APP_ID
```
# Value: Your Lark app ID
# Example: cli_a1b2c3d4e5f6g7h8
```

#### LARK_APP_SECRET
```
# Value: Your Lark app secret
# Example: AbCdEfGhIjKlMnOpQrStUvWxYz123456
```

#### LARK_VERIFY_TOKEN
```
# Value: Verification token from Lark console
# Example: AbCdEfGhIjKlMnOp
```

#### LARK_ENCRYPT_KEY
```
# Value: Encryption key from Lark console (if enabled)
# Example: 1234567890abcdef1234567890abcdef
```

### 4. Add OpenAI Credentials

#### OPENAI_API_KEY
```
# Value: Your OpenAI API key
# Example: sk-proj-AbCdEfGhIjKlMnOpQrStUvWxYz1234567890
```

### 5. Add Application Secrets

#### ACCOUNTANT_EMAIL
```
# Value: External accountant's email address
# Example: accountant@company.com
```

#### COMPANY_NAME
```
# Value: Your company name for vouchers
# Example: Your Company Ltd.
```

#### COMPANY_TAX_ID
```
# Value: Chinese tax ID (18 digits)
# Example: 91110000123456789X
```

### Summary of Required GitHub Secrets

| Secret Name | Description | Example |
|-------------|-------------|---------|
| `AWS_ACCESS_KEY_ID` | AWS IAM access key | AKIAIOSFODNN7... |
| `AWS_SECRET_ACCESS_KEY` | AWS IAM secret key | wJalrXUtnFEMI... |
| `LARK_APP_ID` | Lark application ID | cli_a1b2c3d4... |
| `LARK_APP_SECRET` | Lark application secret | AbCdEfGhIjKl... |
| `LARK_VERIFY_TOKEN` | Lark webhook verification token | AbCdEfGh... |
| `LARK_ENCRYPT_KEY` | Lark encryption key (optional) | 12345678... |
| `OPENAI_API_KEY` | OpenAI API key | sk-proj-... |
| `ACCOUNTANT_EMAIL` | Accountant's email | accountant@... |
| `COMPANY_NAME` | Company name | Your Company |
| `COMPANY_TAX_ID` | Tax ID | 91110000... |

## AWS Infrastructure Setup

### Option 1: AWS ECS (Recommended for Production)

#### Step 1: Create ECR Repository

```bash
# Login to AWS
aws configure

# Create ECR repository
aws ecr create-repository \
    --repository-name ai-reimbursement \
    --region ap-southeast-1 \
    --image-scanning-configuration scanOnPush=true
```

#### Step 2: Create ECS Cluster

```bash
# Create ECS cluster
aws ecs create-cluster \
    --cluster-name ai-reimbursement-cluster \
    --region ap-southeast-1
```

#### Step 3: Create Task Definition

Create file `aws/task-definition.json`:

```json
{
  "family": "ai-reimbursement-task",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "512",
  "memory": "1024",
  "executionRoleArn": "arn:aws:iam::YOUR_ACCOUNT_ID:role/ecsTaskExecutionRole",
  "containerDefinitions": [
    {
      "name": "ai-reimbursement",
      "image": "YOUR_ACCOUNT_ID.dkr.ecr.ap-southeast-1.amazonaws.com/ai-reimbursement:latest",
      "essential": true,
      "portMappings": [
        {
          "containerPort": 8080,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {
          "name": "LOG_LEVEL",
          "value": "info"
        }
      ],
      "secrets": [
        {
          "name": "LARK_APP_ID",
          "valueFrom": "arn:aws:secretsmanager:ap-southeast-1:YOUR_ACCOUNT_ID:secret:lark-app-id"
        },
        {
          "name": "LARK_APP_SECRET",
          "valueFrom": "arn:aws:secretsmanager:ap-southeast-1:YOUR_ACCOUNT_ID:secret:lark-app-secret"
        },
        {
          "name": "OPENAI_API_KEY",
          "valueFrom": "arn:aws:secretsmanager:ap-southeast-1:YOUR_ACCOUNT_ID:secret:openai-api-key"
        },
        {
          "name": "ACCOUNTANT_EMAIL",
          "valueFrom": "arn:aws:secretsmanager:ap-southeast-1:YOUR_ACCOUNT_ID:secret:accountant-email"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/ai-reimbursement",
          "awslogs-region": "ap-southeast-1",
          "awslogs-stream-prefix": "ecs"
        }
      },
      "mountPoints": [
        {
          "sourceVolume": "data",
          "containerPath": "/root/data"
        },
        {
          "sourceVolume": "vouchers",
          "containerPath": "/root/generated_vouchers"
        }
      ]
    }
  ],
  "volumes": [
    {
      "name": "data",
      "efsVolumeConfiguration": {
        "fileSystemId": "fs-YOUR_EFS_ID",
        "transitEncryption": "ENABLED"
      }
    },
    {
      "name": "vouchers",
      "efsVolumeConfiguration": {
        "fileSystemId": "fs-YOUR_EFS_ID",
        "rootDirectory": "/vouchers",
        "transitEncryption": "ENABLED"
      }
    }
  ]
}
```

Register the task definition:

```bash
aws ecs register-task-definition \
    --cli-input-json file://aws/task-definition.json
```

#### Step 4: Create AWS Secrets Manager Entries

```bash
# Store Lark credentials
aws secretsmanager create-secret \
    --name lark-app-id \
    --secret-string "cli_your_app_id" \
    --region ap-southeast-1

aws secretsmanager create-secret \
    --name lark-app-secret \
    --secret-string "your_app_secret" \
    --region ap-southeast-1

aws secretsmanager create-secret \
    --name lark-verify-token \
    --secret-string "your_verify_token" \
    --region ap-southeast-1

aws secretsmanager create-secret \
    --name lark-encrypt-key \
    --secret-string "your_encrypt_key" \
    --region ap-southeast-1

# Store OpenAI credentials
aws secretsmanager create-secret \
    --name openai-api-key \
    --secret-string "sk-proj-your_api_key" \
    --region ap-southeast-1

# Store application config
aws secretsmanager create-secret \
    --name accountant-email \
    --secret-string "accountant@company.com" \
    --region ap-southeast-1

aws secretsmanager create-secret \
    --name company-name \
    --secret-string "Your Company Ltd." \
    --region ap-southeast-1

aws secretsmanager create-secret \
    --name company-tax-id \
    --secret-string "91110000123456789X" \
    --region ap-southeast-1
```

#### Step 5: Create EFS for Persistent Storage

```bash
# Create EFS file system
aws efs create-file-system \
    --performance-mode generalPurpose \
    --throughput-mode bursting \
    --encrypted \
    --region ap-southeast-1 \
    --tags Key=Name,Value=ai-reimbursement-efs

# Note the FileSystemId from output
```

#### Step 6: Create ECS Service

```bash
aws ecs create-service \
    --cluster ai-reimbursement-cluster \
    --service-name ai-reimbursement-service \
    --task-definition ai-reimbursement-task \
    --desired-count 1 \
    --launch-type FARGATE \
    --network-configuration "awsvpcConfiguration={subnets=[subnet-xxxxx],securityGroups=[sg-xxxxx],assignPublicIp=ENABLED}" \
    --region ap-southeast-1
```

#### Step 7: Create Application Load Balancer (Optional)

```bash
# Create ALB
aws elbv2 create-load-balancer \
    --name ai-reimbursement-alb \
    --subnets subnet-xxxxx subnet-yyyyy \
    --security-groups sg-xxxxx \
    --region ap-southeast-1

# Create target group
aws elbv2 create-target-group \
    --name ai-reimbursement-tg \
    --protocol HTTP \
    --port 8080 \
    --vpc-id vpc-xxxxx \
    --target-type ip \
    --health-check-path /health \
    --region ap-southeast-1

# Create listener
aws elbv2 create-listener \
    --load-balancer-arn arn:aws:elasticloadbalancing:... \
    --protocol HTTPS \
    --port 443 \
    --certificates CertificateArn=arn:aws:acm:... \
    --default-actions Type=forward,TargetGroupArn=arn:aws:elasticloadbalancing:... \
    --region ap-southeast-1
```

### Option 2: AWS EC2 (Simpler Setup)

#### Step 1: Create EC2 Instance

```bash
# Create security group
aws ec2 create-security-group \
    --group-name ai-reimbursement-sg \
    --description "Security group for AI Reimbursement" \
    --vpc-id vpc-xxxxx

# Add inbound rules
aws ec2 authorize-security-group-ingress \
    --group-id sg-xxxxx \
    --protocol tcp \
    --port 8080 \
    --cidr 0.0.0.0/0

aws ec2 authorize-security-group-ingress \
    --group-id sg-xxxxx \
    --protocol tcp \
    --port 443 \
    --cidr 0.0.0.0/0

# Launch EC2 instance
aws ec2 run-instances \
    --image-id ami-xxxxx \
    --instance-type t3.medium \
    --key-name your-key-pair \
    --security-group-ids sg-xxxxx \
    --subnet-id subnet-xxxxx \
    --user-data file://aws/user-data.sh
```

Create `aws/user-data.sh`:

```bash
#!/bin/bash
# Install Docker
yum update -y
yum install -y docker
systemctl start docker
systemctl enable docker
usermod -a -G docker ec2-user

# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# Install AWS CLI
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
./aws/install

# Create application directory
mkdir -p /opt/ai-reimbursement
cd /opt/ai-reimbursement

# Pull and run Docker container
aws ecr get-login-password --region ap-southeast-1 | docker login --username AWS --password-stdin YOUR_ACCOUNT_ID.dkr.ecr.ap-southeast-1.amazonaws.com
docker pull YOUR_ACCOUNT_ID.dkr.ecr.ap-southeast-1.amazonaws.com/ai-reimbursement:latest
docker run -d \
  --name ai-reimbursement \
  -p 8080:8080 \
  -v /opt/ai-reimbursement/data:/root/data \
  -v /opt/ai-reimbursement/vouchers:/root/generated_vouchers \
  -e LARK_APP_ID="$(aws secretsmanager get-secret-value --secret-id lark-app-id --query SecretString --output text)" \
  -e LARK_APP_SECRET="$(aws secretsmanager get-secret-value --secret-id lark-app-secret --query SecretString --output text)" \
  -e OPENAI_API_KEY="$(aws secretsmanager get-secret-value --secret-id openai-api-key --query SecretString --output text)" \
  -e ACCOUNTANT_EMAIL="$(aws secretsmanager get-secret-value --secret-id accountant-email --query SecretString --output text)" \
  --restart unless-stopped \
  YOUR_ACCOUNT_ID.dkr.ecr.ap-southeast-1.amazonaws.com/ai-reimbursement:latest
```

## Environment Variables Management

### Local Development (.env file)

Create `.env` file (never commit this!):

```bash
# Lark Configuration
LARK_APP_ID=cli_your_app_id
LARK_APP_SECRET=your_app_secret
LARK_VERIFY_TOKEN=your_verify_token
LARK_ENCRYPT_KEY=your_encrypt_key

# OpenAI Configuration
OPENAI_API_KEY=sk-proj-your_api_key

# Application Configuration
ACCOUNTANT_EMAIL=accountant@company.com
COMPANY_NAME=Your Company Ltd.
COMPANY_TAX_ID=91110000123456789X
```

Load and run locally:

```bash
# Load environment variables
source .env

# Or use docker-compose
docker-compose --env-file .env up
```

### GitHub Actions (GitHub Secrets)

Environment variables are injected from GitHub Secrets in the CI/CD pipeline. See `.github/workflows/deploy.yml`.

### AWS Deployment (Secrets Manager)

Production secrets are stored in AWS Secrets Manager and injected into containers via task definition.

## Deployment Workflow

### Automatic Deployment

1. **Push to `develop` branch**: Runs tests and builds Docker image
2. **Push to `main` branch**: Runs tests, builds image, and deploys to AWS
3. **Pull Request**: Runs tests only

### Manual Deployment

```bash
# Tag and push manually
git tag v1.0.0
git push origin v1.0.0

# Trigger deployment via GitHub Actions
# Or deploy manually:
aws ecs update-service \
    --cluster ai-reimbursement-cluster \
    --service ai-reimbursement-service \
    --force-new-deployment \
    --region ap-southeast-1
```

## Monitoring and Troubleshooting

### View Logs

```bash
# ECS logs via CloudWatch
aws logs tail /ecs/ai-reimbursement --follow

# EC2 logs
ssh ec2-user@your-instance-ip
docker logs -f ai-reimbursement
```

### Check Service Health

```bash
# Health check
curl https://your-domain.com/health

# ECS service status
aws ecs describe-services \
    --cluster ai-reimbursement-cluster \
    --services ai-reimbursement-service
```

### Rollback Deployment

```bash
# Rollback to previous task definition
aws ecs update-service \
    --cluster ai-reimbursement-cluster \
    --service ai-reimbursement-service \
    --task-definition ai-reimbursement-task:PREVIOUS_REVISION \
    --force-new-deployment
```

## Security Best Practices

1. ✅ **Never commit secrets** to Git
2. ✅ **Use AWS Secrets Manager** for production secrets
3. ✅ **Enable encryption** at rest and in transit
4. ✅ **Use IAM roles** instead of access keys where possible
5. ✅ **Rotate secrets** regularly (every 90 days)
6. ✅ **Enable CloudTrail** for audit logging
7. ✅ **Use VPC** and security groups properly
8. ✅ **Scan Docker images** for vulnerabilities

## Next Steps

1. ✅ Set up GitHub repository and secrets
2. ✅ Configure AWS infrastructure
3. ✅ Push code to trigger first deployment
4. ✅ Configure Lark webhook to point to AWS endpoint
5. ✅ Test the complete flow
6. ✅ Set up monitoring and alerts

## Support

For issues with the DevOps setup:
- Check GitHub Actions logs
- Review AWS CloudWatch logs
- Verify all secrets are set correctly
- Ensure IAM permissions are correct
