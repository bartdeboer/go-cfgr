// Package jsondoc provides ordered JSON documents with JSON Pointer access.
package jsondoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var (
	ErrInvalidJSON    = errors.New("jsondoc: invalid JSON document")
	ErrInvalidPointer = errors.New("jsondoc: invalid JSON pointer")
)

type Document struct{ root *node }

type node struct {
	scalar   any
	keys     []string
	object   map[string]*node
	array    []*node
	isObject bool
	isArray  bool
}

func Parse(data []byte) (*Document, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	root, err := readNode(decoder)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("%w: multiple JSON values", ErrInvalidJSON)
	}
	return &Document{root: root}, nil
}

func (d *Document) IsObject() bool { return d != nil && d.root != nil && d.root.isObject }

func (d *Document) Decode(dst any) error {
	data, err := d.Marshal()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("jsondoc: decode document: %w", err)
	}
	return nil
}

func (d *Document) Marshal() ([]byte, error) {
	if d == nil || d.root == nil {
		return nil, fmt.Errorf("%w: document is empty", ErrInvalidJSON)
	}
	var output bytes.Buffer
	if err := d.root.writeJSON(&output); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	return output.Bytes(), nil
}

func (d *Document) Get(pointer string) (any, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}
	n, err := nodeAt(d.root, tokens)
	if err != nil {
		return nil, err
	}
	return n.value(), nil
}

func (d *Document) Set(pointer string, value any) error {
	replacement, err := nodeFromValue(value)
	if err != nil {
		return err
	}
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		d.root = replacement
		return nil
	}
	parent, err := nodeAt(d.root, tokens[:len(tokens)-1])
	if err != nil {
		return err
	}
	last := tokens[len(tokens)-1]
	if parent.isObject {
		if _, exists := parent.object[last]; !exists {
			parent.keys = append(parent.keys, last)
		}
		parent.object[last] = replacement
		return nil
	}
	if parent.isArray {
		index, err := arrayIndex(last, len(parent.array))
		if err != nil {
			return err
		}
		parent.array[index] = replacement
		return nil
	}
	return fmt.Errorf("%w: parent is a scalar", ErrInvalidPointer)
}

// Unset removes the value addressed by pointer. Removing an array value shifts
// the following values left. The document root cannot be unset.
func (d *Document) Unset(pointer string) error {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return fmt.Errorf("%w: document root cannot be unset", ErrInvalidPointer)
	}
	parent, err := nodeAt(d.root, tokens[:len(tokens)-1])
	if err != nil {
		return err
	}
	last := tokens[len(tokens)-1]
	if parent.isObject {
		if _, exists := parent.object[last]; !exists {
			return fmt.Errorf("%w: value %q does not exist", ErrInvalidPointer, last)
		}
		delete(parent.object, last)
		keys := parent.keys[:0]
		for _, key := range parent.keys {
			if key != last {
				keys = append(keys, key)
			}
		}
		parent.keys = keys
		return nil
	}
	if parent.isArray {
		index, err := arrayIndex(last, len(parent.array))
		if err != nil {
			return err
		}
		parent.array = append(parent.array[:index], parent.array[index+1:]...)
		return nil
	}
	return fmt.Errorf("%w: parent is a scalar", ErrInvalidPointer)
}

// Merge overlays object documents from left to right. Objects merge
// recursively; arrays, scalars, and null replace earlier values.
func Merge(documents ...*Document) (*Document, error) {
	result := &node{isObject: true, object: make(map[string]*node)}
	for index, document := range documents {
		if document == nil || !document.IsObject() {
			return nil, fmt.Errorf("%w: document %d root must be an object", ErrInvalidJSON, index+1)
		}
		result = mergeNodes(result, document.root)
	}
	return &Document{root: result}, nil
}

func readNode(decoder *json.Decoder) (*node, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return &node{scalar: token}, nil
	}
	switch delimiter {
	case '{':
		result := &node{isObject: true, object: make(map[string]*node)}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("object key is not a string")
			}
			if _, exists := result.object[key]; exists {
				return nil, fmt.Errorf("duplicate object key %q", key)
			}
			child, err := readNode(decoder)
			if err != nil {
				return nil, err
			}
			result.keys = append(result.keys, key)
			result.object[key] = child
		}
		_, err := decoder.Token()
		return result, err
	case '[':
		result := &node{isArray: true}
		for decoder.More() {
			child, err := readNode(decoder)
			if err != nil {
				return nil, err
			}
			result.array = append(result.array, child)
		}
		_, err := decoder.Token()
		return result, err
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delimiter)
	}
}

func nodeFromValue(value any) (*node, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	document, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return document.root, nil
}

func nodeAt(current *node, tokens []string) (*node, error) {
	for _, token := range tokens {
		switch {
		case current.isObject:
			next, exists := current.object[token]
			if !exists {
				return nil, fmt.Errorf("%w: value %q does not exist", ErrInvalidPointer, token)
			}
			current = next
		case current.isArray:
			index, err := arrayIndex(token, len(current.array))
			if err != nil {
				return nil, err
			}
			current = current.array[index]
		default:
			return nil, fmt.Errorf("%w: cannot traverse scalar at %q", ErrInvalidPointer, token)
		}
	}
	return current, nil
}

func (n *node) value() any {
	if n.isObject {
		value := make(map[string]any, len(n.object))
		for key, child := range n.object {
			value[key] = child.value()
		}
		return value
	}
	if n.isArray {
		value := make([]any, len(n.array))
		for index, child := range n.array {
			value[index] = child.value()
		}
		return value
	}
	return n.scalar
}

func (n *node) writeJSON(output *bytes.Buffer) error {
	if n.isObject {
		output.WriteByte('{')
		for index, key := range n.keys {
			if index > 0 {
				output.WriteByte(',')
			}
			encodedKey, _ := json.Marshal(key)
			output.Write(encodedKey)
			output.WriteByte(':')
			if err := n.object[key].writeJSON(output); err != nil {
				return err
			}
		}
		output.WriteByte('}')
		return nil
	}
	if n.isArray {
		output.WriteByte('[')
		for index, child := range n.array {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := child.writeJSON(output); err != nil {
				return err
			}
		}
		output.WriteByte(']')
		return nil
	}
	data, err := json.Marshal(n.scalar)
	if err == nil {
		output.Write(data)
	}
	return err
}

func mergeNodes(low, high *node) *node {
	if !low.isObject || !high.isObject {
		return cloneNode(high)
	}
	merged := cloneNode(low)
	for _, key := range high.keys {
		highChild := high.object[key]
		if lowChild, exists := merged.object[key]; exists {
			merged.object[key] = mergeNodes(lowChild, highChild)
			continue
		}
		merged.keys = append(merged.keys, key)
		merged.object[key] = cloneNode(highChild)
	}
	return merged
}

func cloneNode(source *node) *node {
	clone := &node{scalar: source.scalar, isObject: source.isObject, isArray: source.isArray}
	if source.isObject {
		clone.keys = append([]string(nil), source.keys...)
		clone.object = make(map[string]*node, len(source.object))
		for key, child := range source.object {
			clone.object[key] = cloneNode(child)
		}
	}
	for _, child := range source.array {
		clone.array = append(clone.array, cloneNode(child))
	}
	return clone
}

func pointerTokens(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if pointer[0] != '/' {
		return nil, fmt.Errorf("%w: must be empty or start with '/'", ErrInvalidPointer)
	}
	raw := strings.Split(pointer[1:], "/")
	tokens := make([]string, len(raw))
	for index, token := range raw {
		replaced := strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		if strings.Contains(replaced, "~") && !validPointerToken(token) {
			return nil, fmt.Errorf("%w: invalid escape in token %q", ErrInvalidPointer, token)
		}
		tokens[index] = replaced
	}
	return tokens, nil
}

func validPointerToken(token string) bool {
	for index := 0; index < len(token); index++ {
		if token[index] == '~' {
			if index+1 >= len(token) || token[index+1] != '0' && token[index+1] != '1' {
				return false
			}
			index++
		}
	}
	return true
}

func arrayIndex(token string, length int) (int, error) {
	if token == "-" {
		return 0, fmt.Errorf("%w: array append is not supported", ErrInvalidPointer)
	}
	if token == "" || len(token) > 1 && token[0] == '0' {
		return 0, fmt.Errorf("%w: invalid array index %q", ErrInvalidPointer, token)
	}
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 || index >= length {
		return 0, fmt.Errorf("%w: array index %q is out of bounds", ErrInvalidPointer, token)
	}
	return index, nil
}

// OrderedKeys returns the keys of the object at pointer in document order.
func (d *Document) OrderedKeys(pointer string) ([]string, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}
	n, err := nodeAt(d.root, tokens)
	if err != nil {
		return nil, err
	}
	if !n.isObject {
		return nil, fmt.Errorf("%w: value is not an object", ErrInvalidPointer)
	}
	return append([]string(nil), n.keys...), nil
}
