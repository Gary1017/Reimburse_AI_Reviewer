# AI-Driven Reimbursement Workflow System

An enterprise-grade automated reimbursement workflow system that integrates Lark approval processes with AI-powered auditing, transforming structured form data into legally-binding vouchers compliant with Mainland China accounting regulations.

**Status**: Phase 3 Complete (Attachment Handling), Clean Architecture Refactoring Complete
**Supported Deployments**: Local development, Docker, AWS ECS Fargate (via Terraform)

## 🌟 Key Features

| Feature | Status | Details |
|---------|--------|---------|
| **Webhook Integration** | ✅ Complete | Real-time Lark event streaming with SHA256 signature verification |
| **AI Policy Auditing** | ✅ Complete | OpenAI GPT-4 semantic validation; confidence scoring (Gap: no threshold-based routing) |
| **Market Price Benchmarking** | ✅ Complete | AI-driven pricing validation; flagging outliers (Gap: no feedback loop) |
| **Form Parsing** | ✅ Complete | Multi-item extraction; Chinese field normalization (TRAVEL, MEAL, ACCOMMODATION, etc.) |
| **Invoice Deduplication** | ✅ Complete | Automatic duplicate detection via database lookups |
| **Voucher Generation** | ✅ Complete | Compliant Excel generation; email delivery to accountants |
| **Async Attachments** | ⏳ Planned (Phase 4) | Non-blocking Lark Drive downloads; polling-based sync |
| **Exception Routing** | ⏳ Planned (Phase 4) | Configurable confidence thresholds; low-confidence items routed to human review |
| **Observability** | ⏳ Planned (Phase 4/5) | Prometheus metrics, OpenTelemetry tracing, health checks |
| **10-Year Audit Trail** | ✅ Complete | Immutable transaction logging with ACID guarantees |

---

## 🚀 Development Roadmap

| Phase | Status | Goal | Key Milestones |
|-------|--------|------|-----------------|
| **Phase 1** | ✅ Done | Foundation | Webhook verification, AI policy auditing, SQLite schema |
| **Phase 2** | ✅ Done | Data Extraction | Form parsing, item normalization, Chinese field mapping |
| **Phase 3** | ✅ Done | Attachment Handling | Receipt management, voucher generation, email delivery |
| **Phase 4** | ⏳ Planned | Automation & Reliability | Async downloads, retry logic, health checks, exception routing |
| **Phase 5** | ⏳ Planned | Production & Scale | AWS deployment, observability, immutable audit trail, 1000+ approvals/day |

**For detailed phase breakdowns and task checklists, see [DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md)**.

---

## ⚠️ Known Gaps & Limitations (To be addressed in Phase 4/5)

### Current Shortcomings (Root-Cause Oriented)

1. **Attachment Download Blocking** (ARCH-007)
   - **Issue**: File downloads from Lark Drive are synchronous, blocking webhook response (100ms–5s per file)
   - **Root Cause**: No background worker queue; downloads happen inline in `HandleInstanceCreated`
   - **Impact**: High webhook latency, timeout risk, cascading failures on slow networks
   - **Plan**: Decouple into async worker; mark as PENDING, download in background

2. **Limited AI Confidence Thresholds** (ARCH-008)
   - **Issue**: All items go through Lark approval queue even with high-confidence AI scores
   - **Root Cause**: No configurable threshold logic; no exception-based routing
   - **Impact**: Cannot auto-approve high-confidence items; manual review overhead
   - **Plan**: Introduce configurable thresholds (e.g., >0.95 → AUTO_APPROVED, 0.7–0.95 → IN_REVIEW)

3. **No Observability/Monitoring** (ARCH-009)
   - **Issue**: Cannot detect production bottlenecks or SLO violations in real-time
   - **Root Cause**: Structured logs only; no metrics, tracing, or central alerting
   - **Impact**: Difficult to debug issues; no performance baselines
   - **Plan**: Add Prometheus metrics, OpenTelemetry tracing, health endpoint

---

## Documentation

- **[Architecture Design](docs/ARCHITECTURE.md)**: Clean Architecture layers, dependency rules, and extension points.
- **[Development Plan](docs/DEVELOPMENT_PLAN.md)**: Step-by-step roadmap and links to phase-specific development reports.
- **[Phase Reports](docs/DEVELOPMENT/)**: Detailed reports for each development phase.
- **[CLAUDE.md](CLAUDE.md)**: AI assistant guidance for working with this codebase.

---

## 🚀 Quick Start Guide

### 📋 Prerequisites
- Go 1.22+
- SQLite 3.42+
- Lark Open Platform account with approval workflow access.
- OpenAI API key.

### 🔧 Local Setup
1. **Clone & Config**:
   ```bash
   git clone git@github.com:Gary1017/Reimburse_AI_Reviewer.git
   cd Reimburse_AI_Reviewer
   cp configs/config.example.yaml configs/config.yaml
   cp .env.example .env
   ```
2. **Environment Variables**: Edit `.env` with your `LARK_APP_ID`, `LARK_APP_SECRET`, `OPENAI_API_KEY`, etc.
3. **Template**: Place your Excel template at `templates/reimbursement_form.xlsx`.
4. **Run**:
   ```bash
   go mod download
   go run cmd/server/main.go
   ```

---

## 🛠️ Deployment Guide

### Option 1: Docker (Recommended)
```bash
docker-compose up -d
```

### Option 2: AWS Production (CI/CD)
The system is designed for AWS deployment using GitHub Actions:
1. **GitHub Secrets**: Configure `AWS_ACCESS_KEY_ID`, `LARK_APP_ID`, `OPENAI_API_KEY`, etc., in your repository settings.
2. **Infrastructure**: Use the provided Terraform scripts in `aws/terraform/` to set up ECR, ECS Fargate, and EFS.
3. **Storage**: EFS is required for persistent SQLite data and generated vouchers.
4. **Deploy**: Push to `main` branch to trigger automatic deployment via `.github/workflows/deploy.yml`.

---
**Built with ❤️ for enterprise financial automation**
