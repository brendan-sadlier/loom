package cli

import (
	"bytes"
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	targets, ov := parseArgs([]string{"x=1", "a=b=c", "./bin/app", "1x=2", "build"})
	// if err != nil {
	//	t.Fatal(err)
	// }
	if want := []string{"bin/app", "1x=2", "build"}; !reflect.DeepEqual(targets, want) {
		t.Errorf("targets = %q, want %q", targets, want)
	}
	if want := map[string]string{"x": "1", "a": "b=c"}; !reflect.DeepEqual(ov, want) {
		t.Errorf("overrides = %v, want %v", ov, want)
	}
}

func TestResolveColor(t *testing.T) {
	var buf bytes.Buffer
	for _, tc := range []struct {
		mode string
		want bool
	}{{"always", true}, {"never", false}, {"auto", false}} { // a Buffer is never a terminal
		got, err := resolveColor(tc.mode, &buf)
		if err != nil || got != tc.want {
			t.Errorf("%s: got %v, %v", tc.mode, got, err)
		}
	}
	if _, err := resolveColor("sometimes", &buf); err == nil {
		t.Error("invalid mode accepted")
	}
}
