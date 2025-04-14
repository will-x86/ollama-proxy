package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

var (
	listenAddr    = fmt.Sprintf(":%s", os.Getenv("LISTEN_ADDR"))
	serverAAddr   = os.Getenv("SERVER_A_ADDR")
	serverBAddr   = os.Getenv("SERVER_B_ADDR")
	checkInterval = 5 * time.Second
)

type ServerStatus struct {
	serverAOnline bool
	mu            sync.RWMutex
}

func main() {
	// Works with tailscale, woot woot
	serverA, err := url.Parse(serverAAddr)
	if err != nil {
		log.Fatalf("Failed to parse server A URL: %v", err)
	}

	serverB, err := url.Parse(serverBAddr)
	if err != nil {
		log.Fatalf("Failed to parse server B URL: %v", err)
	}

	proxyA := httputil.NewSingleHostReverseProxy(serverA)
	proxyB := httputil.NewSingleHostReverseProxy(serverB)

	configureWebsocketProxy(proxyA)
	configureWebsocketProxy(proxyB)

	status := &ServerStatus{serverAOnline: true}

	go healthChecker(status, serverA)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		status.mu.RLock()
		serverAIsOnline := status.serverAOnline
		status.mu.RUnlock()

		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		log.Printf("Server A online: %v", serverAIsOnline)

		if serverAIsOnline {
			log.Printf("Proxying to server A: %s", serverAAddr)
			proxyA.ServeHTTP(w, r)
		} else {
			log.Printf("Proxying to server B: %s", serverBAddr)
			proxyB.ServeHTTP(w, r)
		}
	})

	log.Printf("Starting reverse proxy on %s", listenAddr)
	log.Printf("Primary target: %s", serverAAddr)
	log.Printf("Fallback target: %s", serverBAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func configureWebsocketProxy(proxy *httputil.ReverseProxy) {
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		if isWebSocketRequest(req) {
			// Ensure necessary headers are forwarded
			if req.Header.Get("Connection") != "" {
				req.Header.Set("Connection", "Upgrade")
			}
			if req.Header.Get("Upgrade") != "" {
				req.Header.Set("Upgrade", "websocket")
			}
		}
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode == http.StatusSwitchingProtocols {
			log.Println("WebSocket connection established")
		}
		return nil
	}
}

func isWebSocketRequest(req *http.Request) bool {
	return req.Header.Get("Upgrade") == "websocket" &&
		req.Header.Get("Connection") == "Upgrade"
}

func healthChecker(status *ServerStatus, serverA *url.URL) {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	for {
		req, err := http.NewRequest("HEAD", serverA.String(), nil)
		if err != nil {
			log.Printf("Error creating health check request: %v", err)
			setServerAStatus(status, false)
			time.Sleep(checkInterval)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Health check failed for server A: %v", err)
			setServerAStatus(status, false)
		} else {
			resp.Body.Close()
			log.Printf("Server A is online (status code: %d)", resp.StatusCode)
			setServerAStatus(status, true)
		}

		time.Sleep(checkInterval)
	}
}

func setServerAStatus(status *ServerStatus, online bool) {
	status.mu.Lock()
	defer status.mu.Unlock()

	if status.serverAOnline != online {
		if online {
			log.Println("Server A is now online. Switching back to server A.")
		} else {
			log.Println("Server A is offline. Switching to server B.")
		}
		status.serverAOnline = online
	}
}
