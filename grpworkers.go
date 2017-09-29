package thread

import (
	"sync"
)

type (
	funcer struct {
		c     Canceler
		size  int
		reply chan *Funcer
	}

	worker struct {
		c     func()
		h     WorkerHandler
		reply chan *Worker
	}

	closer interface {
		Close() error
	}

	GroupWorkers struct {
		w  *Worker
		wg sync.WaitGroup
		n  int

		addFuncer chan funcer
		addWorker chan worker
		del       chan int
		workers   map[int]closer
	}
)

func handlers(g *GroupWorkers) (WorkerHandler, func()) {
	g.wg.Add(1)
	return func(master *Worker) {
			// главный воркер группы
			var n int
			var f funcer
			var w worker
			stop := master.StopC()
			for {
				select {
				case <-stop:
					// группу закрыли
					return
				case n = <-g.del:
					// пришёл номер воркера в мапе на удаление
					if _, found := g.workers[n]; found {
						delete(g.workers, n)
					}
				case f = <-g.addFuncer:
					// запрос на добавление нового Funcer'а в группу
					f.reply <- g.createFuncer(f)
				case w = <-g.addWorker:
					// запрос на добавление нового Worker'а в группу
					w.reply <- g.createWorker(w)
				}
			}
		}, func() {
			// обработчик закрытия главного воркера группы
			defer g.wg.Done()
			loop := true
			for loop {
				// очищаем хвосты, в случае добавления воркеров в момент закрытия группы
				select {
				case f := <-g.addFuncer:
					f.reply <- nil
				case w := <-g.addWorker:
					w.reply <- nil
				case <-g.del:
				default:
					loop = false
				}
			}
			// закрываем все воркеры, входящие в группу
			for n := 1; n <= g.n; n++ {
				if c, found := g.workers[n]; found {
					c.Close()
				}
			}
			g.workers = nil
		}
}

func NewGroupWorkers() *GroupWorkers {
	g := &GroupWorkers{
		addFuncer: make(chan funcer),
		addWorker: make(chan worker),
		del:       make(chan int, 1),
		workers:   make(map[int]closer, 4),
	}
	g.w = NewWorker(handlers(g))

	return g
}

func (g *GroupWorkers) createFuncer(f funcer) (ptr *Funcer) {
	g.n++
	n := g.n

	g.wg.Add(1)
	c := func(q <-chan func()) {
		// обработчик закрытия воркера
		defer g.wg.Done()
		if !g.w.IsClosed() {
			// отправляем номер воркера на удаление из списка группы
			g.del <- n
		}
		if f.c != nil {
			// если есть пользовательский обработчик, то его выполняем
			// и передаём ему хвост
			f.c(q)
		}
	}

	ptr = NewFuncer(c, f.size)
	g.workers[n] = ptr

	return
}

func (g *GroupWorkers) createWorker(w worker) (ptr *Worker) {
	g.n++
	n := g.n

	g.wg.Add(1)
	c := func() {
		// обработчик закрытия воркера
		defer g.wg.Done()
		if !g.w.IsClosed() {
			// отправляем номер воркера на удаление из списка группы
			g.del <- n
		}
		if w.c != nil {
			// если есть пользовательский обработчик, то его выполняем
			w.c()
		}
	}

	ptr = NewWorker(w.h, c)
	g.workers[n] = ptr

	return
}

func (g *GroupWorkers) Count() int {
	return len(g.workers)
}

// AddAsFuncer может вернуть <nil>, в случае когда группа уже закрыта (закрывается)
func (g *GroupWorkers) AddAsFuncer(c Canceler, sizeQueue int) *Funcer {
	f := funcer{
		c:     c,
		size:  sizeQueue,
		reply: make(chan *Funcer),
	}

	if !g.w.IsClosed() {
		g.addFuncer <- f
		return <-f.reply
	}

	return nil
}

// AddAsWorker может вернуть <nil>, в случае когда группа уже закрыта (закрывается)
func (g *GroupWorkers) AddAsWorker(h WorkerHandler, c func()) *Worker {
	w := worker{
		c:     c,
		h:     h,
		reply: make(chan *Worker),
	}

	if !g.w.IsClosed() {
		g.addWorker <- w
		return <-w.reply
	}

	return nil
}

func (g *GroupWorkers) StopC() <-chan struct{} {
	return g.w.StopC()
}

func (g *GroupWorkers) Close() error {
	return g.w.Close()
}

func (g *GroupWorkers) IsClosed() bool {
	return g.w.IsClosed()
}

func (g *GroupWorkers) Wait() {
	g.wg.Wait()
}
