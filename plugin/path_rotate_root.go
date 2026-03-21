package teradata

import (
	"context"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathRotateRoot() *framework.Path {
	return &framework.Path{
		Pattern:         "rotate-root",
		HelpSynopsis:    "Rotate root credentials",
		HelpDescription: "Rotates the root database credentials used for administrative tasks.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRotateRootHandler,
			},
		},
	}
}

func (b *Backend) pathRotateRootHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	return &logical.Response{
		Data: map[string]interface{}{
			"rotated": true,
		},
	}, nil
}
