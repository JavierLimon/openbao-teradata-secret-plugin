package teradata

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"strings"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/audit"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	teradb "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

const (
	credentialPrefix = "creds/"
)

func (b *Backend) pathCreds() *framework.Path {
	return &framework.Path{
		Pattern:         "creds/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Generate database credentials",
		HelpDescription: "Generates dynamic database credentials for the specified role.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the role",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCredsRead,
			},
		},
	}
}

func (b *Backend) pathCredsBatch() *framework.Path {
	return &framework.Path{
		Pattern:         "creds/batch/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Generate multiple database credentials",
		HelpDescription: "Generates multiple dynamic database credentials for the specified role.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the role",
			},
			"count": {
				Type:        framework.TypeInt,
				Description: "Number of credentials to generate (default: 1, max: 100)",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCredsBatchRead,
			},
		},
	}
}

func (b *Backend) pathCredsRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	role, err := getRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, fmt.Errorf("role %q not found", name)
	}

	if role.MaxCredentials > 0 {
		currentCount, err := countCredentialsByRole(ctx, req.Storage, name)
		if err != nil {
			return nil, fmt.Errorf("failed to count credentials: %w", err)
		}
		if currentCount >= role.MaxCredentials {
			return nil, fmt.Errorf("credential quota exceeded for role %q: max %d, current %d", name, role.MaxCredentials, currentCount)
		}
	}

	if role.UsernamePrefix != "" {
		if err := teradb.ValidateUsername(role.UsernamePrefix); err != nil {
			return nil, fmt.Errorf("invalid username_prefix: %w", err)
		}
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	creationStatement := role.CreationStatement
	rollbackStatement := role.RollbackStatement

	if role.StatementTemplate != "" {
		statement, err := getStatement(ctx, req.Storage, role.StatementTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to load statement template: %w", err)
		}
		if statement == nil {
			return nil, fmt.Errorf("statement template %q not found", role.StatementTemplate)
		}
		if statement.CreationStatement != "" {
			creationStatement = statement.CreationStatement
		}
		if statement.RollbackStatement != "" {
			rollbackStatement = statement.RollbackStatement
		}
	}

	username := generateUsername(role.UsernamePrefix)
	password := generatePassword()

	if err := teradb.ValidateUsername(username); err != nil {
		return nil, fmt.Errorf("generated username validation failed: %w", err)
	}

	createSQL := buildTeradataCreateUserSQL(role, username, password)
	_, err = executeSQL(ctx, cfg.ConnectionString, createSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if creationStatement != "" {
		additionalSQL := creationStatement
		additionalSQL = strings.ReplaceAll(additionalSQL, "{{username}}", username)
		additionalSQL = strings.ReplaceAll(additionalSQL, "{{password}}", password)
		_, err = executeSQL(ctx, cfg.ConnectionString, additionalSQL)
		if err != nil {
			rollbackSQL := rollbackStatement
			if rollbackSQL != "" {
				rollbackSQL = strings.ReplaceAll(rollbackSQL, "{{username}}", username)
				rollbackSQL = strings.ReplaceAll(rollbackSQL, "{{password}}", password)
				executeSQL(ctx, cfg.ConnectionString, rollbackSQL)
			}
			dropSQL := fmt.Sprintf("DROP USER %s", username)
			executeSQL(ctx, cfg.ConnectionString, dropSQL)
			return nil, fmt.Errorf("failed to run creation statement: %w", err)
		}
	}

	ttl := time.Duration(role.DefaultTTL) * time.Second
	maxTTL := time.Duration(role.MaxTTL) * time.Second

	expiresAt := time.Now().Add(ttl)

	cred := &models.Credential{
		Username:  username,
		RoleName:  name,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if err := storeCredential(ctx, req.Storage, username, cred); err != nil {
		return nil, fmt.Errorf("failed to store credential: %w", err)
	}

	resp := &logical.Response{
		Data: map[string]interface{}{
			"username": username,
			"password": password,
			"ttl":      int(ttl.Seconds()),
			"max_ttl":  int(maxTTL.Seconds()),
		},
	}

	if role.DefaultTTL > 0 {
		resp.Secret = &logical.Secret{
			LeaseOptions: logical.LeaseOptions{
				TTL:    ttl,
				MaxTTL: maxTTL,
			},
		}
	}

	leaseID := fmt.Sprintf("teradata/creds/%s/%s", name, username)
	_ = audit.LogCredentialCreation(ctx, req.Storage, username, name, leaseID, nil)

	return resp, nil
}

func (b *Backend) pathCredsBatchRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)
	count := data.Get("count").(int)

	if count <= 0 {
		count = 1
	}
	if count > 100 {
		count = 100
	}

	role, err := getRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, fmt.Errorf("role %q not found", name)
	}

	if role.MaxCredentials > 0 {
		currentCount, err := countCredentialsByRole(ctx, req.Storage, name)
		if err != nil {
			return nil, fmt.Errorf("failed to count credentials: %w", err)
		}
		remainingQuota := role.MaxCredentials - currentCount
		if remainingQuota <= 0 {
			return nil, fmt.Errorf("credential quota exceeded for role %q: max %d, current %d", name, role.MaxCredentials, currentCount)
		}
		if count > remainingQuota {
			count = remainingQuota
		}
	}

	if role.UsernamePrefix != "" {
		if err := teradb.ValidateUsername(role.UsernamePrefix); err != nil {
			return nil, fmt.Errorf("invalid username_prefix: %w", err)
		}
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	creationStatement := role.CreationStatement
	rollbackStatement := role.RollbackStatement

	if role.StatementTemplate != "" {
		statement, err := getStatement(ctx, req.Storage, role.StatementTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to load statement template: %w", err)
		}
		if statement == nil {
			return nil, fmt.Errorf("statement template %q not found", role.StatementTemplate)
		}
		if statement.CreationStatement != "" {
			creationStatement = statement.CreationStatement
		}
		if statement.RollbackStatement != "" {
			rollbackStatement = statement.RollbackStatement
		}
	}

	credentials := make([]map[string]interface{}, 0, count)

	for i := 0; i < count; i++ {
		username := generateUsername(role.UsernamePrefix)
		password := generatePassword()

		if err := teradb.ValidateUsername(username); err != nil {
			return nil, fmt.Errorf("generated username validation failed: %w", err)
		}

		createSQL := buildTeradataCreateUserSQL(role, username, password)
		_, err = executeSQL(ctx, cfg.ConnectionString, createSQL)
		if err != nil {
			return nil, fmt.Errorf("failed to create user %s: %w", username, err)
		}

		if creationStatement != "" {
			additionalSQL := creationStatement
			additionalSQL = strings.ReplaceAll(additionalSQL, "{{username}}", username)
			additionalSQL = strings.ReplaceAll(additionalSQL, "{{password}}", password)
			_, err = executeSQL(ctx, cfg.ConnectionString, additionalSQL)
			if err != nil {
				if rollbackStatement != "" {
					rollbackSQL := strings.ReplaceAll(rollbackStatement, "{{username}}", username)
					rollbackSQL = strings.ReplaceAll(rollbackSQL, "{{password}}", password)
					executeSQL(ctx, cfg.ConnectionString, rollbackSQL)
				}
				dropSQL := fmt.Sprintf("DROP USER %s", username)
				executeSQL(ctx, cfg.ConnectionString, dropSQL)
				return nil, fmt.Errorf("failed to run creation statement for %s: %w", username, err)
			}
		}

		ttl := time.Duration(role.DefaultTTL) * time.Second
		maxTTL := time.Duration(role.MaxTTL) * time.Second
		expiresAt := time.Now().Add(ttl)

		credModel := &models.Credential{
			Username:  username,
			RoleName:  name,
			CreatedAt: time.Now(),
			ExpiresAt: expiresAt,
		}
		if err := storeCredential(ctx, req.Storage, username, credModel); err != nil {
			return nil, fmt.Errorf("failed to store credential for %s: %w", username, err)
		}

		cred := map[string]interface{}{
			"username": username,
			"password": password,
			"ttl":      int(ttl.Seconds()),
			"max_ttl":  int(maxTTL.Seconds()),
		}
		credentials = append(credentials, cred)
	}

	ttl := time.Duration(role.DefaultTTL) * time.Second
	maxTTL := time.Duration(role.MaxTTL) * time.Second

	resp := &logical.Response{
		Data: map[string]interface{}{
			"credentials": credentials,
			"count":       len(credentials),
			"ttl":         int(ttl.Seconds()),
			"max_ttl":     int(maxTTL.Seconds()),
		},
	}

	if role.DefaultTTL > 0 {
		resp.Secret = &logical.Secret{
			LeaseOptions: logical.LeaseOptions{
				TTL:    ttl,
				MaxTTL: maxTTL,
			},
		}
	}

	for _, cred := range credentials {
		if username, ok := cred["username"].(string); ok {
			leaseID := fmt.Sprintf("teradata/creds/%s/%s", name, username)
			_ = audit.LogCredentialCreation(ctx, req.Storage, username, name, leaseID, map[string]interface{}{"batch": true})
		}
	}

	return resp, nil
}

func buildTeradataCreateUserSQL(role *models.Role, username, password string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE USER %s FROM DBC AS\n", username))
	// Teradata doesn't use quotes around password
	sb.WriteString(fmt.Sprintf("PASSWORD = %s\n", password))

	// Default database
	if role.DefaultDatabase != "" {
		sb.WriteString(fmt.Sprintf("DEFAULT DATABASE = %s\n", role.DefaultDatabase))
	} else {
		sb.WriteString(fmt.Sprintf("DEFAULT DATABASE = %s\n", username))
	}

	// PERM is required in Teradata Cloud - default to 1MB
	if role.PermSpace > 0 {
		sb.WriteString(fmt.Sprintf("PERM = %d\n", role.PermSpace))
	} else {
		sb.WriteString("PERM = 1000000\n") // 1MB default
	}

	if role.SpoolSpace > 0 {
		sb.WriteString(fmt.Sprintf("SPOOL = %d\n", role.SpoolSpace))
	}

	if role.Account != "" {
		sb.WriteString(fmt.Sprintf("ACCOUNT = '%s'\n", role.Account))
	}

	if role.Fallback {
		sb.WriteString("FALLBACK\n")
	}

	return sb.String()
}

const (
	passwordMinLength = 16
	passwordMaxLength = 32
	lowerChars        = "abcdefghijklmnopqrstuvwxyz"
	upperChars        = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars        = "0123456789"
	specialChars      = "!@#$%^&*()_+-=[]{}|;:,.<>?"
	allPasswordChars  = lowerChars + upperChars + digitChars + specialChars
)

func generatePassword() string {
	charset := []rune(allPasswordChars)
	length := passwordMinLength + mrand.Intn(passwordMaxLength-passwordMinLength+1)
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		return ""
	}

	for i := range bytes {
		bytes[i] = byte(charset[mrand.Intn(len(charset))])
	}

	password := string(bytes)
	password = ensurePasswordRequirements(password)
	return password
}

func ensurePasswordRequirements(password string) string {
	runes := []rune(password)
	if len(runes) == 0 {
		return password
	}
	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSpecial := false

	for _, r := range runes {
		switch {
		case strings.ContainsRune(lowerChars, r):
			hasLower = true
		case strings.ContainsRune(upperChars, r):
			hasUpper = true
		case strings.ContainsRune(digitChars, r):
			hasDigit = true
		case strings.ContainsRune(specialChars, r):
			hasSpecial = true
		}
	}

	required := []struct {
		check bool
		chars string
	}{
		{hasLower, lowerChars},
		{hasUpper, upperChars},
		{hasDigit, digitChars},
		{hasSpecial, specialChars},
	}

	idx := 0
	for _, req := range required {
		if !req.check {
			pos := idx % len(runes)
			runes[pos] = rune(req.chars[mrand.Intn(len(req.chars))])
			idx++
		}
	}

	return string(runes)
}

func generateUsername(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	suffix := hex.EncodeToString(bytes)
	if prefix == "" {
		prefix = "vault"
	}
	return fmt.Sprintf("%s_%s", prefix, suffix)
}

func executeSQL(ctx context.Context, connString, sql string) (interface{}, error) {
	conn, err := teradb.Connect(connString)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	err = conn.ExecuteMultipleStatements(sql)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func executeGrantStatements(ctx context.Context, connString, grantStatements string) error {
	if strings.TrimSpace(grantStatements) == "" {
		return nil
	}

	conn, err := teradb.Connect(connString)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.ExecuteGrantStatements(grantStatements)
}

func storeCredential(ctx context.Context, storage logical.Storage, username string, cred *models.Credential) error {
	entry, err := logical.StorageEntryJSON(credentialPrefix+username, cred)
	if err != nil {
		return err
	}
	return storage.Put(ctx, entry)
}

func getCredential(ctx context.Context, storage logical.Storage, username string) (*models.Credential, error) {
	entry, err := storage.Get(ctx, credentialPrefix+username)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	var cred models.Credential
	if err := entry.DecodeJSON(&cred); err != nil {
		return nil, err
	}

	return &cred, nil
}

func deleteCredential(ctx context.Context, storage logical.Storage, username string) error {
	return storage.Delete(ctx, credentialPrefix+username)
}

func countCredentialsByRole(ctx context.Context, storage logical.Storage, roleName string) (int, error) {
	entries, err := storage.List(ctx, credentialPrefix)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		cred, err := getCredential(ctx, storage, strings.TrimPrefix(entry, credentialPrefix))
		if err != nil {
			return 0, err
		}
		if cred != nil && cred.RoleName == roleName {
			count++
		}
	}

	return count, nil
}
