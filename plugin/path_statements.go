package teradata

import (
	"context"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathStatements() *framework.Path {
	return &framework.Path{
		Pattern:         "statements/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Manage SQL statements",
		HelpDescription: "Configure SQL statements for dynamic credential generation.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the statement",
			},
			"creation_statement": {
				Type:        framework.TypeString,
				Description: "SQL to create user",
			},
			"revocation_statement": {
				Type:        framework.TypeString,
				Description: "SQL to revoke user",
			},
			"rollback_statement": {
				Type:        framework.TypeString,
				Description: "SQL to rollback on failure",
			},
			"renewal_statement": {
				Type:        framework.TypeString,
				Description: "SQL to renew user",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathStatementWrite,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathStatementRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathStatementWrite,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathStatementDelete,
			},
		},
	}
}

func (b *Backend) pathStatementWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	statement := map[string]interface{}{
		"name":                 name,
		"creation_statement":   data.Get("creation_statement").(string),
		"revocation_statement": data.Get("revocation_statement").(string),
		"rollback_statement":   data.Get("rollback_statement").(string),
		"renewal_statement":    data.Get("renewal_statement").(string),
	}

	entry, err := logical.StorageEntryJSON("statements/"+name, statement)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: statement,
	}, nil
}

func (b *Backend) pathStatementRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	entry, err := req.Storage.Get(ctx, "statements/"+name)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"name":                 name,
			"creation_statement":   "***",
			"revocation_statement": "***",
			"rollback_statement":   "***",
			"renewal_statement":    "***",
		},
	}, nil
}

func (b *Backend) pathStatementDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	err := req.Storage.Delete(ctx, "statements/"+name)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
