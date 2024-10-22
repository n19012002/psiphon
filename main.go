package libpsiphon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/aztecrabbit/liblog"
	"github.com/aztecrabbit/libproxyrotator"
	"github.com/aztecrabbit/libutils"
)

var (
	Loop          = true
	DefaultConfig = &Config{
		CoreName: "psiphon-tunnel-core",
		Tunnel:   10,
		Region:   "",
		Protocols: []string{
			"FRONTED-MEEK-HTTP-OSSH",
			"FRONTED-MEEK-OSSH",
		},
		TunnelWorkers:  200,
		KuotaDataLimit: 0,
		Authorizations: make([]string, 0),
	}
	DefaultKuotaData = &KuotaData{
		Port: make(map[int]map[string]float64),
		All:  0,
	}
	ConfigPathPsiphon = libutils.GetConfigPath("brainfuck-psiphon-pro-go", "storage/psiphon")
)

func Stop() {
	Loop = false
}

func RemoveData() {
	os.RemoveAll(ConfigPathPsiphon + "/data")
}

type Config struct {
	CoreName       string
	Tunnel         int
	Region         string
	Protocols      []string
	TunnelWorkers  int
	KuotaDataLimit int
	Authorizations []string
}

type KuotaData struct {
	Port map[int]map[string]float64
	All  float64
}

type Data struct {
	MigrateDataStoreDirectory string
	UpstreamProxyURL          string
	LocalSocksProxyPort       int
	SponsorId                 string
	PropagationChannelId      string
	EmitBytesTransferred      bool
	EmitDiagnosticNotices     bool
	DisableLocalHTTPProxy     bool
	EgressRegion              string
	TunnelPoolSize            int
	ConnectionWorkerPoolSize  int
	LimitTunnelProtocols      []string
	Authorizations            []string
}

type Psiphon struct {
	ProxyRotator    *libproxyrotator.ProxyRotator
	Config          *Config
	ProxyPort       string
	KuotaData       *KuotaData
	ListenPort      int
	TunnelConnected int
	Verbose         bool
}

func (p *Psiphon) LogInfo(message string, color string) {
	if Loop {
		liblog.LogInfo(message, strconv.Itoa(p.ListenPort), color)
	}
}

func (p *Psiphon) LogVerbose(message string, color string) {
	if p.Verbose {
		p.LogInfo(fmt.Sprintf("%[1]sVERBOSE%[3]s %[2]s::%[3]s %[1]s", color, liblog.Colors["P1"], liblog.Colors["CC"])+message, color)
	}
}

func (p *Psiphon) GetAuthorizations() []string {
	data := make([]string, 0)

	if len(p.Config.Authorizations) != 0 {
		data = append(data, p.Config.Authorizations[0])
		p.Config.Authorizations = append(p.Config.Authorizations[1:], p.Config.Authorizations[0])
	}

	return data
}

func (p *Psiphon) CheckKuotaDataLimit(sent float64, received float64) bool {
	if p.Config.KuotaDataLimit != 0 && int(p.KuotaData.Port[p.ListenPort]["all"]) >= (p.Config.KuotaDataLimit*1000000) &&
		int(sent) == 0 && int(received) <= 64000 {
		return false
	}

	return true
}

func (p *Psiphon) Start() {
	PsiphonData := &Data{
		MigrateDataStoreDirectory: ConfigPathPsiphon + "/data/" + strconv.Itoa(p.ListenPort),
		UpstreamProxyURL:          "http://127.0.0.1:" + p.ProxyPort,
		LocalSocksProxyPort:       p.ListenPort,
		SponsorId:                 "00000000000000FF",
		PropagationChannelId:      "00000000000000FF",
		EmitBytesTransferred:      true,
		EmitDiagnosticNotices:     true,
		DisableLocalHTTPProxy:     true,
		EgressRegion:              strings.ToUpper(p.Config.Region),
		TunnelPoolSize:            p.Config.Tunnel,
		ConnectionWorkerPoolSize:  p.Config.TunnelWorkers,
		LimitTunnelProtocols:      p.Config.Protocols,
		Authorizations:            p.GetAuthorizations(),
	}

	libutils.JsonWrite(PsiphonData, PsiphonData.MigrateDataStoreDirectory+"/config.json")

	PsiphonFileBoltdb := PsiphonData.MigrateDataStoreDirectory + "/ca.psiphon.PsiphonTunnel.tunnel-core/datastore/psiphon.boltdb"
	if _, err := os.Stat(PsiphonFileBoltdb); os.IsNotExist(err) {
		libutils.CopyFile(
			libutils.RealPath("/storage/psiphon/database/psiphon.boltdb"), PsiphonFileBoltdb,
		)
	}

	p.LogInfo("Connecting", liblog.Colors["G1"])

	for Loop {
		p.KuotaData.Port[p.ListenPort] = make(map[string]float64)
		p.KuotaData.Port[p.ListenPort]["all"] = 0
		p.TunnelConnected = 0

		command := exec.Command(
			libutils.RealPath(p.Config.CoreName), "-config", PsiphonData.MigrateDataStoreDirectory+"/config.json",
		)
		command.Dir = PsiphonData.MigrateDataStoreDirectory

		stderr, err := command.StderrPipe()
		if err != nil {
			panic(err)
		}

		scanner := bufio.NewScanner(stderr)
		go func() {
			var text string
			var line map[string]interface{}
			for Loop && scanner.Scan() {
				text = scanner.Text()
				json.Unmarshal([]byte(text), &line)

				noticeType := line["noticeType"]

				if noticeType == "BytesTransferred" {
					data := line["data"].(map[string]interface{})
					diagnosticID := data["diagnosticID"].(string)
					sent := data["sent"].(float64)
					received := data["received"].(float64)

					p.KuotaData.Port[p.ListenPort][diagnosticID] += sent + received
					p.KuotaData.Port[p.ListenPort]["all"] += sent + received
					p.KuotaData.All += sent + received

					if p.CheckKuotaDataLimit(sent, received) == false {
						break
					}

					liblog.LogReplace(
						fmt.Sprintf(
							"%v (%v) (%v) (%v)",
							p.ListenPort,
							diagnosticID,
							libutils.BytesToSize(p.KuotaData.Port[p.ListenPort][diagnosticID]),
							libutils.BytesToSize(p.KuotaData.All),
						),
						liblog.Colors["G1"],
					)

				} else if noticeType == "ActiveTunnel" {
					p.ProxyRotator.AddProxy("0.0.0.0:" + strconv.Itoa(p.ListenPort))
					p.TunnelConnected++
					if p.Config.Tunnel > 1 {
						diagnosticID := line["data"].(map[string]interface{})["diagnosticID"].(string)
						p.LogInfo(fmt.Sprintf("Connected (%s)", diagnosticID), liblog.Colors["Y1"])
					}
					if p.TunnelConnected == p.Config.Tunnel {
						p.LogInfo("Connected", liblog.Colors["Y1"])
					}

				} else if noticeType == "Alert" || noticeType == "Warning" {
					message := line["data"].(map[string]interface{})["message"].(string)

					if strings.HasPrefix(message, "Config migration:") {
						continue
					} else if strings.Contains(message, "meek round trip failed") {
						if p.Config.Tunnel == 1 && p.Config.Tunnel == p.TunnelConnected && (message == "meek round trip failed: remote error: tls: bad record MAC" ||
							message == "meek round trip failed: context deadline exceeded" ||
							message == "meek round trip failed: EOF" ||
							strings.Contains(message, "psiphon.CustomTLSDial")) {
							p.LogVerbose(text, liblog.Colors["R1"])
							break
						}
					} else if strings.Contains(message, "controller shutdown due to component failure") ||
						strings.Contains(message, "psiphon.(*ServerContext).DoConnectedRequest") ||
						strings.Contains(message, "psiphon.(*Tunnel).sendSshKeepAlive") ||
						strings.Contains(message, "psiphon.(*Tunnel).Activate") ||
						strings.Contains(message, "underlying conn is closed") ||
						strings.Contains(message, "duplicate tunnel:") ||
						strings.Contains(message, "tunnel failed:") {
						p.LogVerbose(text, liblog.Colors["R1"])
						break
					} else if strings.Contains(message, "A connection attempt failed because the connected party did not properly respond after a period of time") {
						p.LogVerbose(text, liblog.Colors["R1"])
						break
					}

					p.LogVerbose(text, liblog.Colors["R1"])
				}
			}
		}()

		command.Run()

		p.LogVerbose("Reconnecting", liblog.Colors["B1"])
		libutils.Sleep(time.Second * 3)
	}

	liblog.LogDelete(p.ListenPort, liblog.Colors["R1"])
}
