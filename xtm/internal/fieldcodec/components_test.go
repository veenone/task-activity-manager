package fieldcodec

import (
	"reflect"
	"testing"
)

func TestComponentsRoundTrip(t *testing.T) {
	enc := EncodeComponents([]string{" User Management ", "", "API"})
	if enc != "\nUser Management\nAPI\n" {
		t.Fatalf("encode %q", enc)
	}
	if got := DecodeComponents(enc); !reflect.DeepEqual(got, []string{"User Management", "API"}) {
		t.Fatalf("decode %v", got)
	}
	if EncodeComponents(nil) != "" {
		t.Fatal("empty list should encode to empty string")
	}
	if got := DecodeComponents(""); got == nil || len(got) != 0 {
		t.Fatalf("decode empty %#v", got)
	}
	if ComponentFilterPattern("API") != "%\nAPI\n%" {
		t.Fatalf("pattern %q", ComponentFilterPattern("API"))
	}
}
