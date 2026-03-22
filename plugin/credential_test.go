package teradata

import (
	"strings"
	"testing"
)

func TestGeneratePassword(t *testing.T) {
	for i := 0; i < 100; i++ {
		password := generatePassword()

		if len(password) < passwordMinLength {
			t.Errorf("password too short: got %d, min %d", len(password), passwordMinLength)
		}
		if len(password) > passwordMaxLength {
			t.Errorf("password too long: got %d, max %d", len(password), passwordMaxLength)
		}

		if !strings.ContainsAny(password, lowerChars) {
			t.Errorf("password missing lowercase char: %s", password)
		}
		if !strings.ContainsAny(password, upperChars) {
			t.Errorf("password missing uppercase char: %s", password)
		}
		if !strings.ContainsAny(password, digitChars) {
			t.Errorf("password missing digit char: %s", password)
		}
		if !strings.ContainsAny(password, specialChars) {
			t.Errorf("password missing special char: %s", password)
		}
	}
}

func TestGeneratePasswordLength(t *testing.T) {
	lengths := make(map[int]bool)
	for i := 0; i < 1000; i++ {
		password := generatePassword()
		lengths[len(password)] = true
	}

	minFound, maxFound := 32, 16
	for l := range lengths {
		if l < minFound {
			minFound = l
		}
		if l > maxFound {
			maxFound = l
		}
	}

	if minFound != passwordMinLength {
		t.Errorf("expected min length %d, got %d", passwordMinLength, minFound)
	}
	if maxFound != passwordMaxLength {
		t.Errorf("expected max length %d, got %d", passwordMaxLength, maxFound)
	}
}

func TestGeneratePasswordCharacterSets(t *testing.T) {
	tests := []struct {
		name        string
		charSet     string
		shouldHave  string
		description string
	}{
		{"lowercase", lowerChars, lowerChars, "should contain lowercase"},
		{"uppercase", upperChars, upperChars, "should contain uppercase"},
		{"digits", digitChars, digitChars, "should contain digits"},
		{"special", specialChars, specialChars, "should contain special"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for i := 0; i < 100; i++ {
				password := generatePassword()
				if strings.ContainsAny(password, tt.shouldHave) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s in generated passwords", tt.description)
			}
		})
	}
}

func TestEnsurePasswordRequirements(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantLen  int
	}{
		{"all present", "aB1!aB1!aB1!aB1!", 16},
		{"missing lower", "ABCD1234!@#$abcd", 16},
		{"missing upper", "abcd1234!@#$ABCD", 16},
		{"missing digit", "abcdABCD!@#$1234", 16},
		{"missing special", "abcdABCD1234!@#$", 16},
		{"all missing", "abcdABCD1234!@#$", 16},
		{"single char all", "aB1!", 4},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensurePasswordRequirements(tt.password)
			if len(result) != tt.wantLen {
				t.Errorf("ensurePasswordRequirements(%q): got len %d, want %d",
					tt.password, len(result), tt.wantLen)
			}
		})
	}
}

func TestEnsurePasswordRequirementsContainsAllTypes(t *testing.T) {
	passwords := []string{
		"abcdefghijklmnop",
		"ABCDEFGHIJKLMNOP",
		"0123456789012345",
		"!@#$%^&*()_+-=[]",
	}

	for _, pw := range passwords {
		result := ensurePasswordRequirements(pw)
		if !strings.ContainsAny(result, lowerChars) {
			t.Errorf("result missing lowercase: %s", result)
		}
		if !strings.ContainsAny(result, upperChars) {
			t.Errorf("result missing uppercase: %s", result)
		}
		if !strings.ContainsAny(result, digitChars) {
			t.Errorf("result missing digit: %s", result)
		}
		if !strings.ContainsAny(result, specialChars) {
			t.Errorf("result missing special: %s", result)
		}
	}
}

func TestGenerateUsername(t *testing.T) {
	for i := 0; i < 100; i++ {
		username := generateUsername("test", "")

		if !strings.HasPrefix(username, "test_") {
			t.Errorf("username should start with 'test_', got: %s", username)
		}

		parts := strings.Split(username, "_")
		if len(parts) != 2 {
			t.Errorf("username should have one underscore, got: %s", username)
		}

		if len(parts[1]) != 16 {
			t.Errorf("suffix should be 16 hex chars (8 bytes), got %d: %s",
				len(parts[1]), username)
		}
	}
}

func TestGenerateUsernameDefaultPrefix(t *testing.T) {
	for i := 0; i < 100; i++ {
		username := generateUsername("", "")

		if !strings.HasPrefix(username, "vault_") {
			t.Errorf("username should default to 'vault_' prefix, got: %s", username)
		}
	}
}

func TestGenerateUsernameNilPrefix(t *testing.T) {
	for i := 0; i < 100; i++ {
		username := generateUsername("", "")

		parts := strings.Split(username, "_")
		if len(parts) != 2 {
			t.Errorf("username should have one underscore, got: %s", username)
		}

		if len(parts[1]) != 16 {
			t.Errorf("suffix should be 16 hex chars, got %d: %s",
				len(parts[1]), username)
		}
	}
}

func TestGenerateUsernameUniqueness(t *testing.T) {
	usernames := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		username := generateUsername("vault", "")
		if usernames[username] {
			t.Errorf("duplicate username generated: %s", username)
		}
		usernames[username] = true
	}
}

func TestGenerateUsernameHexSuffix(t *testing.T) {
	for i := 0; i < 100; i++ {
		username := generateUsername("test", "")
		parts := strings.Split(username, "_")
		suffix := parts[1]

		for _, c := range suffix {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("username suffix should be hex: %s", username)
				break
			}
		}
	}
}

func TestGenerateUsernameCustomPrefix(t *testing.T) {
	tests := []struct {
		prefix string
	}{
		{"myapp"},
		{"admin"},
		{"user"},
		{"db"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			username := generateUsername(tt.prefix, "")
			expectedPrefix := tt.prefix + "_"
			if !strings.HasPrefix(username, expectedPrefix) {
				t.Errorf("expected prefix %q, got %q", expectedPrefix, username)
			}
		})
	}
}
