package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func assertJSONEq(t *testing.T, want, got []byte) {
	t.Helper()
	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unmarshal want json: %v", err)
	}
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got json: %v", err)
	}
	if !deepEqualJSON(wantValue, gotValue) {
		t.Fatalf("json mismatch\nwant: %s\ngot:  %s", want, got)
	}
}

func deepEqualJSON(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
