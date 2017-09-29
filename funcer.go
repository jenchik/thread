package thread

import (
	"sync"
)

type Canceler func(<-chan func())

type Funcer struct {
	c     Canceler
	queue chan func()

	semCloser SemaphoreInt
	stop      chan struct{}
	wg        sync.WaitGroup
}

func NewFuncer(c Canceler, sizes ...int) *Funcer {
	sizeQueue := 1
	if len(sizes) > 0 && sizes[0] > 0 {
		sizeQueue = sizes[0]
	}

	f := &Funcer{
		c:     c,
		queue: make(chan func(), sizeQueue),
		stop:  make(chan struct{}),
	}
	go f.worker()

	return f
}

func (f *Funcer) Queue() int {
	return len(f.queue)
}

func (f *Funcer) Close() error {
	if f.semCloser.Acquire() {
		close(f.stop)
	}

	return nil
}

func (f *Funcer) IsClosed() bool {
	return f.semCloser.IsLocked()
}

func (f *Funcer) Wait() {
	f.wg.Wait()
}

func (f *Funcer) StopC() <-chan struct{} {
	return f.stop
}

func (f *Funcer) Put(fn func()) bool {
	if !f.semCloser.IsLocked() {
		f.queue <- fn
		return true
	}

	return false
}

func (f *Funcer) worker() {
	f.wg.Add(1)
	defer func() {
		close(f.queue)
		if f.c != nil {
			f.c(f.queue)
		}
		f.wg.Done()
	}()

	for {
		select {
		case <-f.stop:
			return
		case fn := <-f.queue:
			fn()
		}
	}
}
