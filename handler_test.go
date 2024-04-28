package fseventserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
)

type FakeHandler struct{}

func (self *FakeHandler) ServeFSEvent(ctx context.Context) error {
	fmt.Print(ctx)
	return nil
}

func TestRegisterHandler(t *testing.T) {
	handler := &FakeHandler{}
	serverMux := NewServerMux()
	serverMux.register("/tmp", handler)
	key, _ := glob.Compile("/tmp")
	assert.Equal(t, key, serverMux.store[0].key)
}

func TestFindHandler(t *testing.T) {
	handler1 := &FakeHandler{}
	handler2 := &FakeHandler{}
	serverMux := NewServerMux()

	serverMux.register("/mnt/**", handler2)
	serverMux.register("/tmp/**", handler1)
	assert.Equal(t, 2, len(serverMux.store))

	ctx := context.Background()
	req := &request{path: "/tmp/Videos"}
	ctx = context.WithValue(ctx, "request", req)
	actual := serverMux.findHandler(ctx)
	assert.Equal(t, handler1, actual)
}
