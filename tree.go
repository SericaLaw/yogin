package yogin

import (
	"fmt"
	"path"
	"strings"
)

type node struct {
	segment   string
	handlers  HandlersChain
	children  []*node
	wildChild *node	// at most one :param or *catchAll style child
	path      string
	fullPath  string
}

type Param struct {
	Key   string
	Value string
}

type Params []Param

func (ps Params) Get(name string) (string, bool) {
	for _, entry := range ps {
		if entry.Key == name {
			return entry.Value, true
		}
	}
	return "", false
}

func (ps Params) ByName(name string) (va string) {
	va, _ = ps.Get(name)
	return
}

type nodeValue struct {
	handlers	HandlersChain
	params 		Params
	fullPath	string
}

func (n *node) insertChild(segments []string, level int, fullPath string, handlers HandlersChain) {
	if len(segments) == level {
		if n.fullPath != "" {
			panic(fmt.Sprintf("new route %s conflicts with existing route %s", fullPath, n.fullPath))
		}
		n.fullPath = fullPath
		n.handlers = handlers
		return
	}

	segment := segments[level]

	if isCatchAll(segment) && len(n.children) > 0 {
		panic(fmt.Sprintf("catch-all conflicts with existing handle for the path segment root in path %s", fullPath))
	}

	if isWild(segment) && n.wildChild != nil && n.wildChild.segment != segment {
		panic(fmt.Sprintf("%s in new path %s conflicts with existing wildcard %s in existing prefix %s", segment, fullPath, n.wildChild.segment, n.wildChild.path))
	}

	child := n.matchChild(segment)
	isWild := isWild(segment)

	// create new node
	if child == nil {
		child = &node{segment: segment, path: path.Join(n.path, segment)}
		if isWild {
			n.wildChild = child
		} else {
			n.children = append(n.children, child)
		}
	}

	if isCatchAll(segment) && level+1 != len(segments) {
		panic(fmt.Sprintf("catch-all routes are only allowed at the end of the path %v", fullPath))
	}

	child.insertChild(segments, level+1, fullPath, handlers)
}

func (n *node) matchChild(segment string) *node {
	// let the caller judge if there is conflict
	if isCatchAll(segment) || isParam(segment) {
		if n.wildChild != nil && n.wildChild.segment != segment {
			panic("conflict")
		}
		return n.wildChild
	}
	for _, child := range n.children {
		if child.segment == segment {
			return child
		}
	}

	return nil
}

func (n *node) getValue(segments []string, level int) *node {
	if len(segments) == level || strings.HasPrefix(n.segment, "*") {
		if n.handlers == nil {
			return nil
		}
		return n
	}

	segment := segments[level]
	child := n.matchChildAndWild(segment)
	if child == nil {
		return nil
	}

	return child.getValue(segments, level+1)
}

func (n *node) matchChildAndWild(segment string) *node {
	for _, child := range n.children {
		if child.segment == segment {
			return child
		}
	}
	// if no string literal child, return wildChild (which can also be nil)
	return n.wildChild
}

type methodTree struct {
	method	string
	root 	*node
}

// addRoute allows at most one :param or *catchAll style segment at each position
// *catchAll should only appears at the end of the route
func (t *methodTree) addRoute(path string, handlers HandlersChain) {
	segments := parseSegments(path)
	t.root.insertChild(segments, 0, path, handlers)
}

// getRoute always tries to match string literal first
// E.g. If /:hello/world and /hello/:world both exist, for URL /hello/world,
// it will match /hello/:world
func (t *methodTree) getRoute(path string) (value nodeValue) {
	segments := parseSegments(path)
	n := t.root.getValue(segments, 0)

	if n != nil {
		value.handlers = n.handlers
		value.fullPath = n.fullPath

		patterns := parseSegments(n.fullPath)
		value.params = make(Params, 0)
		for index, pattern := range patterns {
			if pattern[0] == ':' {
				value.params = append(value.params, Param{pattern[1:], segments[index]})
			} else if pattern[0] == '*' && len(pattern) > 1 {
				value.params = append(value.params, Param{pattern[1:], strings.Join(segments[index:], "/")})
			}
		}
	}
	return
}

func parseSegments(path string) []string {
	vs := strings.Split(path, "/")

	segments := make([]string, 0)
	for _, item := range vs {
		if item != "" {
			segments = append(segments, item)
		}
	}
	return segments
}

func isCatchAll(segment string) bool {
	return strings.HasPrefix(segment, "*")
}

func isParam(segment string) bool {
	return strings.HasPrefix(segment, ":")
}

func isWild(segment string) bool {
	return isCatchAll(segment) || isParam(segment)
}
