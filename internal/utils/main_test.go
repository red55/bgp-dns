package utils

import "testing"

// equalStrings compares two string slices element-wise, treating a nil slice
// as empty.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// refDifference is an independent, deliberately naive linear-scan
// implementation of the Difference contract (slice1 \ slice2, first
// occurrence wins) used only to cross-check the map-based Difference on
// fixed inputs.
func refDifference(a, b []string) []string {
	memberOf := func(set []string, x string) bool {
		for _, s := range set {
			if s == x {
				return true
			}
		}
		return false
	}
	var out []string
	for _, x := range a {
		if memberOf(b, x) || memberOf(out, x) {
			continue
		}
		out = append(out, x)
	}
	return out
}

// TestDifference exercises the difference contract: result is
// slice1 minus slice2 in slice1's original relative order, duplicate entries
// of slice1 collapse to their first occurrence, and slice2 never contributes
// elements. Nil slices behave as empty. This asymmetry is load-bearing for
// the cache upsert path, where gone=Difference(prev, cur) and
// arrived=Difference(cur, prev): a fresh entry (prev empty) must yield
// gone==nil and arrived==all current IPs.
func TestDifference(t *testing.T) {
	tests := []struct {
		name   string
		slice1 []string
		slice2 []string
		want   []string
	}{
		{
			name:   "no overlap returns all of slice1",
			slice1: []string{"a", "b"},
			slice2: []string{"c", "d"},
			want:   []string{"a", "b"},
		},
		{
			name:   "identical sets yields nothing",
			slice1: []string{"x", "y"},
			slice2: []string{"x", "y"},
			want:   nil,
		},
		{
			name: "both empty",
			want: nil,
		},
		{
			name:   "first empty yields nothing (fresh-entry case)",
			slice2: []string{"a", "b"},
			want:   nil,
		},
		{
			name:   "second empty returns all of slice1 (arrived case)",
			slice1: []string{"a", "b"},
			want:   []string{"a", "b"},
		},
		{
			name:   "partial overlap keeps non-overlapping members in order",
			slice1: []string{"a", "b"},
			slice2: []string{"b", "c"},
			want:   []string{"a"},
		},
		{
			name:   "order preserved from slice1",
			slice1: []string{"c", "a", "b"},
			slice2: []string{"b"},
			want:   []string{"c", "a"},
		},
		{
			name:   "duplicates in slice1 collapse to first occurrence",
			slice1: []string{"x", "x", "y"},
			slice2: []string{"x", "z"},
			want:   []string{"y"},
		},
		{
			name:   "new IP arriving while another disappears",
			slice1: []string{"10.0.0.1", "10.0.0.2"},
			slice2: []string{"10.0.0.1"},
			want:   []string{"10.0.0.2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Difference(tc.slice1, tc.slice2)
			if !equalStrings(got, tc.want) {
				t.Errorf("Difference(%v, %v) = %v, want %v", tc.slice1, tc.slice2, got, tc.want)
			}
		})
	}

	// Interleaved hand-picked deterministic input pair (20 elements in
	// slice1 incl. one duplicate; 16 in slice2) with expected output computed
	// by the contract: slice1 \ slice2 = [a b d f h j l n p r].
	t.Run("hand-picked 36 element inputs", func(t *testing.T) {
		slice1 := []string{"a", "b", "c", "d", "e", "a", "f", "g", "h", "i",
			"j", "k", "l", "m", "n", "o", "p", "q", "r", "s"}
		slice2 := []string{"c", "e", "g", "i", "k", "m", "o", "q", "s",
			"t", "u", "v", "w", "x", "c", "t"}
		want := []string{"a", "b", "d", "f", "h", "j", "l", "n", "p", "r"}

		got := Difference(slice1, slice2)
		if !equalStrings(got, want) {
			t.Errorf("Difference = %v, want %v", got, want)
		}
		ref := refDifference(slice1, slice2)
		if !equalStrings(ref, want) {
			t.Errorf("reference disagree: ref = %v, want %v", ref, want)
		}
		if !equalStrings(got, ref) {
			t.Errorf("Difference = %v disagrees with reference = %v", got, ref)
		}
	})
}
