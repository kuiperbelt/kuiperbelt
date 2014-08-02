package kuiperbelt

type BroadcastNotifier struct {
	conn Connector
}

func NewBroadcastNotifier(conn Connector) *BroadcastNotifier {

	return &BroadcastNotifier{
		conn: conn,
	}
}

func (bn *BroadcastNotifier) ReceiveMessage(message []byte) {
	go bn.runBroadcaster(message)
}

func (bn *BroadcastNotifier) runBroadcaster(message []byte) {
	conChans := bn.conn.GetChans()

	for _, connChan := range conChans {
		connChan <- message
	}
}
