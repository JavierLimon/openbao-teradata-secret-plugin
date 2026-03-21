package security

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantError bool
		errType   error
	}{
		{
			name:      "empty password",
			password:  "",
			wantError: true,
			errType:   ErrInvalidPassword,
		},
		{
			name:      "valid password",
			password:  "MySecureP@ssw0rd",
			wantError: false,
		},
		{
			name:      "password with single quote",
			password:  "pass'word",
			wantError: true,
			errType:   ErrInvalidPassword,
		},
		{
			name:      "password with double quote",
			password:  `pass"word`,
			wantError: true,
			errType:   ErrInvalidPassword,
		},
		{
			name:      "password with semicolon - allowed as special char",
			password:  "pass;word",
			wantError: false,
		},
		{
			name:      "password with backslash",
			password:  `pass\word`,
			wantError: true,
			errType:   ErrInvalidPassword,
		},
		{
			name:      "password with null byte",
			password:  "pass\x00word",
			wantError: true,
			errType:   ErrInvalidPassword,
		},
		{
			name:      "password too long",
			password:  strings.Repeat("a", MaxPasswordLength+1),
			wantError: true,
			errType:   ErrPasswordTooLong,
		},
		{
			name:      "valid password at max length",
			password:  strings.Repeat("a", MaxPasswordLength),
			wantError: false,
		},
		{
			name:      "password with special chars including semicolon",
			password:  "!@#$%^&*()_+-=[]{}|;:,.<>?",
			wantError: false,
		},
		{
			name:      "password with only special chars",
			password:  "!@#$%^&*()",
			wantError: false,
		},
		{
			name:      "password with space - not allowed",
			password:  "pass word",
			wantError: true,
			errType:   ErrInvalidPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidatePassword() expected error for %q, got nil", tt.password)
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("ValidatePassword() error = %v, want error type %v", err, tt.errType)
				}
			} else if err != nil {
				t.Errorf("ValidatePassword() unexpected error for %q: %v", tt.password, err)
			}
		})
	}
}

func TestValidateSQLStatement(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		wantError bool
		errType   error
	}{
		{
			name:      "empty statement",
			statement: "",
			wantError: true,
			errType:   ErrEmptyStatement,
		},
		{
			name:      "whitespace only",
			statement: "   ",
			wantError: true,
			errType:   ErrEmptyStatement,
		},
		{
			name:      "valid GRANT SELECT statement",
			statement: "GRANT SELECT ON mydb TO myuser",
			wantError: false,
		},
		{
			name:      "valid GRANT SELECT lowercase",
			statement: "grant select on mydb to myuser",
			wantError: false,
		},
		{
			name:      "valid GRANT with multiple privileges",
			statement: "GRANT SELECT, INSERT, UPDATE ON mydb TO myuser",
			wantError: false,
		},
		{
			name:      "valid REVOKE statement",
			statement: "REVOKE SELECT ON mydb FROM myuser",
			wantError: false,
		},
		{
			name:      "statement with SQL comment",
			statement: "GRANT SELECT ON mydb TO myuser -- comment",
			wantError: true,
			errType:   ErrCommentDetected,
		},
		{
			name:      "statement with block comment start",
			statement: "GRANT SELECT ON mydb /* comment */ TO myuser",
			wantError: true,
			errType:   ErrCommentDetected,
		},
		{
			name:      "statement with semicolon - not allowed",
			statement: "GRANT SELECT ON mydb TO myuser;",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with double dash comment",
			statement: "GRANT SELECT ON mydb TO myuser --",
			wantError: true,
			errType:   ErrCommentDetected,
		},
		{
			name:      "statement too long",
			statement: strings.Repeat("a", MaxStatementLength+1),
			wantError: true,
			errType:   ErrStatementTooLong,
		},
		{
			name:      "statement with SELECT as standalone",
			statement: "SELECT * FROM users",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with INSERT keyword",
			statement: "INSERT INTO users VALUES(1)",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with UPDATE keyword",
			statement: "UPDATE users SET id=1",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with DELETE keyword",
			statement: "DELETE FROM users",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with DROP keyword",
			statement: "DROP USER myuser",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with CREATE keyword",
			statement: "CREATE TABLE users",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with UNION keyword",
			statement: "UNION SELECT * FROM users",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with xp_ pattern",
			statement: "EXEC xp_cmdshell 'dir'",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with sp_ pattern",
			statement: "EXEC sp_stored_proc",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with waitfor pattern",
			statement: "WAITFOR DELAY '00:00:05'",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "valid GRANT EXECUTE statement",
			statement: "GRANT EXECUTE ON mydb TO myuser",
			wantError: false,
		},
		{
			name:      "statement with INTO keyword standalone",
			statement: "INSERT INTO users VALUES(1)",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with FROM keyword standalone",
			statement: "SELECT * FROM users",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with WHERE keyword standalone",
			statement: "SELECT * FROM users WHERE id=1",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "statement with JOIN keyword standalone",
			statement: "SELECT * FROM a JOIN b ON a.id=b.id",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "valid GRANT ALL statement",
			statement: "GRANT ALL ON mydb TO myuser",
			wantError: false,
		},
		{
			name:      "valid GRANT with column privileges",
			statement: "GRANT SELECT (col1, col2) ON mydb TO myuser",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSQLStatement(tt.statement)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateSQLStatement() expected error for %q, got nil", tt.statement)
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("ValidateSQLStatement() error = %v, want error type %v", err, tt.errType)
				}
			} else if err != nil {
				t.Errorf("ValidateSQLStatement() unexpected error for %q: %v", tt.statement, err)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		wantError bool
		errType   error
	}{
		{
			name:      "empty username",
			username:  "",
			wantError: true,
		},
		{
			name:      "valid username",
			username:  "validuser",
			wantError: false,
		},
		{
			name:      "valid username with underscore",
			username:  "valid_user",
			wantError: false,
		},
		{
			name:      "valid username with numbers",
			username:  "user123",
			wantError: false,
		},
		{
			name:      "username too long",
			username:  strings.Repeat("a", 31),
			wantError: true,
		},
		{
			name:      "username at max length",
			username:  strings.Repeat("a", 30),
			wantError: false,
		},
		{
			name:      "username with dollar sign",
			username:  "user$123",
			wantError: false,
		},
		{
			name:      "username with space",
			username:  "user name",
			wantError: true,
		},
		{
			name:      "username with semicolon",
			username:  "user;name",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "username with SQL injection",
			username:  "user' OR '1'='1",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "username with SELECT keyword",
			username:  "userSELECT",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
		{
			name:      "username with DROP keyword",
			username:  "userDROP",
			wantError: true,
			errType:   ErrDangerousPattern,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateUsername() expected error for %q, got nil", tt.username)
					return
				}
			} else if err != nil {
				t.Errorf("ValidateUsername() unexpected error for %q: %v", tt.username, err)
			}
		})
	}
}

func TestValidateStatementTemplates(t *testing.T) {
	tests := []struct {
		name        string
		creation    string
		revocation  string
		rollback    string
		renewal     string
		wantError   bool
		errContains string
	}{
		{
			name:       "all empty - valid",
			creation:   "",
			revocation: "",
			rollback:   "",
			renewal:    "",
			wantError:  false,
		},
		{
			name:       "valid GRANT SELECT in creation",
			creation:   "GRANT SELECT ON mydb TO {{username}}",
			revocation: "",
			rollback:   "",
			renewal:    "",
			wantError:  false,
		},
		{
			name:       "valid REVOKE in revocation",
			creation:   "",
			revocation: "REVOKE SELECT ON mydb FROM {{username}}",
			rollback:   "",
			renewal:    "",
			wantError:  false,
		},
		{
			name:        "invalid creation statement with SELECT standalone",
			creation:    "SELECT * FROM users",
			revocation:  "",
			rollback:    "",
			renewal:     "",
			wantError:   true,
			errContains: "creation_statement",
		},
		{
			name:        "invalid creation statement with semicolon",
			creation:    "GRANT SELECT ON mydb TO {{username}};",
			revocation:  "",
			rollback:    "",
			renewal:     "",
			wantError:   true,
			errContains: "creation_statement",
		},
		{
			name:        "invalid rollback statement with DROP",
			creation:    "",
			revocation:  "",
			rollback:    "DROP USER {{username}}",
			renewal:     "",
			wantError:   true,
			errContains: "rollback_statement",
		},
		{
			name:        "invalid renewal statement with DELETE",
			creation:    "",
			revocation:  "",
			rollback:    "",
			renewal:     "DELETE FROM users WHERE name='{{username}}'",
			wantError:   true,
			errContains: "renewal_statement",
		},
		{
			name:        "invalid revocation statement with INSERT",
			creation:    "",
			revocation:  "INSERT INTO users VALUES(1)",
			rollback:    "",
			renewal:     "",
			wantError:   true,
			errContains: "revocation_statement",
		},
		{
			name:       "valid multiple statements",
			creation:   "GRANT SELECT ON mydb TO {{username}}",
			revocation: "REVOKE SELECT ON mydb FROM {{username}}",
			rollback:   "",
			renewal:    "",
			wantError:  false,
		},
		{
			name:       "valid GRANT ALL in creation",
			creation:   "GRANT ALL ON mydb TO {{username}}",
			revocation: "",
			rollback:   "",
			renewal:    "",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatementTemplates(tt.creation, tt.revocation, tt.rollback, tt.renewal)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateStatementTemplates() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateStatementTemplates() error = %v, want error containing %q", err, tt.errContains)
				}
			} else if err != nil {
				t.Errorf("ValidateStatementTemplates() unexpected error: %v", err)
			}
		})
	}
}

func TestSanitizeStringForSQL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "no special chars",
			input: "abcdef",
			want:  "abcdef",
		},
		{
			name:  "single quote escaped",
			input: "pass'word",
			want:  "pass''word",
		},
		{
			name:  "double quote escaped",
			input: `pass"word`,
			want:  `pass""word`,
		},
		{
			name:  "backslash escaped",
			input: `pass\word`,
			want:  `pass\\word`,
		},
		{
			name:  "mixed special chars",
			input: `a'b"c\d`,
			want:  `a''b""c\\d`,
		},
		{
			name:  "semicolon preserved",
			input: "a;b;c",
			want:  "a;b;c",
		},
		{
			name:  "dash preserved",
			input: "a-b-c",
			want:  "a-b-c",
		},
		{
			name:  "asterisk preserved",
			input: "a*b*c",
			want:  "a*b*c",
		},
		{
			name:  "forward slash preserved",
			input: "a/b/c",
			want:  "a/b/c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeStringForSQL(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeStringForSQL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
