package main

import (
	"iter"
	"sync"
	"tccgo/mes"
)

type taskGroup struct {
	wg sync.WaitGroup
}

func (tg *taskGroup) start(f func()) {
	tg.wg.Add(1)
	go func() {
		defer tg.wg.Done()
		f()
	}()
}

func (tg *taskGroup) wait() {
	tg.wg.Wait()
}

func discardMessages(it iter.Seq[mes.Message]) {
	for range it {
	}
}

func test1(address string) {
}
