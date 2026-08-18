package utils

// Difference returns the elements present in slice1 but absent from slice2,
// preserving slice1's original relative order. Membership testing is
// map-based, so the function runs in O(len(slice1)+len(slice2)) instead of
// the previous nested-loop quadratic scan. Duplicate entries collapse to a
// single occurrence — intentional: callers feed BGP ref-counted /32
// operations where duplicates carry no meaning (Advance gates on the first
// reference, Withdraw on the last, so duplicate IPs within one call were
// always a no-op even under the old occurrence-preserving behavior).
// Nil slices behave as empty.
func Difference(slice1 []string, slice2 []string) (diff []string) {
	inSlice2 := make(map[string]struct{}, len(slice2))
	for _, s := range slice2 {
		inSlice2[s] = struct{}{}
	}

	added := make(map[string]struct{})
	for _, s := range slice1 {
		if _, ok := inSlice2[s]; !ok {
			if _, dup := added[s]; !dup {
				diff = append(diff, s)
				added[s] = struct{}{}
			}
		}
	}

	return diff
}
