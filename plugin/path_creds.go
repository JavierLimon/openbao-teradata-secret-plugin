package teradata

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	teradb "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
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

func (b *Backend) pathCredsRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	role, err := getRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, fmt.Errorf("role %q not found", name)
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	username := generateUsername(role.UsernamePrefix)
	password := generatePassword()

	createSQL := buildTeradataCreateUserSQL(role, username, password)
	_, err = executeSQL(ctx, cfg.ConnectionString, createSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if role.CreationStatement != "" {
		additionalSQL := role.CreationStatement
		additionalSQL = strings.ReplaceAll(additionalSQL, "{{username}}", username)
		additionalSQL = strings.ReplaceAll(additionalSQL, "{{password}}", password)
		_, err = executeSQL(ctx, cfg.ConnectionString, additionalSQL)
		if err != nil {
			dropSQL := fmt.Sprintf("DROP USER %s", username)
			executeSQL(ctx, cfg.ConnectionString, dropSQL)
			return nil, fmt.Errorf("failed to run creation statement: %w", err)
		}
	}

	ttl := time.Duration(role.DefaultTTL) * time.Second
	maxTTL := time.Duration(role.MaxTTL) * time.Second

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

func generatePassword() string {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
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

	// TODO: Execute SQL using cgo ODBC
	// For now, just test connection
	return nil, nil
}
