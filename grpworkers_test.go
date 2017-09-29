package thread_test

import (
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/jenchik/thread"
	"github.com/stretchr/testify/assert"
)

type (
	chanCloser interface {
		StopC() <-chan struct{}
	}
)

func init() {
	numCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPUs)
}

func isClosed(c chanCloser) bool {
	select {
	case <-c.StopC():
		return true
	default:
	}
	return false
}

func closeGroup(g *GroupWorkers) <-chan struct{} {
	closed := make(chan struct{})
	go func() {
		g.Close()
		g.Wait()
		close(closed)
	}()

	return closed
}

func TestNewGroup(t *testing.T) {
	g := NewGroupWorkers()
	assert.Equal(t, 0, g.Count(), "Group must be empty")

	assert.Equal(t, false, isClosed(g), "Group should not be closed")
	g.Close()
	assert.Equal(t, true, isClosed(g), "Group should be closed")
}

func TestWaitEmptyGroup(t *testing.T) {
	g := NewGroupWorkers()

	done := make(chan struct{})
	go func() {
		g.Wait()
		close(done)
	}()

	select {
	case <-time.After(time.Millisecond * 10):
	case <-done:
		assert.Fail(t, "Empty group does not wait")
	}
}

func TestGroupAddWorker(t *testing.T) {
	g := NewGroupWorkers()

	done := make(chan struct{})
	w := g.AddAsWorker(nil, func() {
		close(done)
	})

	assert.Equal(t, 1, g.Count(), "Count workers must be equal to 1")
	assert.Equal(t, false, w.IsClosed(), "Worker should not be closed")

	w.Close()
	assert.Equal(t, true, w.IsClosed(), "Worker should be closed")

	<-done
	time.Sleep(time.Millisecond) // ждём пока пул воркеров обновит свой список
	assert.Equal(t, 0, g.Count(), "Count workers must be equal to 0")
	assert.Equal(t, false, isClosed(g), "Group should not be closed")
}

func TestGroupAddWorkers(t *testing.T) {
	g := NewGroupWorkers()

	done1 := make(chan struct{})
	w1 := g.AddAsWorker(nil, func() {
		close(done1)
	})

	g.AddAsWorker(nil, nil)

	done2 := make(chan struct{})
	w2 := g.AddAsWorker(func(tw *Worker) {
		time.Sleep(time.Millisecond * 10)
		tw.Close()
	}, func() {
		close(done2)
	})

	assert.Equal(t, 3, g.Count(), "Count workers must be equal to 3")
	assert.Equal(t, false, w1.IsClosed(), "Worker should not be closed")
	assert.Equal(t, false, w2.IsClosed(), "Worker should not be closed")

	<-done2
	assert.Equal(t, false, w1.IsClosed(), "Worker should not be closed")
	assert.Equal(t, true, w2.IsClosed(), "Worker should be closed")

	time.Sleep(time.Millisecond) // ждём пока пул воркеров обновит свой список
	assert.Equal(t, 2, g.Count(), "Count workers must be equal to 2")

	w1.Close()
	assert.Equal(t, true, w1.IsClosed(), "Worker should be closed")

	<-done1
	time.Sleep(time.Millisecond) // ждём пока пул воркеров обновит свой список
	assert.Equal(t, false, isClosed(g), "Group should not be closed")
	assert.Equal(t, 1, g.Count(), "Count workers must be equal to 1")
}

func TestGroupCloseWorker(t *testing.T) {
	g := NewGroupWorkers()

	g.AddAsWorker(nil, nil)

	assert.Equal(t, 1, g.Count(), "Count workers must be equal to 1")

	called := false
	w := g.AddAsWorker(nil, func() {
		called = true
	})

	assert.Equal(t, 2, g.Count(), "Count workers must be equal to 2")
	assert.Equal(t, false, w.IsClosed(), "Worker should not be closed")
	assert.Equal(t, false, called)

	select {
	case <-time.After(time.Millisecond * 10):
		assert.Fail(t, "Group is in deadlock")
	case <-closeGroup(g):
	}

	assert.Equal(t, true, called)
	assert.Equal(t, 0, g.Count(), "Count workers must be equal to 0")
	assert.Equal(t, true, isClosed(g), "Group should be closed")
}

func TestGroupWithMoreWorkers(t *testing.T) {
	g := NewGroupWorkers()

	wg := sync.WaitGroup{}
	start := make(chan struct{})
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			defer wg.Done()
			<-start
			g.AddAsWorker(nil, nil)
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, 1000, g.Count(), "Count workers must be equal to 1000")

	select {
	case <-time.After(time.Millisecond * 10):
		assert.Fail(t, "Group is in deadlock")
	case <-closeGroup(g):
	}

	assert.Equal(t, 0, g.Count(), "Count workers must be equal to 0")
	assert.Equal(t, true, isClosed(g), "Group should be closed")
}

func TestGroupConcurently(t *testing.T) {
	g := NewGroupWorkers()

	wg := sync.WaitGroup{}
	start := make(chan struct{})
	wg.Add(20000)
	for i := 0; i < 10000; i++ {
		go func() {
			r := rand.Int63n(1000)
			<-start
			g.AddAsWorker(func(tw *Worker) {
				tw.Close()
				time.Sleep(time.Microsecond * time.Duration(r))
				g.AddAsWorker(nil, nil)
			}, func() {
				wg.Done()
			})
		}()
		go func() {
			<-start
			g.AddAsWorker(func(tw *Worker) {
				tw.Close()
			}, func() {
				defer wg.Done()
				g.AddAsWorker(nil, nil)
			})
		}()
	}
	close(start)
	wg.Wait()

	time.Sleep(time.Millisecond) // ждём пока пул воркеров обновит свой список
	assert.Equal(t, 20000, g.Count(), "Count workers must be equal to 20000")

	select {
	case <-time.After(time.Millisecond * 20):
		assert.Fail(t, "Group is in deadlock")
	case <-closeGroup(g):
	}

	assert.Equal(t, 0, g.Count(), "Count workers must be equal to 0")
	assert.Equal(t, true, isClosed(g), "Group should be closed")
}

func TestGroupClose(t *testing.T) {
	g := NewGroupWorkers()

	var counter, check uint32
	wg := sync.WaitGroup{}
	start := make(chan struct{})
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			defer wg.Done()
			<-start
			w := g.AddAsWorker(nil, func() {
				atomic.AddUint32(&counter, 1)
			})
			if w != nil {
				atomic.AddUint32(&check, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, 1000, g.Count(), "Count workers must be equal to 1000")

	start2 := make(chan struct{})
	for i := 0; i < 10000; i++ {
		go func() {
			<-start2
			w := g.AddAsWorker(nil, func() {
				atomic.AddUint32(&counter, 1)
			})
			if w != nil {
				atomic.AddUint32(&check, 1)
			}
		}()
	}

	closed := make(chan struct{})
	go func() {
		close(start2)
		g.Close()
		g.Wait()
		close(closed)
	}()

	select {
	case <-time.After(time.Millisecond * 20):
		assert.Fail(t, "Group is in deadlock")
	case <-closed:
	}

	assert.Equal(t, 0, g.Count(), "Count workers must be equal to 0")
	assert.Equal(t, true, isClosed(g), "Group should be closed")
	assert.Equal(t, check, counter, "Count workers must be equal")
}

func TestGroupClose2(t *testing.T) {
	g := NewGroupWorkers()

	wg := sync.WaitGroup{}
	start := make(chan struct{})
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			defer wg.Done()
			r1 := rand.Int63n(100)
			r2 := rand.Int63n(100)
			<-start
			g.AddAsWorker(func(*Worker) {
				for {
					select {
					case <-g.StopC():
						return
					case <-time.After(time.Microsecond * time.Duration(r1)):
					}
				}
			}, nil)
			g.AddAsWorker(func(tw *Worker) {
				select {
				case <-g.StopC():
					select {
					case <-tw.StopC():
					default:
						tw.Close()
					}
					return
				case <-time.After(time.Microsecond * time.Duration(r2)):
				}
			}, nil)
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, 2000, g.Count(), "Count workers must be equal to 2000")

	select {
	case <-time.After(time.Millisecond * 100):
		assert.Fail(t, "Group is in deadlock")
	case <-closeGroup(g):
	}

	assert.Equal(t, 0, g.Count(), "Count workers must be equal to 0")
	assert.Equal(t, true, isClosed(g), "Group should be closed")
}
