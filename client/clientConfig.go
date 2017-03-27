package main

import (
	"Lunnel/msg"
	"crypto/sha1"
	"encoding/json"
	"io/ioutil"
	rawLog "log"
	"net"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"golang.org/x/crypto/pbkdf2"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Prod    bool   `yaml:"prod,omitempty"`
	LogFile string `yaml:"log_file,omitempty"`
	//if EncryptMode is tls and ServerName is empty,ServerAddr can't be IP format
	ServerAddr  string `yaml:"server_addr"`
	ServerName  string `yaml:"server_name,omitempty"`
	TrustedCert string `yaml:"trusted_cert,omitempty"`
	SecretKey   string `yaml:"secret_key,omitempty"`
	//none:no encryption
	//aes:encrpted by aes
	//tls:encrpted by tls,which is default
	EncryptMode string                      `yaml:"encrypt_mode,omitempty"`
	Tunnels     map[string]msg.TunnelConfig `yaml:"tunnels"`
	AuthToken   string                      `yaml:"auth_token,omitempty"`
	//mix: switch between kcp and tcp automatically,which is default
	//kcp: communicate with server in kcp
	//tcp: communicate with server in tcp
	Transport string `yaml:"transport,omitempty"`
	HttpProxy string `yaml:"http_proxy,omitempty"`
}

var cliConf Config

func LoadConfig(configFile string) error {
	if configFile != "" {
		content, err := ioutil.ReadFile(configFile)
		if err != nil {
			return errors.Wrap(err, "read config file")
		}
		if strings.HasSuffix(configFile, "json") {
			err = json.Unmarshal(content, &cliConf)
			if err != nil {
				return errors.Wrap(err, "unmarshal config file using json decode")
			}
		} else {
			err = yaml.Unmarshal(content, &cliConf)
			if err != nil {
				return errors.Wrap(err, "unmarshal config file using yaml decode")
			}
		}
	}
	if cliConf.ServerAddr == "" {
		cliConf.ServerAddr = "lunnel.snakeoil.com:8080"
	}
	if cliConf.EncryptMode == "" {
		cliConf.EncryptMode = "tls"
	}
	if cliConf.EncryptMode == "aes" {
		if cliConf.SecretKey == "" {
			log.Fatalln("client can't start AES mode without configuring SecretKey")
		}
		pass := pbkdf2.Key([]byte(cliConf.SecretKey), []byte("lunnel"), 4096, 32, sha1.New)
		cliConf.SecretKey = string(pass[:16])
	}
	if cliConf.EncryptMode != "tls" && cliConf.EncryptMode != "aes" && cliConf.EncryptMode != "none" {
		return errors.New("invalid EncryptMode")
	}
	if cliConf.EncryptMode == "tls" && cliConf.ServerName == "" {
		var err error
		cliConf.ServerName, err = resovleServerName(cliConf.ServerAddr)
		if err != nil {
			return errors.Wrap(err, "resovleServerName")
		}
	}
	if len(cliConf.Tunnels) == 0 {
		log.Warningln("no proxying tunnels sepcified")
	}
	if cliConf.Transport == "" {
		cliConf.Transport = "mix"
	} else if cliConf.Transport != "kcp" && cliConf.Transport != "tcp" && cliConf.Transport != "mix" {
		return errors.Errorf("invalid transport mode:%s", cliConf.Transport)
	}
	if (os.Getenv("http_proxy") != "" || os.Getenv("HTTP_PROXY") != "") && cliConf.HttpProxy == "" {
		if os.Getenv("http_proxy") != "" {
			cliConf.HttpProxy = os.Getenv("http_proxy")
		} else if os.Getenv("HTTP_PROXY") != "" {
			cliConf.HttpProxy = os.Getenv("HTTP_PROXY")
		}
	}
	return nil
}

func resovleServerName(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.Wrap(err, "net.SplitHostPort")
	}
	if net.ParseIP(host) != nil {
		return "", errors.New("ServerAddress can't be ip format")
	}
	return host, nil
}

func InitLog() {
	if cliConf.Prod {
		log.SetLevel(log.WarnLevel)
	} else {
		log.SetLevel(log.DebugLevel)
	}
	if cliConf.LogFile != "" {
		f, err := os.OpenFile(cliConf.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
		if err != nil {
			rawLog.Fatalf("open log file failed!err:=%v\n", err)
			return
		}
		log.SetOutput(f)
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetOutput(os.Stdout)
		log.SetFormatter(&log.TextFormatter{})
	}
}