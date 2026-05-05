package cmd

import "testing"

func TestInferPorts(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"long flag", "python app.py --port 8080", []string{"8080"}},
		{"equals flag", "vite --host 0.0.0.0 --port=5173", []string{"5173"}},
		{"env", "PORT=3000 node server.js", []string{"3000"}},
		{"host colon", "uvicorn app:app --host 0.0.0.0 --port 8000", []string{"8000"}},
		{"http server", "python3 -m http.server 9000", []string{"9000"}},
		{"multiple dedupe sorted", "PORT=3000 app --port 8080 --listen :3000", []string{"3000", "8080"}},
		{"none", "pnpm dev --host 0.0.0.0", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferPorts(tt.cmd)
			if len(got) != len(tt.want) {
				t.Fatalf("inferPorts(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("inferPorts(%q) = %v, want %v", tt.cmd, got, tt.want)
				}
			}
		})
	}
}

func TestPortsLabel(t *testing.T) {
	if got := portsLabel(nil); got != "unknown" {
		t.Fatalf("empty label = %q", got)
	}
	if got := portsLabel([]string{"3000", "8080"}); got != "3000,8080" {
		t.Fatalf("ports label = %q", got)
	}
}
