package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	humanize "github.com/dustin/go-humanize"
	flags "github.com/jessevdk/go-flags"
	"github.com/kazeburo/sabo/saboreader"
)

// Version set in compile
var Version string

const minimumBandwidh = 32 * 1000

type cmdOpts struct {
	MaxBandWidth string `long:"max-bandwidth" description:"max bandwidth (Bytes/sec)" required:"true"`
	WorkDir      string `long:"work-dir" description:"directory for control bandwidth" required:"true"`
	Version      bool   `short:"v" long:"version" description:"Show version"`
}

func main() {
	opts := cmdOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		os.Exit(1)
	}

	if opts.Version {
		fmt.Printf(`sabo %s
Compiler: %s %s
`,
			Version,
			runtime.Compiler,
			runtime.Version())
		return

	}

	bw, err := humanize.ParseBytes(opts.MaxBandWidth)
	if err != nil {
		log.Printf("Cannot parse -max-bandwidth: %v", err)
		os.Exit(1)
	}
	if bw < minimumBandwidh {
		log.Printf("max-bandwidth > 32K required: %d", bw)
		os.Exit(1)
	}

	ctx := context.Background()
	reader, err := saboreader.NewReaderWithContext(ctx, os.Stdin, filepath.Clean(opts.WorkDir), bw)
	if err != nil {
		log.Fatalf("Cannot create reader:%s", err)
	}
	defer reader.CleanUp()
	err = reader.RefreshLimiter(ctx)
	if err != nil {
		log.Fatalf("Cannot create initial bandwidth:%s", err)
	}
	go reader.RunRefresh(ctx)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sc
		os.Stdin.Close()
	}()
	buf := make([]byte, minimumBandwidh)
	io.CopyBuffer(os.Stdout, reader, buf)
}
