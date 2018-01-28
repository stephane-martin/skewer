package es

import (
	"net/http"

	"github.com/inconshreveable/log15"
	"github.com/olivere/elastic"
	"github.com/stephane-martin/skewer/conf"
)

func GetClient(config conf.ElasticDestConfig, httpClient *http.Client, logger log15.Logger) (c *elastic.Client, err error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	elasticOpts := []elastic.ClientOptionFunc{}
	elasticOpts = append(elasticOpts, elastic.SetURL(config.URLs...))
	elasticOpts = append(elasticOpts, elastic.SetHttpClient(httpClient))
	if logger != nil {
		elasticOpts = append(elasticOpts, elastic.SetErrorLog(&ESLogger{Logger: logger}))
	}
	elasticOpts = append(elasticOpts, elastic.SetHealthcheck(config.HealthCheck))
	elasticOpts = append(elasticOpts, elastic.SetSniff(config.Sniffing))
	if config.HealthCheck {
		elasticOpts = append(elasticOpts, elastic.SetHealthcheckTimeout(config.HealthCheckTimeout))
		elasticOpts = append(elasticOpts, elastic.SetHealthcheckTimeoutStartup(config.HealthCheckTimeoutStartup))
		elasticOpts = append(elasticOpts, elastic.SetHealthcheckInterval(config.HealthCheckInterval))
	}

	if config.TLSEnabled {
		elasticOpts = append(elasticOpts, elastic.SetScheme("https"))
	}
	if len(config.Username) > 0 && len(config.Password) > 0 {
		elasticOpts = append(elasticOpts, elastic.SetBasicAuth(config.Username, config.Password))
	}

	c, err = elastic.NewClient(elasticOpts...)
	if err != nil {
		return nil, err
	}
	return c, nil
}
