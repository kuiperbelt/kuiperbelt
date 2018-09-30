package kuiperbelt

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type receivedMessage struct {
	Message     io.Reader
	ContentType string
}

func newReceivedMessage(msgType int, r io.Reader) receivedMessage {
	var contentType string
	switch msgType {
	case websocket.TextMessage:
		contentType = "text/plain"
	case websocket.BinaryMessage:
		contentType = "application/octet-stream"
	}
	return receivedMessage{
		Message:     r,
		ContentType: contentType,
	}
}

// Receiver is proxy message from a client
type Receiver interface {
	Receive(context.Context, receivedMessage) error
}

// NewReceiverCallback is generate Receiver that proxy message to callback.Receive
func newReceiverCallback(client *http.Client, callback *url.URL) Receiver {
	return &receiverCallback{
		client:   client,
		callback: callback,
	}
}

type receiverCallback struct {
	client   *http.Client
	callback *url.URL
}

func (r *receiverCallback) Receive(ctx context.Context, m receivedMessage) error {
	req, err := http.NewRequest(
		http.MethodPost,
		r.callback.String(),
		m.Message,
	)
	if err != nil {
		return errors.Wrap(err, "cannot create receive callback request")
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", m.ContentType)

	resp, err := r.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed post receive callback request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := errCallbackResponseNotOK(resp.StatusCode)
		return errors.Wrap(err, "unsuccessful post receive callback request")
	}

	return nil
}

type discardReceiver struct {
	pool sync.Pool
}

func newDiscardReceiver() Receiver {
	return &discardReceiver{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, ioBufferSize)
			},
		},
	}
}

func (r *discardReceiver) Receive(ctx context.Context, m receivedMessage) error {
	buf := r.pool.Get().([]byte)
	_, err := io.CopyBuffer(ioutil.Discard, m.Message, buf)
	if err != nil {
		return errors.Wrap(err, "cannot read message on discard")
	}
	r.pool.Put(buf)
	return err
}
