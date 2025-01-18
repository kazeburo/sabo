package saboreader

// This code referred shapeio
// https://github.com/fujiwara/shapeio

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

const (
	refreshInterval = 1
)

// Reader reader with limiter
type Reader struct {
	r       io.Reader
	limiter *rate.Limiter
	ctx     context.Context
	ticker  *time.Ticker
	mu      *sync.RWMutex
	lfh     *os.File
	lfn     string
	wd      string
	bw      uint64
}

// NewReaderWithContext returns a reader that implements io.Reader with rate limiting.
func NewReaderWithContext(ctx context.Context, r io.Reader, workDir string, bw uint64, identifier int) (*Reader, error) {
	_, err := os.ReadDir(workDir)
	if err != nil {
		return nil, errors.Wrap(err, "could not open workdir")
	}
	lockfileName := fmt.Sprintf("sabo_%d_%d.lock", bw, identifier)
	lockfile := filepath.Join(workDir, lockfileName)
	tmpfile := filepath.Join(workDir, "_"+lockfileName)
	file, err := os.OpenFile(tmpfile, syscall.O_RDWR|syscall.O_CREAT, 0600)
	if err != nil {
		return nil, errors.Wrap(err, "could not create lockfile in workdir")
	}

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return nil, errors.Wrap(err, "could not lock lockfile in workdir")
	}
	err = os.Rename(tmpfile, lockfile)
	if err != nil {
		return nil, errors.Wrap(err, "could not rename lockfile")
	}
	ticker := time.NewTicker(refreshInterval * time.Second)
	reader := &Reader{
		r:      r,
		ctx:    ctx,
		ticker: ticker,
		mu:     new(sync.RWMutex),
		lfh:    file,
		lfn:    lockfile,
		wd:     workDir,
		bw:     bw,
	}
	err = reader.refreshLimiter()
	if err != nil {
		reader.CleanUp()
		return nil, err
	}
	go reader.runRefresh()

	return reader, err
}

// CleanUp clean up lockfile
func (s *Reader) CleanUp() {
	s.ticker.Stop()
	defer os.Remove(s.lfn)
	s.lfh.Close()
}

func (s *Reader) getRateLimit() *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	limiter := s.limiter
	return limiter
}

// Read reads bytes into p.
func (s *Reader) Read(p []byte) (int, error) {
	limiter := s.getRateLimit()
	if limiter == nil {
		return s.r.Read(p)
	}
	n, err := s.r.Read(p)
	if err != nil {
		return n, err
	}
	// log.Printf("read: %d", n)
	if err := limiter.WaitN(s.ctx, n); err != nil {
		return n, err
	}
	return n, nil
}

func (s *Reader) refreshLimiter() error {
	files, err := os.ReadDir(s.wd)
	if err != nil {
		return errors.Wrap(err, "Cannot open workdir")
	}
	locked := uint64(0)
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if m, _ := regexp.MatchString(fmt.Sprintf("^sabo_%d_", s.bw), file.Name()); !m {
			continue
		}

		lfile, err := os.OpenFile(filepath.Join(s.wd, file.Name()), syscall.O_RDONLY, 0600)
		if err != nil {
			continue
		}
		err = syscall.Flock(int(lfile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		// fmt.Fprintf(os.Stderr, "= %s => %v\n", lfile.Name(), err)
		if err != nil {
			locked = locked + 1
			lfile.Close()
		} else {
			lfile.Close()
			os.Remove(filepath.Join(s.wd, file.Name()))
		}

	}
	bytesPerSec := float64(s.bw)
	if locked > 0 {
		bytesPerSec = float64(s.bw / locked)
	}
	l := rate.Limit(bytesPerSec)
	b := int(math.Ceil(bytesPerSec))

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.limiter != nil && l == s.limiter.Limit() {
		return nil
	}

	// fmt.Fprintf(os.Stderr, "new limit %f\n", bytesPerSec)
	limiter := rate.NewLimiter(l, b)
	limiter.AllowN(time.Now(), b)
	s.limiter = limiter
	return nil
}

func (s *Reader) runRefresh() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
			err := s.refreshLimiter()
			if err != nil {
				log.Printf("Regularly refresh limiter failed:%v", err)
			}
		}
	}
}
