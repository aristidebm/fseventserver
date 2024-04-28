package main

import (
	"context"
	"fmt"
	"log"

	"example.com/fseventserver"
)



func main() {
    fseventserver.HandleFunc("/mnt/filedispatch/downloads/**", func(ctx context.Context) error {
        value := ctx.Value("request")
        req := value.(*fseventserver.Request)
        fmt.Printf("root > %s", req.Path)
        return nil
    })

    if err := fseventserver.ListenAndServe("/mnt/filedispatch/downloads/", nil); err != nil {
        log.Fatal(err)
    }
}
