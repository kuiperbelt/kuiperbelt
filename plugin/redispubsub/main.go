package redispubsub

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mackee/kuiperbelt/plugin"

	log "gopkg.in/Sirupsen/logrus.v0"
	"gopkg.in/redis.v3"
)

const (
	RedisChannelTemplate = "kuiperbelt:channel:%s"
)

type Plugin struct {
	keyMap      map[string]struct{}
	locker      *sync.Mutex
	redisClient *redis.Client
	redisPubSub *redis.PubSub
	receiver    chan plugin.ReceivedMessage
}

func NewPlugin() (*Plugin, error) {
	p := new(Plugin)
	p.keyMap = map[string]struct{}{}
	p.locker = new(sync.Mutex)
	p.redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	pubsub, err := p.redisClient.Subscribe("kuiperbelt:global")
	if err != nil {
		return nil, fmt.Errorf("pubsub error: %s", err)
	}
	p.redisPubSub = pubsub
	p.receiver = make(chan plugin.ReceivedMessage)
	go p.pollReceiver()

	return p, nil
}

func (p *Plugin) pollReceiver() {
	for {
		message, err := p.redisPubSub.ReceiveMessage()
		if err != nil {
			log.Fatalf("receive message error: %s", err)
			break
		}
		splitedChannel := strings.Split(message.Channel, ":")
		key := splitedChannel[len(splitedChannel)-1]
		resp := plugin.ReceivedMessage{
			Keys:    []string{key},
			Message: []byte(message.Payload),
		}
		p.receiver <- resp
	}
}

func (p *Plugin) Relay(args plugin.RelayArgs, resp *plugin.RelayResp) error {
	errKeys := make([]string, 0, len(args.Keys))
	for _, key := range args.Keys {
		channel := fmt.Sprintf(RedisChannelTemplate, key)
		intCmd := p.redisClient.Publish(channel, string(args.Message))
		if intCmd.Val() == 0 {
			errKeys = append(errKeys, key)
		}
	}

	if len(errKeys) == 0 {
		return nil
	}
	resp.NotExistsKeys = errKeys

	return nil
}

func (p *Plugin) registeredKeys() []string {
	var keys []string
	for key, _ := range p.keyMap {
		keys = append(keys, key)
	}
	return keys
}

func (p *Plugin) RegisterKeys(args plugin.RelayArgs, resp *plugin.RelayResp) error {
	p.locker.Lock()
	defer p.locker.Unlock()
	additionalChannels := make([]string, 0, len(args.Keys))
	for _, key := range args.Keys {
		p.keyMap[key] = struct{}{}
		additionalChannel := fmt.Sprintf(RedisChannelTemplate, key)
		additionalChannels = append(additionalChannels, additionalChannel)
	}
	p.redisPubSub.Subscribe(additionalChannels...)
	resp.RegisteredKeys = p.registeredKeys()
	return nil
}

func (p *Plugin) RemoveKeys(args plugin.RelayArgs, resp *plugin.RelayResp) error {
	p.locker.Lock()
	defer p.locker.Unlock()
	deletionChannels := make([]string, 0, len(args.Keys))
	for _, key := range args.Keys {
		delete(p.keyMap, key)
		deletionChannel := fmt.Sprintf(RedisChannelTemplate, key)
		deletionChannels = append(deletionChannels, deletionChannel)
	}
	p.redisPubSub.Unsubscribe(deletionChannels...)
	resp.RegisteredKeys = p.registeredKeys()
	return nil
}

func (p *Plugin) ReceiveMessage(args plugin.RelayArgs, resp *plugin.ReceivedMessage) error {
	message := <-p.receiver
	resp.Keys = message.Keys
	resp.Message = message.Message
	return nil
}
