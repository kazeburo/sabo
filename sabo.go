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
var version string

const minimumBandwidh = 32 * 1000

type cmdOpts struct {
	MaxBandWidth string `long:"max-bandwidth" description:"max bandwidth (Bytes/sec)" required:"true"`
	WorkDir      string `long:"work-dir" description:"directory for control bandwidth" required:"true"`
	Version      bool   `short:"v" long:"version" description:"Show version"`
}

func printVersion() {
	fmt.Printf(`%s %s
Compiler: %s %s
`,
		os.Args[0],
		version,
		runtime.Compiler,
		runtime.Version())
}

func _main() int {
	opts := cmdOpts{}
	psr := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	_, err := psr.Parse()
	if opts.Version {
		printVersion()
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	bw, err := humanize.ParseBytes(opts.MaxBandWidth)
	if err != nil {
		log.Printf("Cannot parse -max-bandwidth: %v", err)
		return 1
	}
	if bw < minimumBandwidh {
		log.Printf("max-bandwidth > 32K required: %d", bw)
		return 1
	}

	ctx := context.Background()
	reader, err := saboreader.NewReaderWithContext(ctx, os.Stdin, filepath.Clean(opts.WorkDir), bw, os.Getpid())
	if err != nil {
		log.Fatalf("Cannot create reader:%s", err)
	}
	defer reader.CleanUp()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sc
		os.Stdin.Close()
	}()
	buf := make([]byte, minimumBandwidh)
	_, err = io.CopyBuffer(os.Stdout, reader, buf)
	if err != nil {
		log.Printf("%v", err)
	}
	return 0
}

func main() {
	os.Exit(_main())
}
