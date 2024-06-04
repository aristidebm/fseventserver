package main

import (
	"context"
	"fmt"
	"log"
    "log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"example.com/fseventserver"
	"github.com/fsnotify/fsnotify"
)


func main() {
    fseventserver.HandleFunc("~/Downloads/*.mp4", Mp3Converter)
    log.Fatal(fseventserver.ListenAndServe("~/Downloads", nil))
}

func Mp3Converter(ctx context.Context) error {
    logger := slog.Default()

    value := ctx.Value("request")

    req := value.(*fseventserver.Request)
    
    logger.Info(fmt.Sprintf("receive the request %+v", req))
    
    if !req.Action.Has(fsnotify.Create) || !req.Mimetype.Is("video/mp4") {
        logger.Warn(fmt.Sprintf("%s: %+v", fseventserver.ErrHandlingRequest.Error(), req))
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
