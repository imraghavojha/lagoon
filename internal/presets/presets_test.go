package presets

import "testing"

func TestMergePackagesDedupesInOrder(t *testing.T) {
	got := MergePackages([]string{"python311", "uv"}, []string{"uv", "git"})
	want := []string{"python311", "uv", "git"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestFindClones(t *testing.T) {
	p, ok := Find("python")
	if !ok {
		t.Fatal("python preset missing")
	}
	p.Packages[0] = "mutated"
	again, _ := Find("python")
	if again.Packages[0] == "mutated" {
		t.Fatal("Find returned shared preset slice")
	}
}
