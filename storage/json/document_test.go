package json_test

import (
	"errors"
	"testing"

	"github.com/bartdeboer/go-cfgr/storage/json"
)

func TestJSONPointerReadsEscapedObjectField(t *testing.T) {
	doc, err := json.NewDocument([]byte(`{"profile":{"display/name":"Ada","til~de":true}}`))
	if err != nil {
		t.Fatal(err)
	}

	name, err := doc.Get("/profile/display~1name")
	if err != nil {
		t.Fatal(err)
	}
	tilde, err := doc.Get("/profile/til~0de")
	if err != nil {
		t.Fatal(err)
	}

	if name != "Ada" || tilde != true {
		t.Fatalf("escaped fields: name=%v tilde=%v", name, tilde)
	}
}

func TestJSONPointerUpdatesFieldsWithoutReorderingObjects(t *testing.T) {
	doc, err := json.NewDocument([]byte(`{"profile":{},"items":[{"name":"first"}]}`))
	if err != nil {
		t.Fatal(err)
	}

	if err := doc.Set("/profile/name", "Ada"); err != nil {
		t.Fatal(err)
	}
	if err := doc.Set("/items/0/name", "updated"); err != nil {
		t.Fatal(err)
	}

	data, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"profile":{"name":"Ada"},"items":[{"name":"updated"}]}`; got != want {
		t.Fatalf("updated JSON = %s, want %s", got, want)
	}
}

func TestJSONPointerRejectsUnsupportedPaths(t *testing.T) {
	doc, err := json.NewDocument([]byte(`{"items":[]}`))
	if err != nil {
		t.Fatal(err)
	}

	for _, pointer := range []string{
		"items",          // missing leading slash
		"/bad~2escape",   // invalid escape
		"/items/-",       // array append is not supported
		"/missing/child", // missing parent
	} {
		if err := doc.Set(pointer, true); !errors.Is(err, json.ErrInvalidPointer) {
			t.Errorf("Set(%q) error = %v, want ErrInvalidPointer", pointer, err)
		}
	}
}

func TestJSONDocumentRequiresExactlyOneValidValue(t *testing.T) {
	for _, input := range []string{
		`{"broken":`,
		`{}{}`,
		`{} trailing-junk`,
	} {
		if _, err := json.NewDocument([]byte(input)); !errors.Is(err, json.ErrInvalidJSON) {
			t.Errorf("NewJSONDocument(%q) error = %v, want ErrInvalidJSON", input, err)
		}
	}
}
