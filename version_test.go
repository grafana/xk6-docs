package docs

import (
	"runtime/debug"
	"testing"
)

func TestDetectK6Version(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		info      *debug.BuildInfo
		available bool
		want      string
		wantErr   bool
	}{
		{
			name: "k6 dependency found",
			info: &debug.BuildInfo{Deps: []*debug.Module{
				{Path: "github.com/spf13/cobra", Version: "v1.10.2"},
				{Path: "go.k6.io/k6", Version: "v1.6.0"},
				{Path: "github.com/sirupsen/logrus", Version: "v1.9.3"},
			}},
			available: true,
			want:      "v1.6.x",
		},
		{
			name:      "k6 dependency not found",
			info:      &debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/spf13/cobra", Version: "v1.10.2"}}},
			available: true,
			wantErr:   true,
		},
		{
			name:      "build info unavailable",
			available: false,
			wantErr:   true,
		},
		{
			name:      "k6 pre-release version",
			info:      &debug.BuildInfo{Deps: []*debug.Module{{Path: "go.k6.io/k6", Version: "v0.55.2-rc.1"}}},
			available: true,
			want:      "v0.55.x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := detectK6Version(func() (*debug.BuildInfo, bool) { return tt.info, tt.available })
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
