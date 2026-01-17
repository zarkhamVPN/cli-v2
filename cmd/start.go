package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"zarkham/core"
	"zarkham/core/config"
	"zarkham/core/logger"

	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/spf13/cobra"
)

var nodeInstance *core.ZarkhamNode

var p2pPort int

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Zarkham dVPN node",
	Run: func(cmd *cobra.Command, args []string) {
		appCfg, err := config.LoadOrInit(cfgDir)
		if err != nil {
			log.Fatalf("Failed to load app config: %v", err)
		}

		nodeInstance, err = core.NewZarkhamNode(core.Config{
			ConfigDir:   cfgDir,
			RpcEndpoint: rpcEndpoint,
			SubmitTC:    appCfg.SubmitTC,
			P2PPort:     p2pPort,
		})
		if err != nil {
			log.Fatalf("Failed to initialize node: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := nodeInstance.Start(ctx, profile); err != nil {
			log.Fatalf("Failed to start node: %v", err)
		}

		log.Printf("Zarkham Node is running (Profile: %s)", profile)

		go startServer()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down...")
		_ = nodeInstance.Stop()
	},
}

func startServer() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/warden-status", corsMiddleware(handleWardenStatus))
	mux.HandleFunc("/api/seeker-status", corsMiddleware(handleSeekerStatus))
	mux.HandleFunc("/api/wardens", corsMiddleware(handleGetWardens))
	mux.HandleFunc("/api/warden/lookup", corsMiddleware(handleWardenLookup))
	mux.HandleFunc("/api/deposit", corsMiddleware(handleDeposit))
	mux.HandleFunc("/api/node/connect", corsMiddleware(handleNodeConnect))
	mux.HandleFunc("/api/node/latency", corsMiddleware(handleGetLatency))
	mux.HandleFunc("/api/node/bandwidth", corsMiddleware(handleGetBandwidth))
	mux.HandleFunc("/api/seeker/disconnect", corsMiddleware(handleSeekerDisconnect))
	mux.HandleFunc("/api/balance", corsMiddleware(handleGetBalance))
	mux.HandleFunc("/api/addresses", corsMiddleware(handleGetAddresses))
	mux.HandleFunc("/api/register-warden", corsMiddleware(handleRegisterWarden))
	mux.HandleFunc("/api/node/start", corsMiddleware(handleNodeStart))
	mux.HandleFunc("/api/node/stop", corsMiddleware(handleNodeStop))
	mux.HandleFunc("/api/node/status", corsMiddleware(handleNodeStatus))
	mux.HandleFunc("/api/history", corsMiddleware(handleGetHistory))
	mux.HandleFunc("/api/profile", corsMiddleware(handleGetProfile))
	mux.HandleFunc("/api/transfer", corsMiddleware(handleTransfer))
	mux.HandleFunc("/api/logs", corsMiddleware(handleLogs))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else if path[0] == '/' {
			path = path[1:]
		}

		f, err := embeddedGUI.Open(path)
		if os.IsNotExist(err) {
			index, err := embeddedGUI.Open("index.html")
			if err != nil {
				http.Error(w, "GUI not found", 404)
				return
			}
			defer index.Close()
			stat, _ := index.Stat()
			http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
			return
		} else if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer f.Close()

		stat, _ := f.Stat()
		http.ServeContent(w, r, path, stat.ModTime(), f.(io.ReadSeeker))
	})

	log.Println("Zarkham API Server listening on :8088")
	if err := http.ListenAndServe(":8088", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	logChan := logger.Subscribe()
	defer logger.Unsubscribe(logChan)

	notify := w.(http.CloseNotifier).CloseNotify()
	
	for {
		select {
		case entry := <-logChan:
			data, err := json.Marshal(entry)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		case <-notify:
			return
		case <-r.Context().Done():
			return
		}
	}
}

func handleSeekerStatus(w http.ResponseWriter, r *http.Request) {
	reg, data, err := nodeInstance.GetSeekerStatus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"is_registered": reg,
		"seeker":        data,
	})
}

func handleSeekerDisconnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WardenAuthority string `json:"wardenAuthority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}

	err := nodeInstance.DisconnectWarden(r.Context(), profile, req.WardenAuthority)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{}"))
}

func handleWardenStatus(w http.ResponseWriter, r *http.Request) {
	reg, data, err := nodeInstance.GetWardenStatus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"is_registered": reg,
		"warden":        data,
	})
}

func handleGetWardens(w http.ResponseWriter, r *http.Request) {
	wardens, err := nodeInstance.GetWardens(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(wardens)
}

func handleWardenLookup(w http.ResponseWriter, r *http.Request) {
	peerID := r.URL.Query().Get("peer_id")
	if peerID == "" {
		http.Error(w, "missing peer_id", 400)
		return
	}
	warden, err := nodeInstance.LookupWarden(r.Context(), peerID)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	json.NewEncoder(w).Encode(warden)
}

func handleDeposit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount uint64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	sig, err := nodeInstance.DepositEscrow(r.Context(), req.Amount)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"signature": sig})
}

func handleNodeConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Multiaddr   string `json:"multiaddr"`
		EstimatedMb uint64 `json:"estimatedMb"` 
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	
	mb := req.EstimatedMb
	if mb == 0 {
		mb = 100 
	}

	err := nodeInstance.ManualConnect(r.Context(), req.Multiaddr, mb)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func handleGetLatency(w http.ResponseWriter, r *http.Request) {
	lat, err := nodeInstance.GetLatency(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(lat)
}

func handleGetBalance(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	if profile == "" {
		http.Error(w, "missing profile", 400)
		return
	}
	bal, err := nodeInstance.GetWalletBalance(r.Context(), profile)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]uint64{"lamports": bal})
}

func handleGetAddresses(w http.ResponseWriter, r *http.Request) {
	addrs, err := nodeInstance.GetAddresses()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(addrs)
}

func handleRegisterWarden(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile     string  `json:"profile"`
		StakeToken  string  `json:"stakeToken"`
		StakeAmount float64 `json:"stakeAmount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	sig, err := nodeInstance.RegisterWarden(r.Context(), req.Profile, req.StakeToken, req.StakeAmount)
	if err != nil {
		log.Printf("Registration failed: %v", err)
		http.Error(w, fmt.Sprintf("Registration failed: %v", err), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"transactionSignature": sig})
}

func handleNodeStart(w http.ResponseWriter, r *http.Request) {
	if err := nodeInstance.Start(r.Context(), profile); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func handleNodeStop(w http.ResponseWriter, r *http.Request) {
	if err := nodeInstance.Stop(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func handleNodeStatus(w http.ResponseWriter, r *http.Request) {
	status := nodeInstance.Status()

	var p2pMultiaddr string
	var publicQuicAddr string
	var privateQuicAddr string

	for _, addrStr := range status.Addresses {
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			log.Printf("Failed to parse multiaddr '%s': %v", addrStr, err)
			continue
		}

		if _, err := ma.ValueForProtocol(multiaddr.P_QUIC_V1); err == nil {
			if manet.IsPublicAddr(ma) {
				publicQuicAddr = addrStr
				break 
			} else if privateQuicAddr == "" && !strings.Contains(addrStr, "127.0.0.1") {
				privateQuicAddr = addrStr
			}
		}
	}

	if publicQuicAddr != "" {
		p2pMultiaddr = publicQuicAddr
	} else if privateQuicAddr != "" {
		p2pMultiaddr = privateQuicAddr
	} else if len(status.Addresses) > 0 {
		for _, addrStr := range status.Addresses {
			ma, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				continue
			}
			if _, err := ma.ValueForProtocol(multiaddr.P_QUIC_V1); err == nil {
				p2pMultiaddr = addrStr
				break
			}
		}
	}

	if p2pMultiaddr != "" && status.PeerID != "" {
		if !strings.Contains(p2pMultiaddr, "/p2p/") {
			p2pMultiaddr = fmt.Sprintf("%s/p2p/%s", p2pMultiaddr, status.PeerID)
		}
	}

	response := map[string]interface{}{
		"isRunning":    status.IsRunning,
		"peerId":       status.PeerID,
		"addresses":    status.Addresses, 
		"p2pMultiaddr": p2pMultiaddr,
		"version":      "v1.0.0",
	}
	json.NewEncoder(w).Encode(response)
}

func handleGetHistory(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	if profile == "" {
		http.Error(w, "missing profile", 400)
		return
	}
	history, err := nodeInstance.GetHistory(r.Context(), profile)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(history)
}

func handleGetBandwidth(w http.ResponseWriter, r *http.Request) {
	bw, err := nodeInstance.GetBandwidth(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(bw)
}

func handleGetProfile(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"profile": profile})
}

func handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile   string  `json:"profile"`
		Recipient string  `json:"recipient"`
		Amount    float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	sig, err := nodeInstance.TransferFunds(r.Context(), req.Profile, req.Recipient, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"signature": sig})
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func init() {
	startCmd.Flags().IntVar(&p2pPort, "p2p-port", 0, "Port to listen for P2P connections (0 for random)")
	rootCmd.AddCommand(startCmd)
}