package jsondoc_test

import (
	"errors"
	"testing"

	"github.com/bartdeboer/go-cfgr/jsondoc"
)

func TestDocumentUpdatesValuesInObjectOrder(t *testing.T) {
	document, err := jsondoc.Parse([]byte(`{"name":"service","server":{"host":"localhost","port":8080},"enabled":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := document.Set("/server/port", 9090); err != nil {
		t.Fatal(err)
	}
	if err := document.Set("/server/tls", true); err != nil {
		t.Fatal(err)
	}

	contents, err := document.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"name":"service","server":{"host":"localhost","port":9090,"tls":true},"enabled":false}`
	if string(contents) != want {
		t.Fatalf("contents = %s, want %s", contents, want)
	}
}

func TestDocumentUnsetsObjectAndArrayValues(t *testing.T) {
	document, err := jsondoc.Parse([]byte(`{"first":1,"obsolete":true,"last":3,"items":["keep","remove","last"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := document.Unset("/obsolete"); err != nil {
		t.Fatal(err)
	}
	if err := document.Unset("/items/1"); err != nil {
		t.Fatal(err)
	}

	contents, err := document.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"first":1,"last":3,"items":["keep","last"]}`
	if string(contents) != want {
		t.Fatalf("contents = %s, want %s", contents, want)
	}
}

func TestParseRejectsDuplicateObjectKeys(t *testing.T) {
	_, err := jsondoc.Parse([]byte(`{"enabled":false,"enabled":true}`))
	if !errors.Is(err, jsondoc.ErrInvalidJSON) {
		t.Fatalf("Parse() error = %v, want ErrInvalidJSON", err)
	}
}

func TestMergeRecursesThroughObjectsAndReplacesOtherValues(t *testing.T) {
	defaults, err := jsondoc.Parse([]byte(`{"server":{"host":"localhost","port":8080},"items":["default"]}`))
	if err != nil {
		t.Fatal(err)
	}
	local, err := jsondoc.Parse([]byte(`{"server":{"port":9090},"items":["local"],"enabled":true}`))
	if err != nil {
		t.Fatal(err)
	}

	merged, err := jsondoc.Merge(defaults, local)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := merged.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"server":{"host":"localhost","port":9090},"items":["local"],"enabled":true}`
	if string(contents) != want {
		t.Fatalf("contents = %s, want %s", contents, want)
	}
}
