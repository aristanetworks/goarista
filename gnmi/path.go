// Copyright (c) 2017 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"fmt"
	"sort"
	"strings"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

// nextTokenIndex returns the end index of the first token.
func nextTokenIndex(path string) int {
	var inBrackets bool
	var escape bool
	for i, c := range path {
		switch c {
		case '[':
			inBrackets = true
			escape = false
		case ']':
			if !escape {
				inBrackets = false
			}
			escape = false
		case '\\':
			escape = !escape
		case '/':
			if !inBrackets && !escape {
				return i
			}
			escape = false
		default:
			escape = false
		}
	}
	return len(path)
}

// SplitPath splits a gnmi path according to the spec. See
// https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-path-conventions.md
// No validation is done. Behavior is undefined if path is an invalid
// gnmi path. TODO: Do validation?
func SplitPath(path string) []string {
	var result []string
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	for len(path) > 0 {
		i := nextTokenIndex(path)
		result = append(result, path[:i])
		path = path[i:]
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}
	}
	return result
}

// SplitPaths splits multiple gnmi paths
func SplitPaths(paths []string) [][]string {
	out := make([][]string, len(paths))
	for i, path := range paths {
		out[i] = SplitPath(path)
	}
	return out
}

// StrPath builds a human-readable form of a gnmi path.
// e.g. /a/b/c[e=f]
func StrPath(path *pb.Path) string {
	if path == nil {
		return "/"
	} else if len(path.Elem) != 0 {
		return strPathV04(path)
	} else if len(path.Element) != 0 {
		return strPathV03(path)
	}
	return "/"
}

// writeKey is used as a helper to contain the logic of writing keys as a string.
func writeKey(b *strings.Builder, key map[string]string) {
	// Sort the keys so that they print in a consistent
	// order. We don't have the YANG AST information, so the
	// best we can do is sort them alphabetically.
	size := 0
	keys := make([]string, 0, len(key))
	for k, v := range key {
		keys = append(keys, k)
		size += len(k) + len(v) + 3 // [, =, ]
	}
	sort.Strings(keys)
	b.Grow(size)
	for _, k := range keys {
		b.WriteByte('[')
		b.WriteString(escapeKey(k))
		b.WriteByte('=')
		b.WriteString(escapeValue(key[k]))
		b.WriteByte(']')
	}
}

// KeyToString is used to get the string representation of the keys.
func KeyToString(key map[string]string) string {
	if len(key) == 1 {
		for k, v := range key {
			return "[" + escapeKey(k) + "=" + escapeValue(v) + "]"
		}
	}
	var b strings.Builder
	writeKey(&b, key)
	return b.String()
}

func writeElem(b *strings.Builder, elm *pb.PathElem) {
	b.WriteString(escapeName(elm.Name))
	if len(elm.Key) > 0 {
		writeKey(b, elm.Key)
	}
}

func escapeKey(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `=`, `\=`)
	return s
}

func escapeValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `]`, `\]`)
	return s
}

func escapeName(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `/`, `\/`)
	s = strings.ReplaceAll(s, `[`, `\[`)
	return s
}

// ElemToString is used to get the string representation of the Element.
func ElemToString(elm *pb.PathElem) string {
	b := &strings.Builder{}
	writeElem(b, elm)
	return b.String()
}

// strPathV04 handles the v0.4 gnmi and later path.Elem member.
func strPathV04(path *pb.Path) string {
	b := &strings.Builder{}
	for _, elm := range path.Elem {
		b.WriteRune('/')
		writeElem(b, elm)
	}
	return b.String()
}

// strPathV03 handles the v0.3 gnmi and earlier path.Element member.
func strPathV03(path *pb.Path) string {
	return "/" + strings.Join(path.Element, "/")
}

// upgradePath modernizes a Path by translating the contents of the Element field to Elem
func upgradePath(path *pb.Path) *pb.Path {
	if path != nil && len(path.Elem) == 0 {
		var elems []*pb.PathElem
		for _, element := range path.Element {
			n, keys, _ := parseElement(element)
			elems = append(elems, &pb.PathElem{Name: n, Key: keys})
		}
		path.Elem = elems
		path.Element = nil
	}
	return path
}

// JoinPaths joins multiple gnmi paths into a single path
func JoinPaths(paths ...*pb.Path) *pb.Path {
	var elems []*pb.PathElem
	for _, path := range paths {
		if path != nil {
			path = upgradePath(path)
			elems = append(elems, path.Elem...)
		}
	}
	return &pb.Path{Elem: elems}
}

// ParseGNMIElements builds up a gnmi path, from user-supplied text
func ParseGNMIElements(elms []string) (*pb.Path, error) {
	var parsed []*pb.PathElem
	for _, e := range elms {
		n, keys, err := parseElement(e)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, &pb.PathElem{Name: n, Key: keys})
	}
	return &pb.Path{
		Element: elms, // Backwards compatibility with pre-v0.4 gnmi
		Elem:    parsed,
	}, nil
}

// parseElement parses a path element, according to the gNMI specification. See
// https://github.com/openconfig/reference/blame/master/rpc/gnmi/gnmi-path-conventions.md
//
// It returns the first string (the current element name), and an optional map of key name
// value pairs.
func parseElement(pathElement string) (string, map[string]string, error) {
	// First check if there are any keys, i.e. do we have at least one '[' in the element
	name, keyStart := findUnescaped(pathElement, '[')
	if keyStart < 0 {
		return name, nil, nil
	}

	// Error if there is no element name or if the "[" is at the beginning of the path element
	if len(name) == 0 {
		return "", nil, fmt.Errorf("failed to find element name in %q", pathElement)
	}

	// Look at the keys now.
	keys := make(map[string]string)
	keyPart := pathElement[keyStart:]
	for keyPart != "" {
		k, v, nextKey, err := parseKey(keyPart)
		if err != nil {
			return "", nil, err
		}
		keys[k] = v
		keyPart = nextKey
	}
	return name, keys, nil
}

// parseKey returns the key name, key value and the remaining string to be parsed,
func parseKey(s string) (string, string, string, error) {
	if s[0] != '[' {
		return "", "", "", fmt.Errorf("failed to find opening '[' in %q", s)
	}
	k, iEq := findUnescaped(s[1:], '=')
	if iEq < 0 {
		return "", "", "", fmt.Errorf("failed to find '=' in %q", s)
	}

	rhs := s[1+iEq+1:]
	v, iClosBr := findUnescaped(rhs, ']')
	if iClosBr < 0 {
		return "", "", "", fmt.Errorf("failed to find ']' in %q", s)
	}

	next := rhs[iClosBr+1:]
	return k, v, next, nil
}

// findUnescaped will return the index of the first unescaped match of 'find', and the unescaped
// string leading up to it.
func findUnescaped(s string, find byte) (string, int) {
	// Take a fast track if there are no escape sequences
	if strings.IndexByte(s, '\\') == -1 {
		i := strings.IndexByte(s, find)
		if i < 0 {
			return s, -1
		}
		return s[:i], i
	}

	// Find the first match, taking care of escaped chars.
	var b strings.Builder
	var i int
	len := len(s)
	for i = 0; i < len; {
		ch := s[i]
		if ch == find {
			return b.String(), i
		} else if ch == '\\' && i < len-1 {
			i++
			ch = s[i]
		}
		b.WriteByte(ch)
		i++
	}
	return b.String(), -1
}
