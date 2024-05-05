package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"example.com/fseventserver"
)



func main() {
    fseventserver.HandleFunc("~/Downloads/*.mp4", Mp3Converter)
    fseventserver.HandleFunc("~/Downloads/*.md", PDFConverter)
    fseventserver.HandleFunc("~/Downloads/*.json", JSONPretty)

    if err := fseventserver.ListenAndServe("~/Downloads", nil); err != nil {
        log.Fatal(err)
    }
}

func Mp3Converter(ctx context.Context) error {
    value := ctx.Value("request")
    req := value.(*fseventserver.Request)
    if req.Mimetype.Extension() != ".mp4" {
        return errors.New("")
    }
    name := strings.TrimSuffix(req.Path, req.Mimetype.Extension()) 
    name = fmt.Sprintf("%s.mp3", name) 
    cmd := exec.Command("ffmpeg", "-i", req.Path, "-vn", name)
    return cmd.Run()
}

func PDFConverter(ctx context.Context) error {
    value := ctx.Value("request")
    req := value.(*fseventserver.Request)
    fmt.Println(req.Mimetype.Extension())
    if req.Mimetype.Extension() != ".md" {
        return errors.New("")
    }
    name := strings.TrimSuffix(req.Path, req.Mimetype.Extension()) 
    name = filepath.Base(fmt.Sprintf("%s.pdf", name))
    cmd := exec.Command("pandoc",  req.Path, "-o", filepath.Join("/tmp", name))
    return cmd.Run()
}

func JSONPretty(ctx context.Context) error {
    var err error
    value := ctx.Value("request")
    req := value.(*fseventserver.Request)
    if req.Mimetype.Extension() != ".json" {
        return errors.New("")
    }
    name := strings.TrimSuffix(req.Path, req.Mimetype.Extension()) 
    name = filepath.Base(fmt.Sprintf("%s.pretty%s", name, req.Mimetype.Extension()))
    output := filepath.Join("/tmp", name) 
    cmd := exec.Command("jq", ".", req.Path)

    f, err := os.Create(output)
    if err != nil {
        return fmt.Errorf("cannot run command %s (error: %s)", cmd, err) 
    }
    defer f.Close()

	cmd.Stderr = os.Stderr
	cmd.Stdout = f

    if err = cmd.Run(); err != nil {
        return fmt.Errorf("cannot run command %s (error: %s)", cmd, err) 
    }
    return nil
}

func MailSender(ctx context.Context) error {
    value := ctx.Value("request")
    req := value.(*fseventserver.Request)
    if req.Mimetype.Extension() != ".xlsx" {
        return errors.New("")
    }
    return nil
}
