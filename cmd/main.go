package main

import (
	"context"
	"fmt"
	"log"
    "log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	fsevt "example.com/fseventserver"
	"github.com/fsnotify/fsnotify"
)


func main() {
    fsevt.Handle("~/Downloads/*.mp4", fsevt.Use(fsevt.HandlerFunc(Mp3Converter), fsevt.LoggingMiddleware))
    log.Fatal(fsevt.ListenAndServe("~/Downloads", nil))
}

func Mp3Converter(ctx context.Context) error {
    logger := slog.Default()

    value := ctx.Value("request")

    req := value.(*fsevt.Request)
    
    if !req.Action.Has(fsnotify.Create) || !req.Mimetype.Is("video/mp4") {
        logger.Warn(fmt.Sprintf("%s: %+v", fsevt.ErrHandlingRequest.Error(), req))
        return nil
    }

    name := strings.TrimSuffix(req.Path, req.Mimetype.Extension()) 
    name = filepath.Base(name)
    name = fmt.Sprintf("%s.mp3", name) 
    name = filepath.Join("/tmp", name)
    cmd := exec.Command("ffmpeg", "-i", req.Path, "-vn", name)

    logger.Info(fmt.Sprintf("converting the file %s", req.Path))
    return cmd.Run()
}
