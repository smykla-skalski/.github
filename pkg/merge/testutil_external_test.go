package merge_test

// NOTE: These test helper functions (arraysEqual, valuesEqual, mapsEqual) are intentionally
// duplicated from testutil_test.go to support package isolation. The merge_test package (external
// tests) and merge package (internal tests) cannot share helper functions directly due to different
// package namespaces. This duplication ensures both test suites have access to the same utilities.

// arraysEqual compares two slices deeply.
func arraysEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !valuesEqual(a[i], b[i]) {
			return false
		}
	}

	return true
}

// valuesEqual compares two values deeply.
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	switch va := a.(type) {
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok {
			return false
		}

		return mapsEqual(va, vb)
	case []any:
		vb, ok := b.([]any)
		if !ok {
			return false
		}

		return arraysEqual(va, vb)
	default:
		return a == b
	}
}

// mapsEqual compares two maps deeply.
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}

		if !valuesEqual(va, vb) {
			return false
		}
	}

	return true
}
