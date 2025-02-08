package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Handle struct {
	validToken   string
	ollamaServer string
	listenerAddr string
	keyFile      string
	certFile     string
	httpClient   *http.Client
}

func main() {
	h := readConfig("./config.conf")
	http.HandleFunc("/", h.handleRequest)

	var err error
	if fileExist(h.keyFile) && fileExist(h.certFile) {
		fmt.Println("Starting HTTPS server...")
		err = http.ListenAndServeTLS(h.listenerAddr, h.certFile, h.keyFile, nil)
	} else if h.certFile == "" && h.keyFile == "" {
		fmt.Println("Starting HTTP server...")
		err = http.ListenAndServe(h.listenerAddr, nil)
	} else {
		fmt.Println("Certificate or key file not found!")
	}
	if err != nil {
		log.Fatal(err)
	}
}

func (h Handle) handleRequest(w http.ResponseWriter, r *http.Request) {
	if h.authorized(r.Header.Get("Authorization")) == false {
		fmt.Printf("Unauthorized request from %s\n", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	r.Header.Del("Authorization")

	fmt.Printf("Accepted request from %s\n", r.RemoteAddr)

	ollamaUrl := fmt.Sprintf("%s%s", h.ollamaServer, r.URL.String())

	ollamaReq, err := http.NewRequest(r.Method, ollamaUrl, nil)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		fmt.Printf("Failed to create request: %v\n", err)
		return
	}

	ollamaReq.Header = r.Header
	ollamaReq.Body = r.Body

	ollamaResp, err := h.httpClient.Do(ollamaReq)
	if err != nil {
		http.Error(w, "Request failed", http.StatusBadGateway)
		fmt.Printf("Request to %s failed: %v\n", ollamaUrl, err)
		return
	}
	defer ollamaResp.Body.Close()

	w.Header().Set("Content-Type", "application/json")

	buff := make([]byte, 4096)

	for {
		n, err := ollamaResp.Body.Read(buff)

		if err != nil && err != io.EOF {
			http.Error(w, "Request failed", http.StatusBadGateway)
			fmt.Printf("Request to %s failed: %v\n", ollamaUrl, err)
			return
		}

		w.Write(buff[:n])
		w.(http.Flusher).Flush()

		if n == 0 || err == io.EOF {
			break
		}
	}
}

func (h Handle) authorized(authHeader string) bool {
	parts := strings.Split(authHeader, " ")
	if parts[0] == "Bearer" && parts[1] == h.validToken {
		return true
	}
	return false
}

func readConfig(file string) Handle {
	configData, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	handle := Handle{}

	for _, l := range strings.Split(string(configData), "\n") {
		if strings.Contains(l, ":") == false {
			continue
		}

		line := strings.SplitN(strings.TrimSpace(l), ":", 2)
		tType, tVal := strings.ToLower(line[0]), line[1]

		switch tType {
		case "ollama_server":
			handle.ollamaServer = strings.ToLower(tVal)
		case "auth_token":
			handle.validToken = tVal
		case "listener_addr":
			handle.listenerAddr = strings.ToLower(tVal)
		case "key_file":
			handle.keyFile = tVal
		case "cert_file":
			handle.certFile = tVal
		default:
			continue
		}
	}
	handle.httpClient = &http.Client{}

	if handle.listenerAddr == "" {
		log.Fatal("Listener address not set in config")
	}
	if handle.ollamaServer == "" {
		log.Fatal("Ollama server URL not set in config")
	}
	if handle.validToken == "" {
		log.Fatal("Auth token not set in config")
	}

	return handle
}

func fileExist(file string) bool {
	_, err := os.Stat(file)
	if err == nil {
		return true
	}
	return false
}
