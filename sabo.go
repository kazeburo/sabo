package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	humanize "github.com/dustin/go-humanize"
	flags "github.com/jessevdk/go-flags"
	"github.com/kazeburo/sabo/saboreader"
)

// Version set in compile
var Version string

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
		fmt.Printf(`motarei %s
Compiler: %s %s
`,
			Version,
			runtime.Compiler,
			runtime.Version())
		return

	}

	bw, err := humanize.ParseBytes(opts.MaxBandWidth)
	if err != nil {
		fmt.Println("Cannot parse -max-bandwidth", err)
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
	reader.RunRefresh(ctx)

	io.Copy(os.Stdout, reader)
}
