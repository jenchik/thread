package thread

import (
	"sync"
)

type WorkerHandler func(*Worker)

type Worker struct {
	c func()
	h WorkerHandler

	semCloser SemaphoreInt
	stop      chan struct{}
	wg        sync.WaitGroup
}

func NewWorker(h WorkerHandler, c func()) *Worker {
	if h == nil {
		h = defaultWorkerHandler
	}
	w := &Worker{
		c:    c,
		h:    h,
		stop: make(chan struct{}),
	}
	go w.worker()

	return w
}

func (w *Worker) Close() error {
	if w.semCloser.Acquire() {
		close(w.stop)
	}

	return nil
}

func (w *Worker) IsClosed() bool {
	return w.semCloser.IsLocked()
}

func (w *Worker) Wait() {
	w.wg.Wait()
}

func (w *Worker) StopC() <-chan struct{} {
	return w.stop
}

func (w *Worker) worker() {
	w.wg.Add(1)
	defer func() {
		if w.c != nil {
			w.c()
		}
		w.wg.Done()
	}()

	for {
		select {
		case <-w.stop:
			return
		default:
			w.h(w)
		}
	}
}

func defaultWorkerHandler(w *Worker) {
	<-w.stop
}
