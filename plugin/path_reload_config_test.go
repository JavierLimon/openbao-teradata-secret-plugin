package teradata

import (
	"context"
	"testing"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func TestPathReloadConfigHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		region        string
		setupStorage  func(logical.Storage) error
		wantSuccess   bool
		wantErr       bool
		errContains   string
		checkResponse func(*testing.T, *logical.Response)
	}{
		{
			name:   "reload non-existent default config",
			region: "",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			wantSuccess: false,
			wantErr:     false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp == nil {
					t.Fatal("response is nil")
				}
				success, ok := resp.Data["success"].(bool)
				if !ok {
					t.Fatal("success field not a boolean")
				}
				if success {
					t.Error("expected success=false for non-existent config")
				}
				if resp.Data["message"] == nil {
					t.Error("expected message field")
				}
			},
		},
		{
			name:   "reload non-existent regional config",
			region: "eu-central",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			wantSuccess: false,
			wantErr:     false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp == nil {
					t.Fatal("response is nil")
				}
				success, ok := resp.Data["success"].(bool)
				if !ok {
					t.Fatal("success field not a boolean")
				}
				if success {
					t.Error("expected success=false for non-existent regional config")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBackend()
			ctx := context.Background()

			storage := &logical.InmemStorage{}

			if err := tt.setupStorage(storage); err != nil {
				t.Fatalf("setup storage error: %v", err)
			}

			rawData := map[string]interface{}{}
			if tt.region != "" {
				rawData["region"] = tt.region
			}

			data := &framework.FieldData{
				Raw:    rawData,
				Schema: getReloadConfigFieldSchema(),
			}

			req := &logical.Request{
				Storage: storage,
			}

			resp, err := b.pathReloadConfigHandler(ctx, req, data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestPathReloadConfigFieldSchema(t *testing.T) {
	t.Parallel()

	schema := getReloadConfigFieldSchema()

	if schema["region"] == nil {
		t.Fatal("expected region field schema")
	}

	if schema["region"].Type != framework.TypeString {
		t.Errorf("expected region type string, got %v", schema["region"].Type)
	}

	if schema["region"].Description == "" {
		t.Error("expected region description")
	}
}

func getReloadConfigFieldSchema() map[string]*framework.FieldSchema {
	return map[string]*framework.FieldSchema{
		"region": {
			Type:        framework.TypeString,
			Description: "Region identifier to reload (omit for default config)",
		},
	}
}
