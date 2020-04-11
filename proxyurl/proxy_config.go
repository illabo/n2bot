package proxyurl

// Config for proxy providers client package.
type Config struct {
	// Type accepts "socks5", "http" and "https".
	// If empty defaults to "socks5".
	Type string
	// ValidPeriod set in minutes.
	// Tells how long to use randomly acquired proxy before rotate.
	ValidPeriod uint
	// ExcludeCountries is the string of comma separated ISO 3166-1 alpha-2 country codes.
	ExcludeCountries string
	// ProviderAPIURLTemplate is the string template to fill with proxy type and country exclusions.
	// It would be just fmt.Sprintf'ed.
	// Some examples:
	// "http://pubproxy.com/api/proxy?type=%s&not_country=%s&post=true&https=true"
	// "https://api.getproxylist.com/proxy?protocol=%s&notCountry=%s&allowsPost=true&allowsHttps=true"
	// "https://gimmeproxy.com/api/getProxy?post=true&supportsHttps=true&protocol=%s&notCountry=%s"
	ProviderAPIURLTemplate string `toml:"urltemplate"`
	// method is http method to use calling proxy provider api. Defaults to "GET".
	Method string
	// Providertimeout is the time in seconds to wait for while proxy server ip:port would be acquired.
	// If time exceeded unproxied request would be done.
	ProviderTimeout uint
}
