package worker

import "sync"

type countingWaitGroup struct {
	sync.WaitGroup

	countMutex sync.Mutex
	count      int
}

func (cwg *countingWaitGroup) Add(n int) {
	cwg.countMutex.Lock()
	cwg.WaitGroup.Add(n)
	cwg.count += n
	cwg.countMutex.Unlock()
}

func (cwg *countingWaitGroup) Done() {
	cwg.countMutex.Lock()
	cwg.WaitGroup.Done()
	cwg.count--
	cwg.countMutex.Unlock()
}

func (cwg *countingWaitGroup) Count() int {
	cwg.countMutex.Lock()
	count := cwg.count
	cwg.countMutex.Unlock()
	return count
}
