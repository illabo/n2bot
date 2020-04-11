package proxyurl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RandomProxy is a type providing the app with random proxy in selected intervals.
type RandomProxy struct {
	httpClient           *http.Client
	proxyType            string
	providerURL          string
	method               string
	lastAcquiredChan     chan string
	proxyProviderTimeout uint
}

// Get method gets proxy URL. It returns nil whenever it's faild to acquire proxy from provider
// or when it takes too long. Get meant to be used as Proxy in http.Transport.
// If nil URL and nil error are returned from http.Transport's Proxy a request would be made without proxy.
// If error is returned a request would be faied.
func (p *RandomProxy) Get(r *http.Request) (*url.URL, error) {
	timeoutChan := make(chan byte)
	go func() {
		time.Sleep(time.Duration(p.proxyProviderTimeout) * time.Second)
		select {
		case timeoutChan <- '1':
		default:
		}
	}()
	select {
	case u := <-p.lastAcquiredChan:
		p.lastAcquiredChan <- u
		if u == "" {
			return nil, nil
		}
		return url.Parse(u)
	case <-timeoutChan:
		return nil, nil
	}
}

func (p *RandomProxy) startCollectingUrls(valid uint) {
	go func() {
		for {
			u := <-p.lastAcquiredChan
			p.lastAcquiredChan <- u
		}
	}()
	for {
		var hostPort string
		req, err := http.NewRequest(p.method, p.providerURL, nil)
		if err != nil {
			select {
			case <-p.lastAcquiredChan:
			default:
			}
			p.lastAcquiredChan <- hostPort
		}
		res, err := p.httpClient.Do(req)
		if err != nil {
			select {
			case <-p.lastAcquiredChan:
			default:
			}
			p.lastAcquiredChan <- hostPort
		}
		bodyBytes, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			select {
			case <-p.lastAcquiredChan:
			default:
			}
			p.lastAcquiredChan <- hostPort
		}
		hostPort = parseServerAndHostFromRes(bodyBytes)
		if hostPort != "" && strings.HasPrefix(hostPort, p.proxyType) == false {
			hostPort = fmt.Sprintf("%s://%s", p.proxyType, hostPort)
		}
		select {
		case <-p.lastAcquiredChan:
		default:
		}
		p.lastAcquiredChan <- hostPort
		if err == nil {
			time.Sleep(time.Duration(int64(valid)) * time.Second)
		}
	}
}

// NewRandomProxy creates RandomProxy with provided configuration.
func NewRandomProxy(cfg *Config) *RandomProxy {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Type == "" {
		cfg.Type = "socks5"
	}
	if cfg.Method == "" {
		cfg.Method = "GET"
	}
	rp := RandomProxy{
		&http.Client{},
		cfg.Type,
		truncSprintf(cfg.ProviderAPIURLTemplate, cfg.Type, cfg.ExcludeCountries),
		cfg.Method,
		make(chan string),
		cfg.ProviderTimeout,
	}
	go rp.startCollectingUrls(cfg.ValidPeriod)
	return &rp
}

func parseServerAndHostFromRes(body []byte) string {
	var outerMapOfRaws map[string]json.RawMessage
	if json.Unmarshal(body, &outerMapOfRaws) != nil {
		p := strings.Split(string(body), "\n")
		np := len(p)
		rand.Seed(time.Now().UTC().UnixNano())
		r := rand.Intn(np)
		if np > 0 && r < np {
			return strings.TrimSpace(p[r])
		}
	}
	ip, port := digInJSONRaw(outerMapOfRaws)
	return ip + port
}

// Superugly as hell function, but it works.
func digInJSONRaw(data map[string]json.RawMessage) (string, string) {
	var ip string
	var port string
	var mapOfRaws map[string]json.RawMessage
	var listOfRaws []json.RawMessage
	for k, v := range data {
		if k == "ip" {
			json.Unmarshal(v, &ip)
			ip = ip + ":"
		} else if k == "port" {
			var p json.Number
			json.Unmarshal(v, &p)
			port = p.String()
		} else {
			err := json.Unmarshal(v, &mapOfRaws)
			if err != nil {
				err = json.Unmarshal(v, &listOfRaws)
				if ip+port == "" && len(listOfRaws) > 0 {
					json.Unmarshal(listOfRaws[0], &mapOfRaws)
					ip, port = digInJSONRaw(mapOfRaws)
				}
			}
			if ip+port == "" {
				ip, port = digInJSONRaw(mapOfRaws)
			}
		}
	}

	return ip, port
}

func truncSprintf(s string, args ...interface{}) string {
	n := strings.Count(s, `%s`)
	if n > len(args) {
		return fmt.Sprintf(s, args...)
	}
	return fmt.Sprintf(s, args[:n]...)
}
