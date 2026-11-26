package clients

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTerraformConfiguration(t *testing.T) {
	required := map[string]any{
		"wiz_url":                "https://api.example.app.wiz.io/graphql",
		"wiz_auth_client_id":     "id",
		"wiz_auth_client_secret": "secret",
	}
	cases := map[string]struct {
		creds   map[string]any
		want    map[string]any
		wantErr bool
	}{
		"RequiredOnly": {
			creds: required,
			want:  required,
		},
		"OptionalPassedThroughAndUnknownDropped": {
			creds: merge(required, map[string]any{"wiz_auth_audience": "beyond-api", "proxy": true, "ca_chain": "", "unknown": "x"}),
			want:  merge(required, map[string]any{"wiz_auth_audience": "beyond-api", "proxy": true}),
		},
		"MissingRequired": {
			creds:   map[string]any{"wiz_url": "https://api.example.app.wiz.io/graphql", "wiz_auth_client_id": "id"},
			wantErr: true,
		},
		"EmptyRequired": {
			creds:   merge(required, map[string]any{"wiz_auth_client_secret": ""}),
			wantErr: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := terraformConfiguration(tc.creds)
			if (err != nil) != tc.wantErr {
				t.Fatalf("terraformConfiguration() error = %v, wantErr %v", err, tc.wantErr)
			}
			if diff := cmp.Diff(tc.want, got); !tc.wantErr && diff != "" {
				t.Errorf("terraformConfiguration() -want +got:\n%s", diff)
			}
		})
	}
}

func merge(maps ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}
