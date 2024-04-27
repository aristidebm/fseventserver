package main

import (
	"context"

	"example.com/fseventserver"
)



func main() {
    fseventserver.HandleFunc("/tmp/Videos", func(ctx context.Context) error {
        return nil
    })
    fseventserver.ListenAndServe("/tmp", nil)
}
