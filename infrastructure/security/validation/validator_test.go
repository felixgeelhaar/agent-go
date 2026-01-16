package validation

import (
	"encoding/json"
	"testing"
)

func TestRequired(t *testing.T) {
	rule := Required()

	if err := rule.Validate("value"); err != nil {
		t.Errorf("expected no error for non-empty string, got: %v", err)
	}

	if err := rule.Validate(""); err == nil {
		t.Error("expected error for empty string")
	}

	if err := rule.Validate(nil); err == nil {
		t.Error("expected error for nil value")
	}
}

func TestMaxLength(t *testing.T) {
	rule := MaxLength(5)

	if err := rule.Validate("abc"); err != nil {
		t.Errorf("expected no error for short string, got: %v", err)
	}

	if err := rule.Validate("abcde"); err != nil {
		t.Errorf("expected no error for exact length string, got: %v", err)
	}

	if err := rule.Validate("abcdef"); err == nil {
		t.Error("expected error for long string")
	}

	// Non-string values should pass
	if err := rule.Validate(123); err != nil {
		t.Errorf("expected no error for non-string, got: %v", err)
	}
}

func TestMinLength(t *testing.T) {
	rule := MinLength(3)

	if err := rule.Validate("abc"); err != nil {
		t.Errorf("expected no error for exact length string, got: %v", err)
	}

	if err := rule.Validate("abcd"); err != nil {
		t.Errorf("expected no error for long string, got: %v", err)
	}

	if err := rule.Validate("ab"); err == nil {
		t.Error("expected error for short string")
	}
}

func TestPattern(t *testing.T) {
	rule := Pattern(`^[a-z]+$`)

	if err := rule.Validate("abc"); err != nil {
		t.Errorf("expected no error for matching string, got: %v", err)
	}

	if err := rule.Validate("ABC"); err == nil {
		t.Error("expected error for non-matching string")
	}

	if err := rule.Validate("abc123"); err == nil {
		t.Error("expected error for non-matching string with numbers")
	}
}

func TestAllowedValues(t *testing.T) {
	rule := AllowedValues("apple", "banana", "cherry")

	if err := rule.Validate("apple"); err != nil {
		t.Errorf("expected no error for allowed value, got: %v", err)
	}

	if err := rule.Validate("banana"); err != nil {
		t.Errorf("expected no error for allowed value, got: %v", err)
	}

	if err := rule.Validate("orange"); err == nil {
		t.Error("expected error for disallowed value")
	}
}

func TestRange(t *testing.T) {
	rule := Range(0, 100)

	if err := rule.Validate(float64(50)); err != nil {
		t.Errorf("expected no error for value in range, got: %v", err)
	}

	if err := rule.Validate(float64(0)); err != nil {
		t.Errorf("expected no error for min value, got: %v", err)
	}

	if err := rule.Validate(float64(100)); err != nil {
		t.Errorf("expected no error for max value, got: %v", err)
	}

	if err := rule.Validate(float64(-1)); err == nil {
		t.Error("expected error for value below range")
	}

	if err := rule.Validate(float64(101)); err == nil {
		t.Error("expected error for value above range")
	}

	// Test with int
	if err := rule.Validate(50); err != nil {
		t.Errorf("expected no error for int in range, got: %v", err)
	}
}

func TestNoSQLInjection(t *testing.T) {
	rule := NoSQLInjection()

	safeInputs := []string{
		"simple text",
		"John Doe",
		"user@example.com",
		"123",
	}

	for _, input := range safeInputs {
		if err := rule.Validate(input); err != nil {
			t.Errorf("expected no error for safe input %q, got: %v", input, err)
		}
	}

	dangerousInputs := []string{
		"'; DROP TABLE users--",
		"' OR '1'='1",
		"1; SELECT * FROM users",
		"UNION SELECT * FROM passwords",
		"admin'--",
		"exec(cmd)",
	}

	for _, input := range dangerousInputs {
		if err := rule.Validate(input); err == nil {
			t.Errorf("expected error for SQL injection attempt: %q", input)
		}
	}
}

func TestNoPathTraversal(t *testing.T) {
	rule := NoPathTraversal()

	safeInputs := []string{
		"file.txt",
		"/home/user/file.txt",
		"images/photo.jpg",
	}

	for _, input := range safeInputs {
		if err := rule.Validate(input); err != nil {
			t.Errorf("expected no error for safe path %q, got: %v", input, err)
		}
	}

	dangerousInputs := []string{
		"../etc/passwd",
		"..\\windows\\system32",
		"foo/../../../etc/passwd",
		"%2e%2e/etc/passwd",
	}

	for _, input := range dangerousInputs {
		if err := rule.Validate(input); err == nil {
			t.Errorf("expected error for path traversal attempt: %q", input)
		}
	}
}

func TestNoCommandInjection(t *testing.T) {
	rule := NoCommandInjection()

	safeInputs := []string{
		"simple text",
		"filename.txt",
		"user-name",
	}

	for _, input := range safeInputs {
		if err := rule.Validate(input); err != nil {
			t.Errorf("expected no error for safe input %q, got: %v", input, err)
		}
	}

	dangerousInputs := []string{
		"file.txt; rm -rf /",
		"test | cat /etc/passwd",
		"$(whoami)",
		"`id`",
		"foo && bar",
	}

	for _, input := range dangerousInputs {
		if err := rule.Validate(input); err == nil {
			t.Errorf("expected error for command injection attempt: %q", input)
		}
	}
}

func TestEmail(t *testing.T) {
	rule := Email()

	validEmails := []string{
		"user@example.com",
		"user.name@example.com",
		"user+tag@example.org",
	}

	for _, email := range validEmails {
		if err := rule.Validate(email); err != nil {
			t.Errorf("expected no error for valid email %q, got: %v", email, err)
		}
	}

	invalidEmails := []string{
		"invalid",
		"@example.com",
		"user@",
		"user@.com",
	}

	for _, email := range invalidEmails {
		if err := rule.Validate(email); err == nil {
			t.Errorf("expected error for invalid email: %q", email)
		}
	}
}

func TestURL(t *testing.T) {
	rule := URL()

	validURLs := []string{
		"http://example.com",
		"https://example.com/path",
		"https://example.com/path?query=value",
	}

	for _, url := range validURLs {
		if err := rule.Validate(url); err != nil {
			t.Errorf("expected no error for valid URL %q, got: %v", url, err)
		}
	}

	invalidURLs := []string{
		"not-a-url",
		"example.com",
		"://example.com",
	}

	for _, url := range invalidURLs {
		if err := rule.Validate(url); err == nil {
			t.Errorf("expected error for invalid URL: %q", url)
		}
	}
}

func TestURLWithSchemes(t *testing.T) {
	rule := URL("https")

	if err := rule.Validate("https://example.com"); err != nil {
		t.Errorf("expected no error for https URL, got: %v", err)
	}

	if err := rule.Validate("http://example.com"); err == nil {
		t.Error("expected error for http URL when only https allowed")
	}
}

func TestCustomRule(t *testing.T) {
	rule := Custom("even_number", func(value interface{}) error {
		num, ok := value.(float64)
		if !ok {
			return nil
		}
		if int(num)%2 != 0 {
			return NewValidationError("must be even")
		}
		return nil
	})

	if err := rule.Validate(float64(2)); err != nil {
		t.Errorf("expected no error for even number, got: %v", err)
	}

	if err := rule.Validate(float64(3)); err == nil {
		t.Error("expected error for odd number")
	}
}

func TestSchemaValidation(t *testing.T) {
	schema := NewSchema().
		AddRule("name", Required()).
		AddRule("name", MaxLength(50)).
		AddRule("email", Required()).
		AddRule("email", Email()).
		AddRule("age", Range(0, 150))

	validInput := json.RawMessage(`{
		"name": "John Doe",
		"email": "john@example.com",
		"age": 30
	}`)

	if err := schema.Validate(validInput); err != nil {
		t.Errorf("expected no error for valid input, got: %v", err)
	}

	invalidInput := json.RawMessage(`{
		"name": "",
		"email": "invalid-email",
		"age": 200
	}`)

	err := schema.Validate(invalidInput)
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestSchemaValidationMissingRequired(t *testing.T) {
	schema := NewSchema().
		AddRule("name", Required())

	input := json.RawMessage(`{"other": "value"}`)

	err := schema.Validate(input)
	if err == nil {
		t.Error("expected error for missing required field")
	}
}

func TestSchemaValidationInvalidJSON(t *testing.T) {
	schema := NewSchema()

	input := json.RawMessage(`{invalid json}`)

	err := schema.Validate(input)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRuleNames(t *testing.T) {
	tests := []struct {
		rule Rule
		name string
	}{
		{Required(), "required"},
		{MaxLength(10), "max_length"},
		{MinLength(1), "min_length"},
		{Pattern(`.*`), "pattern"},
		{AllowedValues("a"), "allowed_values"},
		{Range(0, 1), "range"},
		{NoSQLInjection(), "no_sql_injection"},
		{NoPathTraversal(), "no_path_traversal"},
		{NoCommandInjection(), "no_command_injection"},
		{Email(), "email"},
		{URL(), "url"},
	}

	for _, tt := range tests {
		if tt.rule.Name() != tt.name {
			t.Errorf("expected rule name %q, got %q", tt.name, tt.rule.Name())
		}
	}
}

// ValidationError for custom rules
type ValidationError struct {
	message string
}

func (e *ValidationError) Error() string {
	return e.message
}

func NewValidationError(message string) *ValidationError {
	return &ValidationError{message: message}
}
