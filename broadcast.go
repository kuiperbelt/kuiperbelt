package kuiperbelt

import (
	"log"
	"sync"
)

type BroadcastNotifier struct {
	notifyingChan chan struct{}
	doneChan      chan struct{}
	isNotifying   bool
	message       []byte
	messageLocker *sync.RWMutex
	cond          *sync.Cond
	wg            *sync.WaitGroup
	locker        *sync.Mutex
	count         int
	countLocker   *sync.RWMutex
}

func NewBroadcastNotifier() *BroadcastNotifier {
	locker := new(sync.Mutex)
	cond := sync.NewCond(locker)
	wg := new(sync.WaitGroup)

	return &BroadcastNotifier{
		notifyingChan: make(chan struct{}),
		doneChan:      make(chan struct{}),
		isNotifying:   false,
		message:       make([]byte, 0),
		messageLocker: new(sync.RWMutex),
		locker:        locker,
		cond:          cond,
		wg:            wg,
		count:         0,
		countLocker:   new(sync.RWMutex),
	}
}
func (bn *BroadcastNotifier) ReceiveMessage(message []byte) {
	bn.writeMessage(message)
	go bn.runBroadcaster()
}

func (bn *BroadcastNotifier) runNotifying(uuid string) bool {
	defer bn.wg.Done()
	<-bn.notifyingChan
	connectorChan, existsConnector := connector.GetChan(uuid)
	if !existsConnector {
		return false
	}
	message := bn.readMessage()
	connectorChan <- message
	return true
}

func (bn *BroadcastNotifier) NotifyLoop(uuid string) {
	bn.addCount()
	for {
		if !bn.runNotifying(uuid) {
			break
		}
		<-bn.doneChan
	}
	bn.delCount()
}

func (bn *BroadcastNotifier) runBroadcaster() bool {
	bn.locker.Lock()
	defer bn.locker.Unlock()
	bn.wg.Add(bn.countChans())
	close(bn.notifyingChan)
	log.Printf("sending of broadcast")
	bn.wg.Wait()
	close(bn.doneChan)
	bn.notifyingChan = make(chan struct{})
	bn.doneChan = make(chan struct{})
	log.Printf("end of broadcast")
	return true
}

func (bn *BroadcastNotifier) readMessage() []byte {
	bn.messageLocker.RLock()
	defer bn.messageLocker.RUnlock()
	return bn.message
}

func (bn *BroadcastNotifier) writeMessage(message []byte) {
	bn.messageLocker.Lock()
	defer bn.messageLocker.Unlock()
	bn.message = message
}

func (bn *BroadcastNotifier) countChans() int {
	bn.countLocker.RLock()
	defer bn.countLocker.RUnlock()
	return bn.count
}

func (bn *BroadcastNotifier) addCount() {
	bn.countLocker.Lock()
	defer bn.countLocker.Unlock()
	bn.count += 1
}

func (bn *BroadcastNotifier) delCount() {
	bn.countLocker.Lock()
	defer bn.countLocker.Unlock()
	bn.count -= 1
}
