package fseventserver

import (
	"context"
	"testing"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
)

type FakeHandler struct{}

func (self *FakeHandler) ServeFSEvent(ctx context.Context) error {
	return nil
}

func TestRegisterHandler(t *testing.T) {
	handler := &FakeHandler{}
	serverMux := NewServerMux()
	serverMux.register("/tmp", handler)
	key, _ := glob.Compile("/tmp")
	assert.Equal(t, key, serverMux.store[0].key)
}

func TestRegisterPatternTwice(t *testing.T) {
	handler := &FakeHandler{}
	serverMux := NewServerMux()
	serverMux.register("/tmp", handler)
	assert.Equal(t, 1, len(serverMux.store))
	assert.Error(t, serverMux.register("/tmp", handler))
}

func TestFindHandler(t *testing.T) {
	handler1 := &FakeHandler{}
	handler2 := &FakeHandler{}
	serverMux := NewServerMux()

	serverMux.register("/mnt/**", handler2)
	serverMux.register("/tmp/**", handler1)
	assert.Equal(t, 2, len(serverMux.store))

	ctx := context.Background()
	req := &Request{path: "/tmp/Videos"}
	ctx = context.WithValue(ctx, "request", req)
	actual := serverMux.findHandler(ctx)
	assert.Equal(t, handler1, actual)
}
