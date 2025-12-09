package domain

type Config struct {
	FQDN         string `yaml:"fqdn"`
	PrivateKey   string `yaml:"privatekey"`
	Registration string `yaml:"registration"` // open, invite, close
	SiteKey      string `yaml:"sitekey"`
	Layer        string `yaml:"layer"`
	CCID         string `yaml:"ccid"`
	CSID         string `yaml:"csid"`
}
