package logger

import "testing"

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "valid level", cfg: Config{Level: "info"}, wantErr: false},
		{name: "invalid level", cfg: Config{Level: "bad-level"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := NewLogger(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil || l == nil {
				t.Fatalf("unexpected err=%v logger=%v", err, l)
			}
		})
	}
}
