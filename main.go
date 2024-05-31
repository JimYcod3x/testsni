package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	certificates := make(map[string]*tls.Certificate)
	loadCertificate := func(certFile, keyFile string) (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	}

	// Load your certificates
	cert, err := loadCertificate("server.crt", "server.key")
	if err != nil {
		log.Fatal(err)
	}
	certificates["mqtt-meter.ddns.net"] = cert

	cert2, err := loadCertificate("server1.crt", "server1.key")
	if err != nil {
		log.Fatal(err)
	}
	certificates["ocpp-iil.ddns.net"] = cert2

	// TLS Config with SNI support
	tlsConfig := &tls.Config{
		GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if cert, ok := certificates[clientHello.ServerName]; ok {
				return cert, nil
			}
			// Fallback to a default certificate if needed
			log.Println("Unsupport SNI", clientHello)
			return cert, nil
		},
	}

	// Optionally, if you want clients to authenticate only with certs issued by your CA,
	// you might want to use something like this:
	// certPool := x509.NewCertPool()
	// _ = certPool.AppendCertsFromPEM(caCertPem)
	// tlsConfig.ClientCAs = certPool
	// tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	server := mqtt.New(&mqtt.Options{
		InlineClient: true, // you must enable inline client to use direct publishing and subscribing.
	})
	_ = server.AddHook(new(auth.AllowHook), nil)

	tcp := listeners.NewTCP(listeners.Config{
		ID:        "t1",
		Address:   "172.31.46.181:8888",
		TLSConfig: tlsConfig,
	})
	err = server.AddListener(tcp)
	if err != nil {
		log.Fatal(err)
	}

	err = server.AddHook(new(ExampleHook), &ExampleHookOptions{
		Server: server,
	})

	if err != nil {
		log.Fatal(err)
	}

	ws := listeners.NewWebsocket(listeners.Config{
		ID:        "ws1",
		Address:   ":1882",
		TLSConfig: tlsConfig,
	})
	err = server.AddListener(ws)
	if err != nil {
		log.Fatal(err)
	}

	stats := listeners.NewHTTPStats(
		listeners.Config{
			ID:        "stats",
			Address:   ":8080",
			TLSConfig: tlsConfig,
		}, server.Info,
	)
	err = server.AddListener(stats)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info("main.go finished")
}

type ExampleHookOptions struct {
	Server *mqtt.Server
}

type ExampleHook struct {
	mqtt.HookBase
	config *ExampleHookOptions
}

func (h *ExampleHook) ID() string {
	return "events-example"
}

func (h *ExampleHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect,
		mqtt.OnDisconnect,
		mqtt.OnSubscribed,
		mqtt.OnUnsubscribed,
		mqtt.OnPublished,
		mqtt.OnPublish,
		mqtt.OnPacketRead,
	}, []byte{b})
}

func (h *ExampleHook) Init(config any) error {
	h.Log.Info("initialised")
	if _, ok := config.(*ExampleHookOptions); !ok && config != nil {
		return mqtt.ErrInvalidConfigType
	}

	h.config = config.(*ExampleHookOptions)
	if h.config.Server == nil {
		return mqtt.ErrInvalidConfigType
	}
	return nil
}
func (h *ExampleHook) OnPacketRead(cl *mqtt.Client, pk packets.Packet) (pkx packets.Packet, err error) {

if string(pk.Connect.Username) == "" {
	pk.Connect.UsernameFlag = false
}
if string(pk.Connect.Password) == "" {
	pk.Connect.PasswordFlag = false
}
return pk, nil
}

// subscribeCallback handles messages for subscribed topics
func (h *ExampleHook) subscribeCallback(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
	if cl.ID != "inline" {
	h.Log.Info("hook subscribed message", "client", cl.ID, "topic", pk.TopicName)
}
}

func (h *ExampleHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	if cl.ID != "inline"{
	h.Log.Info("client connected", "client", cl.ID)

	// Example demonstrating how to subscribe to a topic within the hook.
	h.config.Server.Subscribe("hook/direct/publish", 1, h.subscribeCallback)

	// Example demonstrating how to publish a message within the hook
	err := h.config.Server.Publish("hook/direct/publish", []byte("packet hook message"), false, 0)
	if err != nil {
		h.Log.Error("hook.publish", "error", err)
	}
}
	return nil
}

func (h *ExampleHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	if err != nil {
		h.Log.Info("client disconnected", "client", cl.ID, "expire", expire, "error", err)
	} else {
		h.Log.Info("client disconnected", "client", cl.ID, "expire", expire)
	}

}

func (h *ExampleHook) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	if cl.ID != "inline" {
	h.Log.Info(fmt.Sprintf("subscribed qos=%v", reasonCodes), "client", cl.ID, "filters", pk.Filters)
}
}

func (h *ExampleHook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	h.Log.Info("unsubscribed", "client", cl.ID, "filters", pk.Filters)
}

func (h *ExampleHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if cl.ID != "inline" {
	h.Log.Info("received from client", "client", cl.ID, "payload", string(pk.Payload))
	
	pkx := pk
	if string(pk.Payload) == "hello" {
		pkx.Payload = []byte("hello world")
		h.Log.Info("received modified packet from client", "client", cl.ID, "payload", string(pkx.Payload))
	}
	
	return pkx, nil
}
return pk, nil
}

func (h *ExampleHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	if cl.ID != "inline" {
	h.Log.Info("published to client", "client", cl.ID, "payload", string(pk.Payload))
}
}