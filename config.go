package proxy

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

const (
	defaultCmdPrefix    = ">"
	defaultSendInterval = 0.09
	defaultUserLimit    = 10
	defaultAuthBackend  = "files"
	defaultTelnetAddr   = "[::1]:40010"
	defaultBindAddr     = ":40000"
	defaultListInterval = 300
)

var config Config
var configMu sync.RWMutex

var loadConfigOnce sync.Once

type Server struct {
	Name      string
	Addr      string
	Fallbacks []string
}

// A Config contains information from the configuration file
// that affects the way the proxy works.
type Config struct {
	NoPlugins       bool
	CmdPrefix       string
	RequirePasswd   bool
	SendInterval    float32
	UserLimit       int
	AuthBackend     string
	NoTelnet        bool
	TelnetAddr      string
	BindAddr        string
	Servers         []Server
	ForceDefaultSrv bool
	FallbackServers []string
	CSMRF           struct {
		NoCSMs          bool
		ChatMsgs        bool
		ItemDefs        bool
		NodeDefs        bool
		NoLimitMapRange bool
		PlayerList      bool
	}
	MapRange   uint32
	DropCSMRF  bool
	Groups     map[string][]string
	UserGroups map[string]string
	List       struct {
		Enable   bool
		Addr     string
		Interval int

		Name     string
		Desc     string
		URL      string
		Creative bool
		Dmg      bool
		PvP      bool
		Game     string
		FarNames bool
		Mods     []string
	}
}

// Conf returns a copy of the Config used by the proxy.
// Any modifications will not affect the original Config.
func Conf() Config {
	loadConfigOnce.Do(func() {
		if err := LoadConfig(); err != nil {
			log.Fatal(err)
		}
	})

	configMu.RLock()
	defer configMu.RUnlock()

	return config
}

// AddServer dynamically configures a new Server at runtime.
// Servers added in this way are ephemeral and will be lost
// when the proxy shuts down.
// For the media to work you have to specify an alternative
// media source that is always available, even if the server
// is offline.
// WARNING: Reloading the configuration file deletes the
// servers added using this function.
func AddServer(server Server) bool {
	configMu.Lock()
	defer configMu.Unlock()

	for _, srv := range config.Servers {
		if srv.Name == server.Name {
			return false
		}
	}

	config.Servers = append(config.Servers, server)
	return true
}

// RmServer deletes a Server from the Config at runtime.
// Any server can be deleted this way, not just the ones
// added using AddServer.
// WARNING: Reloading the configuration files re-adds the
// servers it contains that were deleted using this function.
func RmServer(name string) {
	configMu.Lock()
	defer configMu.Unlock()

	for i, srv := range config.Servers {
		if srv.Name == name {
			config.Servers = append(config.Servers[:i], config.Servers[1+i:]...)
			return
		}
	}
}

// FallbackServers returns a slice of server names that
// a server can fall back to.
func FallbackServers(server string) []string {
	configMu.RLock()
	defer configMu.RUnlock()

	fallbacks := make([]string, 0)

	conf := Conf()

	// find server
	for _, srv := range conf.Servers {
		if srv.Name == server {
			fallbacks = append(fallbacks, srv.Fallbacks...)
			break
		}
	}

	// global fallbacks
	if len(conf.FallbackServers) == 0 {
		if len(conf.Servers) == 0 {
			return fallbacks
		}

		return append(fallbacks, conf.Servers[0].Name)
	} else {
		return append(fallbacks, conf.FallbackServers...)
	}
}

// LoadConfig attempts to parse the configuration file.
// It leaves the config unchanged if there is an error
// and returns the error.
func LoadConfig() error {
	configMu.Lock()
	defer configMu.Unlock()

	oldConf := config

	config.CmdPrefix = defaultCmdPrefix
	config.SendInterval = defaultSendInterval
	config.UserLimit = defaultUserLimit
	config.AuthBackend = defaultAuthBackend
	config.TelnetAddr = defaultTelnetAddr
	config.BindAddr = defaultBindAddr
	config.FallbackServers = make([]string, 0)
	config.Groups = make(map[string][]string)
	config.UserGroups = make(map[string]string)
	config.List.Interval = defaultListInterval

	f, err := os.OpenFile(Path("config.json"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		config = oldConf
		return err
	}
	defer f.Close()

	if fi, _ := f.Stat(); fi.Size() == 0 {
		f.WriteString("{\n\t\n}\n")
		f.Seek(0, os.SEEK_SET)
	}

	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&config); err != nil {
		config = oldConf
		return err
	}

	log.Print("load config")
	return nil
}
