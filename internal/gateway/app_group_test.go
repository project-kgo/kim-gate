package gateway

import "testing"

func TestParseAppGroup(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		wantGroup string
		wantAppID string
		wantErr   bool
	}{
		{
			name:      "valid user id",
			userID:    "app1:user1",
			wantGroup: "app:app1",
			wantAppID: "app1",
		},
		{
			name:      "trim whitespace",
			userID:    "  app2:user2  ",
			wantGroup: "app:app2",
			wantAppID: "app2",
		},
		{
			name:    "empty string",
			userID:  "",
			wantErr: true,
		},
		{
			name:    "missing separator",
			userID:  "app1-user1",
			wantErr: true,
		},
		{
			name:    "empty app id",
			userID:  ":user1",
			wantErr: true,
		},
		{
			name:    "empty user part",
			userID:  "app1:",
			wantErr: true,
		},
		{
			name:    "blank parts",
			userID:  " : ",
			wantErr: true,
		},
		{
			name:    "multiple separators",
			userID:  "app1:user1:extra",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, appID, err := ParseAppGroup(tt.userID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseAppGroup(%q) error = nil, want error", tt.userID)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAppGroup(%q) error = %v", tt.userID, err)
			}
			if group != tt.wantGroup || appID != tt.wantAppID {
				t.Fatalf("ParseAppGroup(%q) = (%q, %q), want (%q, %q)", tt.userID, group, appID, tt.wantGroup, tt.wantAppID)
			}
		})
	}
}
