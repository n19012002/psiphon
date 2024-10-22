package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Config định nghĩa các cấu hình cho Psiphon
type Config struct {
	CoreName       string
	Tunnel         int
	Region         string
	Protocols      []string
	TunnelWorkers  int
	KuotaDataLimit int
	Authorizations []string
}

// Psiphon chứa các thông tin về kết nối
type Psiphon struct {
	Config         *Config
	TunnelConnected int
	KuotaData      struct {
		Port map[int]map[string]float64
	}
	ListenPort int
	Verbose    bool
}

// DefaultConfig định nghĩa các giá trị mặc định
var DefaultConfig = &Config{
	CoreName: "psiphon-tunnel-core",
	Tunnel:   10, // Số luồng kết nối tối ưu
	Region:   "", // Khu vực không xác định
	Protocols: []string{
		"FRONTED-MEEK-HTTP-OSSH",
		"FRONTED-MEEK-OSSH",
	},
	TunnelWorkers:  200, // Số lượng workers để cải thiện kết nối
	KuotaDataLimit: 0,   // Không giới hạn băng thông
	Authorizations: make([]string, 0),
}

// CheckKuotaDataLimit kiểm tra giới hạn băng thông để tránh throttle
func (p *Psiphon) CheckKuotaDataLimit(sent, received float64) bool {
	if p.Config.KuotaDataLimit != 0 && int(p.KuotaData.Port[p.ListenPort]["all"]) >= (p.Config.KuotaDataLimit*1000000) &&
		int(sent) == 0 && int(received) <= 64000 {
		return false
	}
	return true
}

// LogVerbose ghi log nếu chế độ verbose được bật
func (p *Psiphon) LogVerbose(message string, color string) {
	if p.Verbose {
		fmt.Printf("VERBOSE: %s\n", message)
	}
}

// RunPsiphon khởi chạy Psiphon Tunnel và quản lý kết nối
func (p *Psiphon) RunPsiphon() {
	for {
		// Khởi tạo lại dữ liệu kết nối
		p.KuotaData.Port[p.ListenPort] = make(map[string]float64)
		p.KuotaData.Port[p.ListenPort]["all"] = 0
		p.TunnelConnected = 0

		// Lệnh chạy Psiphon Tunnel Core
		command := exec.Command(
			p.Config.CoreName, "-config", "./config.json",
		)

		// Lấy stderr để ghi lại lỗi
		stderr, err := command.StderrPipe()
		if err != nil {
			panic(err)
		}

		// Đọc log từ Psiphon
		scanner := bufio.NewScanner(stderr)
		go func() {
			for scanner.Scan() {
				text := scanner.Text()
				var line map[string]interface{}
				json.Unmarshal([]byte(text), &line)

				noticeType := line["noticeType"]

				if noticeType == "BytesTransferred" {
					data := line["data"].(map[string]interface{})
					sent := data["sent"].(float64)
					received := data["received"].(float64)

					// Kiểm tra giới hạn dữ liệu
					if !p.CheckKuotaDataLimit(sent, received) {
						p.LogVerbose("Reached data limit, stopping connection...", "R1")
						break
					}
				}

				if noticeType == "Alert" || noticeType == "Warning" {
					message := line["data"].(map[string]interface{})["message"].(string)
					if strings.Contains(message, "tunnel failed:") || strings.Contains(message, "underlying conn is closed") {
						p.LogVerbose("Tunnel failed, reconnecting...", "R1")
						break
					}
				}
			}
		}()

		// Thực thi lệnh Psiphon
		if err := command.Run(); err != nil {
			p.LogVerbose("Error occurred: "+err.Error(), "R1")
		}

		// Chờ 3 giây trước khi thử kết nối lại
		p.LogVerbose("Reconnecting in 3 seconds...", "B1")
		time.Sleep(3 * time.Second)
	}
}

func main() {
	// Khởi tạo đối tượng Psiphon và cấu hình
	psiphon := &Psiphon{
		Config:     DefaultConfig,
		Verbose:    true,
		ListenPort: 8080,
	}
	psiphon.RunPsiphon()
}
