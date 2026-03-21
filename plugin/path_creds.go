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
	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/security"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/webhook"
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
			"region": {
				Type:        framework.TypeString,
				Description: "Region to generate credentials for (uses default config if not specified)",
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
			"region": {
				Type:        framework.TypeString,
				Description: "Region to generate credentials for (uses default config if not specified)",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCredsBatchRead,
			},
		},
	}
}

func (b *Backend) getCachedCredential(ctx context.Context, storage logical.Storage, username string) (*models.Credential, error) {
	cache := b.getCredCache()
	if cache != nil {
		if cred, found := cache.Get(username); found {
			return cred, nil
		}
	}

	cred, err := getCredential(ctx, storage, username)
	if err != nil {
		return nil, err
	}

	if cred != nil && cache != nil {
		cache.Set(username, cred)
	}

	return cred, nil
}

func (b *Backend) cacheCredential(username string, cred *models.Credential) {
	cache := b.getCredCache()
	if cache != nil && cred != nil {
		cache.Set(username, cred)
	}
}

func (b *Backend) invalidateCachedCredential(username string) {
	cache := b.getCredCache()
	if cache != nil {
		cache.Delete(username)
	}
}

func (b *Backend) pathCredsRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)
	region := data.Get("region").(string)

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

	var cfg *models.Config
	if region != "" {
		cfg, err = getConfigByRegion(ctx, req.Storage, region)
		if err != nil {
			return nil, fmt.Errorf("failed to get region config: %w", err)
		}
		if cfg == nil {
			return nil, fmt.Errorf("configuration for region %q not found", region)
		}
	} else {
		cfg, err = getConfig(ctx, req.Storage)
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			return nil, fmt.Errorf("database configuration not found")
		}
	}

	if b.IsDegraded() || !b.IsPoolHealthy(region) {
		canOperate, reason := b.CanOperate(region)
		if !canOperate {
			degradedSince := b.DegradedSince()
			degradationInfo := map[string]interface{}{
				"error":                     "database unavailable - graceful degradation mode active",
				"reason":                    reason,
				"degraded":                  true,
				"graceful_degradation_mode": true,
			}
			if !degradedSince.IsZero() {
				degradationInfo["degraded_since"] = degradedSince
			}
			return &logical.Response{
				Data:     degradationInfo,
				Warnings: []string{"Database is currently unavailable. Credential creation is not possible in graceful degradation mode."},
			}, fmt.Errorf("cannot create credentials: %s", reason)
		}
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

	if err := security.ValidatePassword(password); err != nil {
		return nil, fmt.Errorf("generated password validation failed: %w", err)
	}

	createSQL := buildTeradataCreateUserSQL(role, username, password)
	_, err = executeSQL(ctx, cfg, createSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if creationStatement != "" {
		additionalSQL := creationStatement
		additionalSQL = strings.ReplaceAll(additionalSQL, "{{username}}", username)
		additionalSQL = strings.ReplaceAll(additionalSQL, "{{password}}", password)
		_, err = executeSQL(ctx, cfg, additionalSQL)
		if err != nil {
			rollbackSQL := rollbackStatement
			if rollbackSQL != "" {
				rollbackSQL = strings.ReplaceAll(rollbackSQL, "{{username}}", username)
				rollbackSQL = strings.ReplaceAll(rollbackSQL, "{{password}}", password)
				executeSQL(ctx, cfg, rollbackSQL)
			}
			dropSQL := fmt.Sprintf("DROP USER %s", username)
			executeSQL(ctx, cfg, dropSQL)
			return nil, fmt.Errorf("failed to run creation statement: %w", err)
		}
	}

	ttl := time.Duration(role.DefaultTTL) * time.Second
	maxTTL := time.Duration(role.MaxTTL) * time.Second

	expiresAt := time.Now().Add(ttl)
	leaseID := fmt.Sprintf("teradata/creds/%s/%s", name, username)

	cred := &models.Credential{
		LeaseID:   leaseID,
		Username:  username,
		RoleName:  name,
		Region:    region,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if err := storeCredential(ctx, req.Storage, username, cred); err != nil {
		return nil, fmt.Errorf("failed to store credential: %w", err)
	}
	b.cacheCredential(username, cred)

	sessionVariables := mergeSessionVariables(cfg.SessionVariables, role.SessionVariables)

	resp := &logical.Response{
		Data: map[string]interface{}{
			"username":          username,
			"password":          password,
			"ttl":               int(ttl.Seconds()),
			"max_ttl":           int(maxTTL.Seconds()),
			"lease_id":          leaseID,
			"session_variables": sessionVariables,
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

	_ = audit.LogCredentialCreation(ctx, req.Storage, username, name, leaseID, nil)
	_ = webhook.SendCredentialCreatedWebhook(ctx, req.Storage, username, name, leaseID, nil)

	return resp, nil
}

func (b *Backend) pathCredsBatchRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)
	count := data.Get("count").(int)
	region := data.Get("region").(string)

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

	var cfg *models.Config
	if region != "" {
		cfg, err = getConfigByRegion(ctx, req.Storage, region)
		if err != nil {
			return nil, fmt.Errorf("failed to get region config: %w", err)
		}
		if cfg == nil {
			return nil, fmt.Errorf("configuration for region %q not found", region)
		}
	} else {
		cfg, err = getConfig(ctx, req.Storage)
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			return nil, fmt.Errorf("database configuration not found")
		}
	}

	if b.IsDegraded() || !b.IsPoolHealthy(region) {
		canOperate, reason := b.CanOperate(region)
		if !canOperate {
			degradedSince := b.DegradedSince()
			degradationInfo := map[string]interface{}{
				"error":                     "database unavailable - graceful degradation mode active",
				"reason":                    reason,
				"degraded":                  true,
				"graceful_degradation_mode": true,
			}
			if !degradedSince.IsZero() {
				degradationInfo["degraded_since"] = degradedSince
			}
			return &logical.Response{
				Data:     degradationInfo,
				Warnings: []string{"Database is currently unavailable. Batch credential creation is not possible in graceful degradation mode."},
			}, fmt.Errorf("cannot create credentials: %s", reason)
		}
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

		if err := security.ValidatePassword(password); err != nil {
			return nil, fmt.Errorf("generated password validation failed: %w", err)
		}

		createSQL := buildTeradataCreateUserSQL(role, username, password)
		_, err = executeSQL(ctx, cfg, createSQL)
		if err != nil {
			return nil, fmt.Errorf("failed to create user %s: %w", username, err)
		}

		if creationStatement != "" {
			additionalSQL := creationStatement
			additionalSQL = strings.ReplaceAll(additionalSQL, "{{username}}", username)
			additionalSQL = strings.ReplaceAll(additionalSQL, "{{password}}", password)
			_, err = executeSQL(ctx, cfg, additionalSQL)
			if err != nil {
				if rollbackStatement != "" {
					rollbackSQL := strings.ReplaceAll(rollbackStatement, "{{username}}", username)
					rollbackSQL = strings.ReplaceAll(rollbackSQL, "{{password}}", password)
					executeSQL(ctx, cfg, rollbackSQL)
				}
				dropSQL := fmt.Sprintf("DROP USER %s", username)
				executeSQL(ctx, cfg, dropSQL)
				return nil, fmt.Errorf("failed to run creation statement for %s: %w", username, err)
			}
		}

		ttl := time.Duration(role.DefaultTTL) * time.Second
		maxTTL := time.Duration(role.MaxTTL) * time.Second
		expiresAt := time.Now().Add(ttl)

		leaseID := fmt.Sprintf("teradata/creds/%s/%s", name, username)

		credModel := &models.Credential{
			LeaseID:   leaseID,
			Username:  username,
			RoleName:  name,
			Region:    region,
			CreatedAt: time.Now(),
			ExpiresAt: expiresAt,
		}
		if err := storeCredential(ctx, req.Storage, username, credModel); err != nil {
			return nil, fmt.Errorf("failed to store credential for %s: %w", username, err)
		}
		b.cacheCredential(username, credModel)

		cred := map[string]interface{}{
			"username": username,
			"password": password,
			"ttl":      int(ttl.Seconds()),
			"max_ttl":  int(maxTTL.Seconds()),
			"lease_id": leaseID,
		}
		credentials = append(credentials, cred)
	}

	ttl := time.Duration(role.DefaultTTL) * time.Second
	maxTTL := time.Duration(role.MaxTTL) * time.Second

	sessionVariables := mergeSessionVariables(cfg.SessionVariables, role.SessionVariables)

	resp := &logical.Response{
		Data: map[string]interface{}{
			"credentials":       credentials,
			"count":             len(credentials),
			"ttl":               int(ttl.Seconds()),
			"max_ttl":           int(maxTTL.Seconds()),
			"session_variables": sessionVariables,
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
			_ = webhook.SendCredentialCreatedWebhook(ctx, req.Storage, username, name, leaseID, map[string]interface{}{"batch": true})
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

func executeSQL(ctx context.Context, cfg *models.Config, sql string) (interface{}, error) {
	var result interface{}
	var err error

	connString, err := buildConnectionString(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build connection string: %w", err)
	}

	retryCfg := buildRetryConfig(cfg)

	err = retry.Do(ctx, retryCfg, func() error {
		conn, connErr := teradb.Connect(connString)
		if connErr != nil {
			return connErr
		}
		defer conn.Close()

		if cfg.MaxResultRows > 0 {
			conn.SetMaxResultRows(cfg.MaxResultRows)
		}

		execErr := conn.ExecuteMultipleStatements(ctx, sql)
		if execErr != nil {
			return execErr
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("executeSQL failed after retries: %w", err)
	}

	return result, nil
}

func buildRetryConfig(cfg *models.Config) *retry.Config {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = retry.DefaultMaxAttempts
	}

	initialInterval := time.Duration(cfg.InitialRetryInterval) * time.Millisecond
	if initialInterval <= 0 {
		initialInterval = retry.DefaultInitialInterval
	}

	maxInterval := time.Duration(cfg.MaxRetryInterval) * time.Millisecond
	if maxInterval <= 0 {
		maxInterval = retry.DefaultMaxInterval
	}

	multiplier := cfg.RetryMultiplier
	if multiplier <= 0 {
		multiplier = retry.DefaultMultiplier
	}

	return &retry.Config{
		MaxAttempts:     maxRetries,
		InitialInterval: initialInterval,
		MaxInterval:     maxInterval,
		Multiplier:      multiplier,
	}
}

func buildConnectionString(cfg *models.Config) (string, error) {
	if cfg.ConnectionString != "" {
		return teradb.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout), nil
	}

	if cfg.ConnectionStringTemplate != "" {
		params := map[string]string{
			"server":   cfg.Server,
			"port":     fmt.Sprintf("%d", cfg.Port),
			"database": cfg.Database,
			"username": cfg.Username,
			"password": cfg.Password,
		}

		connStr, err := teradb.BuildConnectionStringFromTemplate(cfg.ConnectionStringTemplate, params)
		if err != nil {
			return "", err
		}
		return teradb.AppendQueryTimeout(connStr, cfg.QueryTimeout), nil
	}

	return "", fmt.Errorf("no connection string or template configured")
}

func executeGrantStatements(ctx context.Context, connString, grantStatements string) error {
	if strings.TrimSpace(grantStatements) == "" {
		return nil
	}

	var err error

	err = retry.Do(ctx, nil, func() error {
		conn, connErr := teradb.Connect(connString)
		if connErr != nil {
			return connErr
		}
		defer conn.Close()

		return conn.ExecuteGrantStatements(ctx, grantStatements)
	})

	if err != nil {
		return fmt.Errorf("executeGrantStatements failed after retries: %w", err)
	}

	return nil
}

func storeCredential(ctx context.Context, storage logical.Storage, username string, cred *models.Credential) error {
	entry, err := logical.StorageEntryJSON(credentialPrefix+username, cred)
	if err != nil {
		return err
	}
	if err := storage.Put(ctx, entry); err != nil {
		return err
	}
	return nil
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

func getCredentialByLeaseID(ctx context.Context, storage logical.Storage, leaseID string) (*models.Credential, string, error) {
	entries, err := storage.List(ctx, credentialPrefix)
	if err != nil {
		return nil, "", err
	}

	for _, entry := range entries {
		username := strings.TrimPrefix(entry, credentialPrefix)
		cred, err := getCredential(ctx, storage, username)
		if err != nil {
			return nil, "", err
		}
		if cred != nil && cred.LeaseID == leaseID {
			return cred, username, nil
		}
	}

	return nil, "", nil
}

func listAllLeases(ctx context.Context, storage logical.Storage) ([]*models.Credential, error) {
	entries, err := storage.List(ctx, credentialPrefix)
	if err != nil {
		return nil, err
	}

	leases := make([]*models.Credential, 0, len(entries))
	for _, entry := range entries {
		username := strings.TrimPrefix(entry, credentialPrefix)
		cred, err := getCredential(ctx, storage, username)
		if err != nil {
			return nil, err
		}
		if cred != nil {
			leases = append(leases, cred)
		}
	}

	return leases, nil
}

func (b *Backend) cleanupExpiredCredentials(ctx context.Context, storage logical.Storage, cfg *models.Config) (int, error) {
	entries, err := storage.List(ctx, credentialPrefix)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	cleaned := 0

	for _, entry := range entries {
		username := strings.TrimPrefix(entry, credentialPrefix)
		cred, err := getCredential(ctx, storage, username)
		if err != nil {
			continue
		}
		if cred != nil && now.After(cred.ExpiresAt) {
			dropSQL := fmt.Sprintf("DROP USER %s", username)
			_, execErr := executeSQL(ctx, cfg, dropSQL)
			if execErr == nil {
				delErr := deleteCredential(ctx, storage, username)
				if delErr == nil {
					b.invalidateCachedCredential(username)
					cleaned++
				}
			}
		}
	}

	return cleaned, nil
}

func mergeSessionVariables(configVars, roleVars map[string]string) map[string]string {
	result := make(map[string]string)

	for k, v := range configVars {
		result[k] = v
	}

	for k, v := range roleVars {
		result[k] = v
	}

	return result
}

func buildSessionVariableSQL(vars map[string]string) []string {
	statements := make([]string, 0, len(vars))
	for name, value := range vars {
		stmt := fmt.Sprintf("SET %s = %s", name, value)
		statements = append(statements, stmt)
	}
	return statements
}

func executeSessionVariables(ctx context.Context, cfg *models.Config, username, password string, sessionVars map[string]string) error {
	if len(sessionVars) == 0 {
		return nil
	}

	statements := buildSessionVariableSQL(sessionVars)
	for _, stmt := range statements {
		_, err := executeSQL(ctx, cfg, stmt)
		if err != nil {
			return fmt.Errorf("failed to execute session variable %q: %w", stmt, err)
		}
	}
	return nil
}
