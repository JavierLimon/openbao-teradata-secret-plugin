package teradata

import (
	"context"
	"fmt"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathStatements() *framework.Path {
	return &framework.Path{
		Pattern:         "statements/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Manage SQL statement templates",
		HelpDescription: "Configure SQL statement templates for dynamic credential generation.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the statement template",
			},
			"creation_statement": {
				Type:        framework.TypeString,
				Description: "SQL statement to execute upon user creation",
			},
			"revocation_statement": {
				Type:        framework.TypeString,
				Description: "SQL statement to execute upon user revocation",
			},
			"rollback_statement": {
				Type:        framework.TypeString,
				Description: "SQL statement to execute on creation failure rollback",
			},
			"renewal_statement": {
				Type:        framework.TypeString,
				Description: "SQL statement to execute upon credential renewal",
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

		ExistenceCheck: b.pathStatementExistenceCheck,
	}
}

func (b *Backend) pathStatementList() *framework.Path {
	return &framework.Path{
		Pattern:         "statements",
		HelpSynopsis:    "List SQL statement templates",
		HelpDescription: "Lists all configured SQL statement templates.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathStatementListHandler,
			},
		},
	}
}

func (b *Backend) pathStatementExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	name := data.Get("name").(string)

	entry, err := req.Storage.Get(ctx, "statements/"+name)
	if err != nil {
		return false, err
	}

	return entry != nil, nil
}

func (b *Backend) pathStatementWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	statement := &models.Statement{
		Name:                name,
		CreationStatement:   data.Get("creation_statement").(string),
		RevocationStatement: data.Get("revocation_statement").(string),
		RollbackStatement:   data.Get("rollback_statement").(string),
		RenewalStatement:    data.Get("renewal_statement").(string),
	}

	entry, err := logical.StorageEntryJSON("statements/"+name, statement)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"name":                 statement.Name,
			"creation_statement":   "***",
			"revocation_statement": "***",
			"rollback_statement":   "***",
			"renewal_statement":    "***",
		},
	}, nil
}

func (b *Backend) pathStatementRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	statement, err := getStatement(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}

	if statement == nil {
		return nil, nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"name":                 statement.Name,
			"creation_statement":   statement.CreationStatement,
			"revocation_statement": statement.RevocationStatement,
			"rollback_statement":   statement.RollbackStatement,
			"renewal_statement":    statement.RenewalStatement,
		},
	}, nil
}

func (b *Backend) pathStatementDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	err := req.Storage.Delete(ctx, "statements/"+name)
	if err != nil {
		return nil, fmt.Errorf("error deleting statement: %w", err)
	}

	return nil, nil
}

func (b *Backend) pathStatementListHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	entries, err := req.Storage.List(ctx, "statements/")
	if err != nil {
		return nil, err
	}

	return logical.ListResponse(entries), nil
}

func getStatement(ctx context.Context, storage logical.Storage, name string) (*models.Statement, error) {
	entry, err := storage.Get(ctx, "statements/"+name)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var statement models.Statement
	if err := entry.DecodeJSON(&statement); err != nil {
		return nil, err
	}

	return &statement, nil
}
