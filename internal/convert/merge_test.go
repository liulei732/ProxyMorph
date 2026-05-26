package convert

import "testing"

func TestMergeAppendsPinnedNodesAndRenamesConflicts(t *testing.T) {
	remote := []Node{{Name: "A", Protocol: "ss", Server: "a.example", Port: 1}}
	pinned := []Node{
		{Name: "A", Protocol: "trojan", Server: "b.example", Port: 443, Pinned: true},
		{Name: "C", Protocol: "ss", Server: "c.example", Port: 8388, Pinned: true},
	}
	got := MergeNodes(remote, pinned, MergeOptions{Mode: "after_remote"})
	names := []string{got[0].Name, got[1].Name, got[2].Name}
	want := []string{"A", "A 2", "C"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %#v, want %#v", names, want)
		}
	}
}

func TestMergeDropsExactDuplicates(t *testing.T) {
	remote := []Node{{Name: "A", Protocol: "ss", Server: "a.example", Port: 1, Params: map[string]string{"password": "x"}}}
	pinned := []Node{{Name: "Pinned A", Protocol: "ss", Server: "a.example", Port: 1, Params: map[string]string{"password": "x"}, Pinned: true}}
	got := MergeNodes(remote, pinned, MergeOptions{Mode: "after_remote"})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(got), got)
	}
}
