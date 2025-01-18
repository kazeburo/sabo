package saboreader

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
)

var rates = []string{
	"500K", // 500KB/sec
	"1M",   // 1MB/sec
	"10M",  // 10MB/sec
	"50M",  // 50MB/sec
}

var srcSizes = []int{
	64 * 1000,
	256 * 1000,
	1024 * 1000,
}

func testSimpleReader(ctx context.Context, t *testing.T, dir string, src *bytes.Reader, limit string, identifier int, para int) {
	bw, _ := humanize.ParseBytes(limit)
	reader, err := NewReaderWithContext(
		ctx,
		src,
		dir,
		bw,
		identifier,
	)
	if err != nil {
		t.Error(err)
		return
	}
	defer reader.CleanUp()
	start := time.Now()
	n, err := io.Copy(io.Discard, reader)
	if err != nil {
		t.Error(err)
		return
	}
	elapsed := time.Since(start)
	realRate := float64(n) / elapsed.Seconds()
	expRate := float64(bw) / float64(para)
	t.Logf(
		"read:%s, elapsed:%s, real:%s/sec, limit:%s/sec. (%0.3f%%)",
		humanize.Bytes(uint64(n)),
		elapsed,
		humanize.Bytes(uint64(realRate)),
		humanize.Bytes(uint64(expRate)),
		realRate/expRate*100,
	)
	if realRate*0.7 > expRate {
		t.Errorf("limit %0.3f but real rate %0.3f", expRate, realRate)
		return
	}

}

func TestSimpleRead(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	for _, srcSize := range srcSizes {
		for _, limit := range rates {
			src := bytes.NewReader(bytes.Repeat([]byte{0}, srcSize))
			testSimpleReader(ctx, t, dir, src, limit, 1, 1)
		}
	}
}

func TestMultiRead(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	for _, limit := range rates {
		var wg sync.WaitGroup
		srcs := []*bytes.Reader{
			bytes.NewReader(bytes.Repeat([]byte{0}, 400*1000)),
			bytes.NewReader(bytes.Repeat([]byte{0}, 400*1000)),
			//bytes.NewReader(bytes.Repeat([]byte{0}, 400*1000)),
			//bytes.NewReader(bytes.Repeat([]byte{0}, 400*1000)),
		}
		for j, src := range srcs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				testSimpleReader(ctx, t, dir, src, limit, j, len(srcs))
			}()
		}
		wg.Wait()
	}
}
