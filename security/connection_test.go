package security

import (
	"strings"
	"testing"
)

func TestValidateConnectionString(t *testing.T) {
	tests := []struct {
		name      string
		connStr   string
		wantError bool
		errType   error
	}{
		{
			name:      "empty connection string",
			connStr:   "",
			wantError: true,
			errType:   ErrEmptyConnectionString,
		},
		{
			name:      "whitespace only",
			connStr:   "   ",
			wantError: true,
			errType:   ErrEmptyConnectionString,
		},
		{
			name:      "valid with DSN",
			connStr:   "DSN=myteradata;",
			wantError: false,
		},
		{
			name:      "valid with SERVER",
			connStr:   "SERVER=myserver;UID=user;PWD=pass",
			wantError: false,
		},
		{
			name:      "valid with SERVERS",
			connStr:   "SERVERS=host1,host2;UID=user;PWD=pass",
			wantError: false,
		},
		{
			name:      "missing DSN and SERVER",
			connStr:   "UID=user;PWD=pass",
			wantError: true,
			errType:   ErrMissingDSN,
		},
		{
			name:      "invalid key with space",
			connStr:   "my key=value",
			wantError: true,
			errType:   ErrMissingDSN,
		},
		{
			name:      "valid complex connection string",
			connStr:   "DSN=Teradata;DBCNAME=teradata.example.com;UID=admin;PWD=secret123;",
			wantError: false,
		},
		{
			name:      "valid Teradata connection string",
			connStr:   "DSN=Teradata_Prod;UID=admin;PWD=password;",
			wantError: false,
		},
		{
			name:      "case insensitive DSN",
			connStr:   "dsn=mydsn;",
			wantError: false,
		},
		{
			name:      "case insensitive SERVER",
			connStr:   "server=myserver;",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConnectionString(tt.connStr)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateConnectionString() expected error for %q, got nil", tt.connStr)
					return
				}
				if tt.errType != nil && !errorsAreEqual(err, tt.errType) {
					t.Errorf("ValidateConnectionString() error = %v, want error type %v", err, tt.errType)
				}
			} else if err != nil {
				t.Errorf("ValidateConnectionString() unexpected error for %q: %v", tt.connStr, err)
			}
		})
	}
}

func TestMaskConnectionString(t *testing.T) {
	tests := []struct {
		name       string
		connStr    string
		wantHas    string
		wantHasNot string
	}{
		{
			name:       "empty",
			connStr:    "",
			wantHasNot: "***",
		},
		{
			name:       "password masked",
			connStr:    "DSN=test;PWD=secret",
			wantHas:    "pwd=***",
			wantHasNot: "secret",
		},
		{
			name:       "multiple sensitive fields",
			connStr:    "DSN=test;PWD=secret;UID=user;PASSWORD=pass2",
			wantHas:    "pwd=***",
			wantHasNot: "secret",
		},
		{
			name:       "sensitive keyword in key",
			connStr:    "DSN=test;MYTOKEN=value",
			wantHas:    "mytoken=***",
			wantHasNot: "value",
		},
		{
			name:       "non-sensitive",
			connStr:    "DSN=test;SERVER=localhost",
			wantHas:    "server=localhost",
			wantHasNot: "***",
		},
		{
			name:    "empty value",
			connStr: "DSN=test;PWD=",
			wantHas: "pwd=***",
		},
		{
			name:       "valid token keyword",
			connStr:    "DSN=test;TOKEN=value",
			wantHas:    "token=***",
			wantHasNot: "value",
		},
		{
			name:       "auth keyword",
			connStr:    "DSN=test;AUTH=token",
			wantHas:    "auth=***",
			wantHasNot: "token",
		},
		{
			name:       "key keyword",
			connStr:    "DSN=test;APIKEY=secret",
			wantHas:    "apikey=***",
			wantHasNot: "secret",
		},
		{
			name:       "credential keyword",
			connStr:    "DSN=test;CREDENTIAL=value",
			wantHas:    "credential=***",
			wantHasNot: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskConnectionString(tt.connStr)
			if tt.wantHas != "" && !strings.Contains(got, tt.wantHas) {
				t.Errorf("MaskConnectionString(%q) should contain %q, got %q", tt.connStr, tt.wantHas, got)
			}
			if tt.wantHasNot != "" && strings.Contains(got, tt.wantHasNot) {
				t.Errorf("MaskConnectionString(%q) should NOT contain %q, got %q", tt.connStr, tt.wantHasNot, got)
			}
		})
	}
}

func TestGetConnectionStringInfo(t *testing.T) {
	tests := []struct {
		name       string
		connStr    string
		wantCreds  bool
		wantServer bool
		wantError  bool
	}{
		{
			name:       "empty",
			connStr:    "",
			wantCreds:  false,
			wantServer: false,
			wantError:  false,
		},
		{
			name:       "with password",
			connStr:    "DSN=test;PWD=secret",
			wantCreds:  true,
			wantServer: true,
			wantError:  false,
		},
		{
			name:       "with server",
			connStr:    "SERVER=localhost;PWD=pass",
			wantCreds:  true,
			wantServer: true,
			wantError:  false,
		},
		{
			name:       "no credentials",
			connStr:    "DSN=test",
			wantCreds:  false,
			wantServer: true,
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasCreds, hasServer, err := GetConnectionStringInfo(tt.connStr)
			if (err != nil) != tt.wantError {
				t.Errorf("GetConnectionStringInfo() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if hasCreds != tt.wantCreds {
				t.Errorf("GetConnectionStringInfo() hasCreds = %v, want %v", hasCreds, tt.wantCreds)
			}
			if hasServer != tt.wantServer {
				t.Errorf("GetConnectionStringInfo() hasServer = %v, want %v", hasServer, tt.wantServer)
			}
		})
	}
}

func errorsAreEqual(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}
	if err1 == nil || err2 == nil {
		return false
	}
	return err1.Error() == err2.Error()
}
