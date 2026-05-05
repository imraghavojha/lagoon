package hardware

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name  string
		ram   int64
		cores int
		arch  string
		want  Class
	}{
		{"small arm", 1024, 4, "arm64", PiClass},
		{"small x86", 2048, 2, "amd64", PiClass},
		{"laptop", 8192, 4, "amd64", LaptopClass},
		{"mini pc", 32768, 12, "amd64", MiniPCClass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.ram, tt.cores, tt.arch); got != tt.want {
				t.Fatalf("Classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultMemoryCap(t *testing.T) {
	if got := DefaultMemoryCap(Machine{Class: PiClass}); got != "768m" {
		t.Fatalf("Pi cap = %q", got)
	}
	if got := DefaultMemoryCap(Machine{Class: LaptopClass}); got != "2g" {
		t.Fatalf("Laptop cap = %q", got)
	}
}
