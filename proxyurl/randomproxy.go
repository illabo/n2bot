package proxyurl

import (
	"io/ioutil"
	"net/http"
	"net/url"
)

// NewTransport creates new instance of proxiedTransport to later inject into http.Client
// implementing proxy url refresh through proxyurl service.
func NewTransport(cfg *Config) *proxiedTransport {
	return &proxiedTransport{
		cfg.ProxiesSource,
		nil,
		nil,
	}
}

type proxiedTransport struct {
	proxySourceAddr string
	hostTransport   *http.Transport
	lastAcqProxy    *url.URL
}

func (prt *proxiedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if prt.lastAcqProxy == nil {
		prt.refreshProxy()
	}
	res, err := prt.hostTransport.RoundTrip(req)
	if err != nil {
		prt.refreshProxy()
	}
	return res, err
}

func (prt *proxiedTransport) InjectIntoClient(c *http.Client) {
	var t *http.Transport
	if c.Transport == nil {
		t = http.DefaultTransport.(*http.Transport).Clone()
	} else {
		t = c.Transport.(*http.Transport)
	}
	t.Proxy = prt.getProxyURL
	prt.hostTransport = t
	c.Transport = prt
}

func (prt *proxiedTransport) refreshProxy() {
	res, err := http.Get(prt.proxySourceAddr)
	if err != nil {
		prt.lastAcqProxy = nil
		return
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		prt.lastAcqProxy = nil
		return
	}
	purl, err := url.Parse(string(b))
	if err != nil {
		prt.lastAcqProxy = nil
		return
	}
	prt.lastAcqProxy = purl
}

func (prt *proxiedTransport) getProxyURL(_ *http.Request) (*url.URL, error) {
	return prt.lastAcqProxy, nil
}
