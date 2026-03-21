package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openbao/openbao/sdk/v2/logical"
)

type AuditEventType string

const (
	EventTypeCredentialCreated AuditEventType = "credential_created"
	EventTypeCredentialRevoked AuditEventType = "credential_revoked"
	EventTypeCredentialRotated AuditEventType = "credential_rotated"
	EventTypeRoleCreated       AuditEventType = "role_created"
	EventTypeRoleUpdated       AuditEventType = "role_updated"
	EventTypeRoleDeleted       AuditEventType = "role_deleted"
	EventTypeRootRotated       AuditEventType = "root_rotated"
)

const auditPrefix = "audit/"

type CredentialAuditData struct {
	Username  string                 `json:"username"`
	RoleName  string                 `json:"role_name"`
	Operation string                 `json:"operation"`
	Timestamp time.Time              `json:"timestamp"`
	LeaseID   string                 `json:"lease_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type RoleAuditData struct {
	RoleName  string    `json:"role_name"`
	Operation string    `json:"operation"`
	Timestamp time.Time `json:"timestamp"`
	DBUser    string    `json:"db_user,omitempty"`
	Statement string    `json:"statement_template,omitempty"`
}

type RotationAuditData struct {
	Operation string    `json:"operation"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

func LogCredentialCreation(ctx context.Context, storage logical.Storage, username, roleName, leaseID string, metadata map[string]interface{}) error {
	auditData := CredentialAuditData{
		Username:  username,
		RoleName:  roleName,
		Operation: string(EventTypeCredentialCreated),
		Timestamp: time.Now().UTC(),
		LeaseID:   leaseID,
		Metadata:  metadata,
	}

	return storeAuditEvent(ctx, storage, EventTypeCredentialCreated, auditData)
}

func LogCredentialRevocation(ctx context.Context, storage logical.Storage, username, roleName string, metadata map[string]interface{}) error {
	auditData := CredentialAuditData{
		Username:  username,
		RoleName:  roleName,
		Operation: string(EventTypeCredentialRevoked),
		Timestamp: time.Now().UTC(),
		Metadata:  metadata,
	}

	return storeAuditEvent(ctx, storage, EventTypeCredentialRevoked, auditData)
}

func LogCredentialRotation(ctx context.Context, storage logical.Storage, success bool, errMsg string, metadata map[string]interface{}) error {
	auditData := RotationAuditData{
		Operation: string(EventTypeCredentialRotated),
		Timestamp: time.Now().UTC(),
		Success:   success,
		Error:     errMsg,
	}

	return storeAuditEvent(ctx, storage, EventTypeCredentialRotated, auditData)
}

func LogRoleCreation(ctx context.Context, storage logical.Storage, roleName, dbUser, statement string) error {
	auditData := RoleAuditData{
		RoleName:  roleName,
		Operation: string(EventTypeRoleCreated),
		Timestamp: time.Now().UTC(),
		DBUser:    dbUser,
		Statement: statement,
	}

	return storeAuditEvent(ctx, storage, EventTypeRoleCreated, auditData)
}

func LogRoleUpdate(ctx context.Context, storage logical.Storage, roleName, dbUser, statement string) error {
	auditData := RoleAuditData{
		RoleName:  roleName,
		Operation: string(EventTypeRoleUpdated),
		Timestamp: time.Now().UTC(),
		DBUser:    dbUser,
		Statement: statement,
	}

	return storeAuditEvent(ctx, storage, EventTypeRoleUpdated, auditData)
}

func LogRoleDeletion(ctx context.Context, storage logical.Storage, roleName string) error {
	auditData := RoleAuditData{
		RoleName:  roleName,
		Operation: string(EventTypeRoleDeleted),
		Timestamp: time.Now().UTC(),
	}

	return storeAuditEvent(ctx, storage, EventTypeRoleDeleted, auditData)
}

func LogRootRotation(ctx context.Context, storage logical.Storage, success bool, errMsg string) error {
	auditData := RotationAuditData{
		Operation: string(EventTypeRootRotated),
		Timestamp: time.Now().UTC(),
		Success:   success,
		Error:     errMsg,
	}

	return storeAuditEvent(ctx, storage, EventTypeRootRotated, auditData)
}

func storeAuditEvent(ctx context.Context, storage logical.Storage, eventType AuditEventType, data interface{}) error {
	if storage == nil {
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal audit data: %w", err)
	}

	entry := logical.StorageEntry{
		Key:   auditPrefix + string(eventType) + "/" + time.Now().Format("20060102150405") + "/" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Value: jsonData,
	}

	if err := storage.Put(ctx, &entry); err != nil {
		return fmt.Errorf("failed to store audit event: %w", err)
	}

	return nil
}
