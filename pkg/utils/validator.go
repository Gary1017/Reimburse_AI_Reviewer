package utils

import (
	"fmt"
	"regexp"
)

// ValidateEmail validates an email address
func ValidateEmail(email string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format: %s", email)
	}
	return nil
}

// ValidateTaxID validates a Chinese tax ID (18 characters)
func ValidateTaxID(taxID string) error {
	if len(taxID) != 18 {
		return fmt.Errorf("tax ID must be 18 characters: %s", taxID)
	}
	
	// Check if it starts with 91 (standard format)
	if taxID[0:2] != "91" {
		return fmt.Errorf("tax ID must start with 91: %s", taxID)
	}
	
	return nil
}

// ValidateAmount validates a reimbursement amount
func ValidateAmount(amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive: %.2f", amount)
	}
	
	if amount > 100000 {
		return fmt.Errorf("amount exceeds maximum limit: %.2f", amount)
	}
	
	return nil
}

// SanitizeString removes potentially harmful characters
func SanitizeString(s string) string {
	// Remove control characters and potential SQL injection patterns
	sanitized := regexp.MustCompile(`[\x00-\x1f\x7f]`).ReplaceAllString(s, "")
	return sanitized
}
