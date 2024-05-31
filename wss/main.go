package main

import (
	"crypto/tls"
	"log"
	"net"
)

func main() {
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


	listener, err := tls.Listen("tcp", ":1882", tlsConfig)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	log.Println("Server started")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		log.Println("connection is not a TLS connection")
		return
	}

	err := tlsConn.Handshake()
	if err != nil {
		log.Printf("TLS handshake error: %v", err)
		return
	}

	state := tlsConn.ConnectionState()


	log.Printf("ClientHelloInfo: %+v", state)
}
