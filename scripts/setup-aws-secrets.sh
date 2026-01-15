#!/bin/bash
# Script to set up AWS Secrets Manager entries for the AI Reimbursement System
# Usage: ./scripts/setup-aws-secrets.sh

set -e

# Configuration
AWS_REGION="ap-southeast-1"
APP_NAME="ai-reimbursement"

echo "================================================"
echo "AWS Secrets Manager Setup for AI Reimbursement"
echo "================================================"
echo ""

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo "❌ AWS CLI not found. Please install it first:"
    echo "   https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
    exit 1
fi

# Check if AWS credentials are configured
if ! aws sts get-caller-identity &> /dev/null; then
    echo "❌ AWS credentials not configured. Please run 'aws configure' first."
    exit 1
fi

echo "✅ AWS CLI configured. Region: $AWS_REGION"
echo ""

# Function to create or update secret
create_or_update_secret() {
    local secret_name=$1
    local secret_description=$2
    local prompt_message=$3
    
    echo "📝 $prompt_message"
    read -sp "Enter value (input hidden): " secret_value
    echo ""
    
    if [ -z "$secret_value" ]; then
        echo "⚠️  Skipping $secret_name (empty value)"
        echo ""
        return
    fi
    
    # Check if secret exists
    if aws secretsmanager describe-secret --secret-id "$secret_name" --region "$AWS_REGION" &> /dev/null; then
        echo "🔄 Updating existing secret: $secret_name"
        aws secretsmanager update-secret \
            --secret-id "$secret_name" \
            --secret-string "$secret_value" \
            --region "$AWS_REGION" &> /dev/null
    else
        echo "✨ Creating new secret: $secret_name"
        aws secretsmanager create-secret \
            --name "$secret_name" \
            --description "$secret_description" \
            --secret-string "$secret_value" \
            --region "$AWS_REGION" &> /dev/null
    fi
    
    echo "✅ Secret $secret_name configured"
    echo ""
}

echo "This script will help you set up secrets in AWS Secrets Manager."
echo "Press Enter to skip any secret you don't want to set now."
echo ""
read -p "Press Enter to continue..."
echo ""

# Lark Credentials
echo "=== Lark Credentials ==="
create_or_update_secret \
    "${APP_NAME}-lark-app-id" \
    "Lark Application ID" \
    "Lark App ID (from Lark Open Platform Console)"

create_or_update_secret \
    "${APP_NAME}-lark-app-secret" \
    "Lark Application Secret" \
    "Lark App Secret (from Lark Open Platform Console)"

create_or_update_secret \
    "${APP_NAME}-lark-verify-token" \
    "Lark Verification Token" \
    "Lark Verify Token (from Lark webhook configuration)"

create_or_update_secret \
    "${APP_NAME}-lark-encrypt-key" \
    "Lark Encryption Key" \
    "Lark Encrypt Key (optional, if encryption enabled)"

# OpenAI Credentials
echo "=== OpenAI Credentials ==="
create_or_update_secret \
    "${APP_NAME}-openai-api-key" \
    "OpenAI API Key" \
    "OpenAI API Key (starts with sk-proj-...)"

# Application Configuration
echo "=== Application Configuration ==="
create_or_update_secret \
    "${APP_NAME}-accountant-email" \
    "External Accountant Email Address" \
    "Accountant Email (e.g., accountant@company.com)"

create_or_update_secret \
    "${APP_NAME}-company-name" \
    "Company Name for Vouchers" \
    "Company Name (e.g., Your Company Ltd.)"

create_or_update_secret \
    "${APP_NAME}-company-tax-id" \
    "Company Tax ID (18 digits)" \
    "Company Tax ID (e.g., 91110000123456789X)"

echo "============================================"
echo "✅ All secrets configured successfully!"
echo "============================================"
echo ""
echo "To view your secrets:"
echo "  aws secretsmanager list-secrets --region $AWS_REGION"
echo ""
echo "To update a secret later:"
echo "  aws secretsmanager update-secret --secret-id SECRET_NAME --secret-string NEW_VALUE --region $AWS_REGION"
echo ""
echo "To delete a secret:"
echo "  aws secretsmanager delete-secret --secret-id SECRET_NAME --region $AWS_REGION"
echo ""
