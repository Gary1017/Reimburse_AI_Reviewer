# AI Prompt Configuration

## Overview

All AI prompts and model parameters used by the OpenAI auditor have been externalized to a YAML configuration file. This allows you to easily update prompts, adjust model behavior (temperature, max_tokens), and tune AI performance without modifying the codebase.

## Configuration File

**Location**: `configs/prompts.yaml`

This file contains all prompts used by the system, organized by audit type:

### Structure

```yaml
policy_audit:
  # Model parameters
  temperature: 0.3        # Controls randomness (0.0-2.0)
  max_tokens: 1000        # Maximum tokens in response

  # Prompts
  system: |
    System prompt for policy auditing
  user_template: |
    User prompt template with variables like {{.ItemType}}, {{.Amount}}, etc.

price_audit:
  temperature: 0.3
  max_tokens: 1000
  system: |
    System prompt for price auditing
  user_template: |
    User prompt template with variables

invoice_extraction:
  temperature: 0.1        # Lower for accuracy
  max_tokens: 4096        # Higher for detailed extraction
  system: |
    System prompt for invoice extraction
  user_template: |
    User prompt template for vision API
```

### Available Configuration Types

1. **policy_audit**
   - Validates reimbursement items against company policies
   - **Model parameters:**
     - `temperature: 0.3` - Balanced creativity/consistency
     - `max_tokens: 1000` - Sufficient for detailed reasoning
   - **Available variables:**
     - `{{.Policies}}` - Company policies JSON
     - `{{.ItemType}}` - Type of reimbursement item
     - `{{.Description}}` - Item description
     - `{{.Amount}}` - Item amount
     - `{{.Currency}}` - Currency code
     - `{{.InvoiceInfo}}` - Invoice information if available

2. **price_audit**
   - Benchmarks prices against market rates
   - **Model parameters:**
     - `temperature: 0.3` - Balanced for price estimation
     - `max_tokens: 1000` - Adequate for price analysis
   - **Available variables:**
     - `{{.ItemType}}` - Category of expense
     - `{{.Description}}` - Expense description
     - `{{.Amount}}` - Submitted amount
     - `{{.Currency}}` - Currency code
     - `{{.InvoiceInfo}}` - Invoice details if available
     - `{{.SubmittedPrice}}` - Amount being claimed

3. **invoice_extraction**
   - Extracts data from Chinese invoice images
   - **Model parameters:**
     - `temperature: 0.1` - Very low for maximum accuracy
     - `max_tokens: 4096` - High limit for detailed invoice data
   - **Variables:** None (static prompt for vision API)

## How to Update Configuration

### 1. Edit the YAML File

You can update both prompts AND model parameters:

Simply edit `configs/prompts.yaml` with your preferred text editor:

```bash
vim configs/prompts.yaml
# or
nano configs/prompts.yaml
```

### 2. Tune Model Parameters

Adjust temperature and token limits for optimal performance:

```yaml
policy_audit:
  # Higher temperature (0.5-0.8) = more creative, varied responses
  # Lower temperature (0.0-0.3) = more consistent, deterministic
  temperature: 0.3

  # Increase if responses are being cut off
  # Decrease to save costs and reduce latency
  max_tokens: 1000
```

**Temperature Guidelines:**
- `0.0-0.2`: Deterministic, best for extraction and classification
- `0.3-0.5`: Balanced, good for most tasks
- `0.6-1.0`: Creative, useful for brainstorming
- `1.1-2.0`: Very creative, rarely needed

### 3. Update System Prompts

System prompts define the AI's role and behavior:

```yaml
policy_audit:
  system: |
    You are a financial compliance auditor for a Chinese enterprise.
    Evaluate reimbursement items against company policies.
    Always respond with valid JSON wrapped in ```json and ``` markers.
```

### 4. Update User Prompt Templates

User prompts use Go template syntax for variable substitution:

```yaml
policy_audit:
  user_template: |
    Evaluate this reimbursement item against company policies:

    **Company Policies:**
    {{.Policies}}

    **Reimbursement Item:**
    - Type: {{.ItemType}}
    - Description: {{.Description}}
    - Amount: {{.Amount}} {{.Currency}}
```

### 5. Restart the Application

After updating configuration, restart the server for changes to take effect:

```bash
make run
# or
./bin/server
```

## Implementation Details

### Architecture

The prompt system follows Clean Architecture principles:

```
configs/prompts.yaml
        ↓
internal/infrastructure/external/openai/prompts.go (LoadPrompts)
        ↓
internal/infrastructure/external/openai/auditor.go (Uses prompts)
        ↓
internal/container/providers.go (Wires everything)
```

### Key Components

1. **PromptConfig** (`openai/prompts.go`)
   - Struct that holds all prompt configurations
   - `LoadPrompts()` function to load from YAML
   - `renderTemplate()` function for variable substitution

2. **Auditor** (`openai/auditor.go`)
   - Updated to accept `*PromptConfig` in constructor
   - Uses templates from config instead of hardcoded strings
   - Methods `buildPolicyPrompt()` and `buildPricePrompt()` now return `(string, error)`

3. **Container** (`container/providers.go`, `container/container.go`)
   - `ProvideAIAuditor()` loads prompts and passes to auditor
   - Called during container initialization

### No Import Cycles

The prompt configuration is in the `openai` package to avoid import cycles:

```
openai package (has prompts.go) ✓
  ↑
container package (loads prompts)
  ↑
config package (no dependency on openai)
```

## Best Practices

### 1. Start with Conservative Parameters

```yaml
# Good - Start conservative, then experiment
temperature: 0.3
max_tokens: 1000

# Risky - High temperature can be unpredictable
temperature: 1.5
max_tokens: 100  # May cut off responses
```

### 2. Keep Prompts Clear and Specific

```yaml
# Good - Clear instructions
system: |
  You are a financial compliance auditor.
  Always respond with valid JSON.

# Bad - Vague instructions
system: |
  You are an AI assistant.
```

### 3. Use Templates for Dynamic Content

```yaml
# Good - Uses template variables
user_template: |
  Expense: {{.Amount}} {{.Currency}}
  Category: {{.ItemType}}

# Bad - Hardcoded values
user_template: |
  Expense: 1000 USD
  Category: TRAVEL
```

### 4. Include Response Format Requirements

```yaml
user_template: |
  ...

  **Required Response Format (JSON):**
  {
    "field1": type,
    "field2": type
  }
```

### 5. Document Expected Variables

Add comments in the YAML file:

```yaml
policy_audit:
  # Variables: Policies, ItemType, Description, Amount, Currency, InvoiceInfo
  user_template: |
    ...
```

## Testing Prompt Changes

### Manual Testing

Use the test command to verify prompts:

```bash
go run cmd/test-gpt-connection/main.go --api-key YOUR_KEY
```

### Integration Testing

Run the full system and check logs:

```bash
make run

# In another terminal
tail -f logs/server.log
```

### Rollback

If prompts cause issues, use git to revert:

```bash
git checkout HEAD -- configs/prompts.yaml
make run
```

## Optimization Tips

### For Policy Auditing
```yaml
policy_audit:
  temperature: 0.3      # Balanced for rule-based reasoning
  max_tokens: 1500      # Increase if violations lists are truncated
```

### For Price Auditing
```yaml
price_audit:
  temperature: 0.4      # Slightly higher for market estimation
  max_tokens: 1000      # Usually sufficient for price analysis
```

### For Invoice Extraction
```yaml
invoice_extraction:
  temperature: 0.0      # Use 0.0 for maximum accuracy
  max_tokens: 4096      # Keep high to capture all invoice fields
```

## Common Issues

### 1. Responses Being Cut Off

**Symptom**: JSON responses incomplete or malformed

**Solution**: Increase `max_tokens` for that audit type

```yaml
policy_audit:
  max_tokens: 2000  # Increased from 1000
```

### 2. Inconsistent Results

**Symptom**: Different results for same input

**Solution**: Lower temperature for more consistency

```yaml
price_audit:
  temperature: 0.1  # More deterministic
```

### 3. Template Syntax Errors

**Error**: `failed to parse template`

**Solution**: Check template syntax - variables must be `{{.VariableName}}`

### 4. Missing Variables

**Error**: `can't evaluate field X`

**Solution**: Ensure all variables in template match those provided in code

### 5. YAML Parsing Errors

**Error**: `failed to unmarshal prompts`

**Solution**: Validate YAML syntax (indentation, quotes, pipes)

### 6. Invalid Parameter Values

**Error**: API errors or unexpected behavior

**Solution**: Check parameter ranges:
- `temperature`: Must be 0.0-2.0
- `max_tokens`: Must be positive integer, reasonable limit ~8000

## Future Enhancements

Potential improvements to the prompt system:

1. **Prompt Versioning** - Track prompt versions for A/B testing
2. **Hot Reload** - Update prompts without restart
3. **Per-Environment Prompts** - Different prompts for dev/staging/prod
4. **Prompt Templates Library** - Reusable template fragments
5. **Validation** - Validate prompts against schema on load

## References

- Go template syntax: https://pkg.go.dev/text/template
- YAML specification: https://yaml.org/spec/1.2/spec.html
- OpenAI best practices: https://platform.openai.com/docs/guides/prompt-engineering
