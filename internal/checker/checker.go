package checker

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AlanZeng-Coder/linkwatch/internal/storage"
)

type Checker struct {
	storage        storage.Storage
	interval       time.Duration
	maxConcurrency int
	httpTimeout    time.Duration
	httpClient     *http.Client
	hostMu         sync.Map
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewChecker(s storage.Storage, interval time.Duration, maxConc int, httpTimeout time.Duration) *Checker {
	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 1,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			return nil
		},
	}
	return &Checker{
		storage:        s,
		interval:       interval,
		maxConcurrency: maxConc,
		httpTimeout:    httpTimeout,
		httpClient:     client,
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (c *Checker) Start() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			c.wg.Wait()
			return
		case <-ticker.C:
			c.checkAll()
		}
	}
}

func (c *Checker) Stop() {
	c.cancel()
}

func (c *Checker) checkAll() {
	targets, _, err := c.storage.ListTargets(c.ctx, "", 10000, "")
	if err != nil {
		log.Printf("Error listing targets: %v", err)
		return
	}

	queue := make(chan *storage.Target, len(targets))
	for _, t := range targets {
		queue <- t
	}
	close(queue)

	sem := make(chan struct{}, c.maxConcurrency)
	c.wg.Add(c.maxConcurrency)
	for i := 0; i < c.maxConcurrency; i++ {
		go func() {
			defer c.wg.Done()
			for target := range queue {
				select {
				case sem <- struct{}{}:
					c.checkOne(target)
					<-sem
				case <-c.ctx.Done():
					return
				}
			}
		}()
	}
}

func (c *Checker) checkOne(t *storage.Target) {
	u, _ := url.Parse(t.URL)
	host := u.Host

	muI, _ := c.hostMu.LoadOrStore(host, &sync.Mutex{})
	mu := muI.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	result := &storage.CheckResult{CheckedAt: time.Now().UTC()}
	backoff := 200 * time.Millisecond
	var resp *http.Response
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		start := time.Now()
		resp, err = c.httpClient.Get(t.URL)
		result.LatencyMs = int(time.Since(start).Milliseconds())
		if err != nil {
			if attempt < 2 && isRetryableError(err) {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			result.Error = err.Error()
			break
		}
		defer resp.Body.Close()
		result.StatusCode = resp.StatusCode
		if attempt < 2 && (resp.StatusCode >= 500) {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			break
		}
		break
	}

	if err := c.storage.SaveCheckResult(c.ctx, t.ID, result); err != nil {
		log.Printf("Error saving result: %v", err)
	}
}

func isRetryableError(err error) bool {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host")
}
