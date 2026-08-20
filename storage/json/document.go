package json

import "github.com/bartdeboer/go-cfgr/jsondoc"

type Document = jsondoc.Document

func NewDocument(data []byte) (*Document, error) {
	return jsondoc.Parse(data)
}
