package fseventserver

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeDepth(t *testing.T) {
	var server Server

	data := map[string]int{
		"/a/b /a":     1,
		"/a/b/c /a":   2,
		"/a/b/c /a/b": 1,
		"/a/b/c/d /a": 3,
		"/b/c/d /a":   -1,
		"/a /a":       0,
	}

	for key, value := range data {
		fields := strings.Fields(key)
		path := fields[0]
		root := fields[1]
		assert.Equal(t, value, server.computeDepth(path, root))
	}
}

func TestWalk(t *testing.T) {
	var err error

	tmpDir, err := prepareTestDirTree("/a/b/c")
	if err != nil {
		t.Fatal("cannot create test directories")
	}

	expected := []string{tmpDir, filepath.Join(tmpDir, "/a"), filepath.Join(tmpDir, "/a/b"), filepath.Join(tmpDir, "/a/b/c")}

	var server Server
	var buf bytes.Buffer
	server.walk(tmpDir, &buf)

	data, err := io.ReadAll(&buf)
	if err != nil {
		t.Fatal("cannot read from buffer")
	}
	actual := strings.Split(string(data), "\n")

	// ReadAll add an extra empty string at the end
	actual = actual[:len(actual)-1]
	assert.Equal(t, expected, actual)
}

func TestWalkWithMaxDepth(t *testing.T) {
	var err error

	tmpDir, err := prepareTestDirTree("/a/b/c")
	if err != nil {
		t.Fatal("cannot create test directories")
	}

	expected := []string{tmpDir, filepath.Join(tmpDir, "/a")}

	server := &Server{
		MaxDepth: 1,
	}

	var buf bytes.Buffer
	server.walk(tmpDir, &buf)

	data, err := io.ReadAll(&buf)
	if err != nil {
		t.Fatal("cannot read from buffer")
	}

	actual := strings.Split(string(data), "\n")
	// ReadAll add an extra empty string at the end
	actual = actual[:len(actual)-1]
	assert.Equal(t, expected, actual)
}

func TestWalkWithIgnoreList(t *testing.T) {
	var err error

	tmpDir, err := prepareTestDirTree("/a/b/c")

	if err != nil {
		t.Fatal("cannot create test directories")
	}

	expected := []string{tmpDir}

	server := &Server{
		IgnoreList: []string{"*/a/**", "*/a*"},
	}

	var buf bytes.Buffer

	server.walk(tmpDir, &buf)

	data, err := io.ReadAll(&buf)
	if err != nil {
		t.Fatal("cannot read from buffer")
	}

	actual := strings.Split(string(data), "\n")
	// ReadAll add an extra empty string at the end
	actual = actual[:len(actual)-1]
	assert.Equal(t, expected, actual)
}

// source https://github.com/golang/go/blob/master/src/path/filepath/example_unix_walk_test.go#L16C1-L29C2
func prepareTestDirTree(tree string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %v\n", err)
	}
	err = os.MkdirAll(filepath.Join(tmpDir, tree), 0755)

	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}
	return tmpDir, nil
}
