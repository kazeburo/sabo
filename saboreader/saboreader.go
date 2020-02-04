package saboreader

// This code referred shapeio
// https://github.com/fujiwara/shapeio

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

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
	mu      *sync.RWMutex
	lfh     *os.File
	lfn     string
	wd      string
	bw      uint64
}

// NewReaderWithContext returns a reader that implements io.Reader with rate limiting.
func NewReaderWithContext(ctx context.Context, r io.Reader, workDir string, bw uint64) (*Reader, error) {
	_, err := ioutil.ReadDir(workDir)
	if err != nil {
		return nil, fmt.Errorf("Cannot open workdir: %v", err)
	}
	lockfileName := fmt.Sprintf("sabo_%d_%d.lock", bw, os.Getpid())
	lockfile := filepath.Join(workDir, lockfileName)
	tmpfile := filepath.Join(workDir, "_"+lockfileName)
	file, err := os.OpenFile(tmpfile, syscall.O_RDWR|syscall.O_CREAT, 0600)
	if err != nil {
		return nil, fmt.Errorf("Cannot create lockfile in workdir: %v", err)
	}

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return nil, fmt.Errorf("Cannot lock lockfile in workdir: %v", err)
	}
	err = os.Rename(tmpfile, lockfile)
	if err != nil {
		return nil, fmt.Errorf("Cannot rename lockfile: %v", err)
	}
	return &Reader{
		r:   r,
		ctx: ctx,
		mu:  new(sync.RWMutex),
		lfh: file,
		lfn: lockfile,
		wd:  workDir,
		bw:  bw,
	}, nil
}

// CleanUp clean up lockfile
func (s *Reader) CleanUp() {
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

// RefreshLimiter refresh limiter
func (s *Reader) RefreshLimiter(ctx context.Context) error {
	files, err := ioutil.ReadDir(s.wd)
	if err != nil {
		return fmt.Errorf("Cannot open workdir: %v", err)
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

// RunRefresh refresh limiter regularly
func (s *Reader) RunRefresh(ctx context.Context) {
	ticker := time.NewTicker(refreshInterval * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case _ = <-ticker.C:
			err := s.RefreshLimiter(ctx)
			if err != nil {
				log.Printf("Regularly refresh limiter failed:%v", err)
			}
		}
	}
}
