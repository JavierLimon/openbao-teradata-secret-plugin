package teradata

import (
	"context"
	"fmt"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathStaticCreds() *framework.Path {
	return &framework.Path{
		Pattern:         "static-creds/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Get static credentials",
		HelpDescription: "Retrieves the current static credentials for the specified static role.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the static role",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathStaticCredsRead,
			},
		},
	}
}

func (b *Backend) pathStaticCredsRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	staticRole, err := getStaticRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}

	if staticRole == nil {
		return nil, fmt.Errorf("static role %q not found", name)
	}

	staticCred, err := getStaticCredential(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}

	if staticCred == nil {
		return nil, fmt.Errorf("credentials for static role %q not found", name)
	}

	resp := map[string]interface{}{
		"username":      staticCred.Username,
		"password":      staticCred.Password,
		"role_name":     staticCred.RoleName,
		"db_name":       staticCred.DBName,
		"last_rotated":  staticCred.LastRotated,
		"next_rotation": staticCred.NextRotation,
	}

	return &logical.Response{
		Data: resp,
	}, nil
}
