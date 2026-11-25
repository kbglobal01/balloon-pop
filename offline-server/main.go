// 풍선 터뜨리기 오프라인 서버: 같은 와이파이의 휴대폰에 게임을 보내 줍니다.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

//go:embed site
var siteFS embed.FS

type addr struct {
	IP    string `json:"ip"`
	Name  string `json:"name"`
	score int
}

func lanAddrs() []addr {
	var out []addr
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP.To4()
			if ip == nil || !ip.IsPrivate() {
				continue
			}
			name := strings.ToLower(ifc.Name)
			s := 0
			for _, bad := range []string{"virtual", "vmware", "vbox", "docker", "vethernet", "wsl", "hyper-v", "tailscale", "zerotier", "utun", "bridge", "br-", "veth", "vmnet"} {
				if strings.Contains(name, bad) {
					s -= 10
				}
			}
			for _, good := range []string{"wi-fi", "wifi", "wlan", "wireless", "en0", "무선"} {
				if strings.Contains(name, good) {
					s += 5
				}
			}
			if ip[0] == 192 && ip[1] == 168 {
				s += 2
				if ip[2] == 56 {
					s -= 6
				}
			} else if ip[0] == 10 {
				s++
			}
			out = append(out, addr{IP: ip.String(), Name: ifc.Name, score: s})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

func listen() (net.Listener, int) {
	for p := 8080; p <= 8099; p++ {
		if l, err := net.Listen("tcp", fmt.Sprintf(":%d", p)); err == nil {
			return l, p
		}
	}
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		fmt.Println("서버를 켤 수 없어요:", err)
		waitExit()
	}
	return l, l.Addr().(*net.TCPAddr).Port
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func waitExit() {
	fmt.Println("\n엔터 키를 누르면 창이 닫혀요.")
	var s string
	fmt.Scanln(&s)
	os.Exit(1)
}

func isLocalHost(h string) bool {
	host, _, err := net.SplitHostPort(h)
	if err != nil {
		host = h
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]"
}

func main() {
	setupConsole()
	site, _ := fs.Sub(siteFS, "site")
	files := http.FileServer(http.FS(site))
	ln, port := listen()

	gameURL := func(r *http.Request) string {
		if r != nil && !isLocalHost(r.Host) {
			return "http://" + r.Host + "/"
		}
		if a := lanAddrs(); len(a) > 0 {
			return fmt.Sprintf("http://%s:%d/", a[0].IP, port)
		}
		return fmt.Sprintf("http://localhost:%d/", port)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/config.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		b, _ := json.Marshal(gameURL(r))
		fmt.Fprintf(w, "window.APP_URL = %s;\n", b)
	})
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		a := lanAddrs()
		if a == nil {
			a = []addr{}
		}
		json.NewEncoder(w).Encode(map[string]any{"port": port, "ips": a})
	})
	mux.HandleFunc("/host", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/host.html"
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-store")
		}
		files.ServeHTTP(w, r)
	})

	fmt.Println("==============================================")
	fmt.Println("  풍선 터뜨리기 게임 서버가 켜졌어요!")
	fmt.Println("==============================================")
	fmt.Println()
	a := lanAddrs()
	if len(a) == 0 {
		fmt.Println("  ! 와이파이에 연결되어 있지 않아요.")
		fmt.Println("    와이파이에 연결하면 시연 화면의 QR코드가 저절로 바뀌어요.")
	} else {
		fmt.Println("  휴대폰에서 열 주소:")
		for _, x := range a {
			fmt.Printf("    http://%s:%d/   (%s)\n", x.IP, port, x.Name)
		}
	}
	host := fmt.Sprintf("http://localhost:%d/host", port)
	fmt.Println()
	fmt.Println("  시연 화면(QR코드):", host)
	fmt.Println()
	fmt.Println("  이 창을 닫으면 게임이 멈춰요. 시연이 끝날 때까지 켜 두세요.")
	fmt.Println()

	go func() {
		time.Sleep(600 * time.Millisecond)
		openBrowser(host)
	}()

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.Serve(ln); err != nil {
		fmt.Println("서버가 멈췄어요:", err)
		waitExit()
	}
}
