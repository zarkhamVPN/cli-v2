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

	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/spf13/cobra"
)

var nodeInstance *core.ZarkhamNode

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Zarkham dVPN node",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		nodeInstance, err = core.NewZarkhamNode(core.Config{
			ConfigDir:   cfgDir,
			RpcEndpoint: rpcEndpoint,
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

		// Start API Server
		go startServer()

		// Wait for interruption
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down...")
		_ = nodeInstance.Stop()
	},
}

func startServer() {
	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("/api/warden-status", corsMiddleware(handleWardenStatus))
	mux.HandleFunc("/api/wardens", corsMiddleware(handleGetWardens))
	mux.HandleFunc("/api/warden/lookup", corsMiddleware(handleWardenLookup)) // New
	mux.HandleFunc("/api/deposit", corsMiddleware(handleDeposit))           // New
	mux.HandleFunc("/api/node/connect", corsMiddleware(handleNodeConnect))
	mux.HandleFunc("/api/balance", corsMiddleware(handleGetBalance))
	mux.HandleFunc("/api/addresses", corsMiddleware(handleGetAddresses))
	mux.HandleFunc("/api/register-warden", corsMiddleware(handleRegisterWarden))
	mux.HandleFunc("/api/node/start", corsMiddleware(handleNodeStart))
	mux.HandleFunc("/api/node/stop", corsMiddleware(handleNodeStop))
	mux.HandleFunc("/api/node/status", corsMiddleware(handleNodeStatus))
	mux.HandleFunc("/api/history", corsMiddleware(handleGetHistory))
	mux.HandleFunc("/api/profile", corsMiddleware(handleGetProfile))

	// GUI Server
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else if path[0] == '/' {
			path = path[1:]
		}

		f, err := embeddedGUI.Open(path)
		if os.IsNotExist(err) {
			// SPA Fallback: Serve index.html for unknown paths
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
		EstimatedMb uint64 `json:"estimatedMb"` // Optional
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	
	mb := req.EstimatedMb
	if mb == 0 {
		mb = 100 // Default
	}

	err := nodeInstance.ManualConnect(r.Context(), req.Multiaddr, mb)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
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

	// Find the optimal P2P multiaddr
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
			// This is a quic-v1 address
			if manet.IsPublicAddr(ma) {
				publicQuicAddr = addrStr
				break // Found a public one, prioritize and break
			} else if privateQuicAddr == "" && !strings.Contains(addrStr, "127.0.0.1") {
				// Keep the first non-loopback private address as a fallback
				privateQuicAddr = addrStr
			}
		}
	}

	if publicQuicAddr != "" {
		p2pMultiaddr = publicQuicAddr
	} else if privateQuicAddr != "" {
		p2pMultiaddr = privateQuicAddr
	} else if len(status.Addresses) > 0 {
		// Fallback to the first available address if no quic-v1 specific found
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

	// Append PeerID to make it a full multiaddr if we found a base address
	if p2pMultiaddr != "" && status.PeerID != "" {
		// Check if it already has the p2p component (it shouldn't, but safety first)
		if !strings.Contains(p2pMultiaddr, "/p2p/") {
			p2pMultiaddr = fmt.Sprintf("%s/p2p/%s", p2pMultiaddr, status.PeerID)
		}
	}

	response := map[string]interface{}{
		"isRunning":    status.IsRunning,
		"peerId":       status.PeerID,
		"addresses":    status.Addresses, // Keep all addresses for debugging/info
		"p2pMultiaddr": p2pMultiaddr,     // The selected optimal multiaddr
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

func handleGetProfile(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"profile": profile})
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
	rootCmd.AddCommand(startCmd)
}
