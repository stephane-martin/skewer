package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/olivere/elastic"
	"github.com/spf13/cobra"
)

var shards int
var replicas int
var check bool
var refreshInterval int
var url string
var test bool
var username string
var password string
var indexName string
var delete bool

type opts struct {
	S settings `json:"settings"`
	M mappings `json:"mappings"`
}

func newOpts(shards int, replicas int, c bool, r time.Duration) opts {
	return opts{
		S: newSettings(shards, replicas, c, r),
		M: newMappings(),
	}
}

func (o *opts) Marshal() string {
	b, err := json.Marshal(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
	return string(b)
}

type settings struct {
	Shards   int           `json:"number_of_shards"`
	Replicas int           `json:"number_of_replicas"`
	Shard    shardSettings `json:"shard"`
	Refresh  string        `json:"refresh_interval"`
}

type shardSettings struct {
	Check bool `json:"check_on_startup"`
}

func newSettings(s int, r int, c bool, refr time.Duration) settings {
	return settings{
		Shards:   s,
		Replicas: r,
		Shard:    shardSettings{Check: c},
		Refresh:  strconv.FormatInt(int64(refr.Seconds()), 10) + "s",
	}
}

type mappings struct {
	Mtyp messageType `json:"syslogmsg"`
}

func newMappings() mappings {
	return mappings{
		Mtyp: messageType{
			Properties: newMessageFields(),
			DynamicTemplates: [](map[string]dynamicTemplate){
				map[string]dynamicTemplate{"detect_dates": dynamicDatetime()},
				map[string]dynamicTemplate{"detect_ns": dynamicNanoSeconds()},
				map[string]dynamicTemplate{"detect_bytes": dynamicBytes()},
				map[string]dynamicTemplate{"detect_pid": dynamicPID()},
				map[string]dynamicTemplate{"strings_as_keywords": stringsAsKeywords()},
			},
		},
	}
}

type messageType struct {
	Properties       messageFields                  `json:"properties"`
	DynamicTemplates [](map[string]dynamicTemplate) `json:"dynamic_templates"`
}

type messageFields struct {
	Facility   strField  `json:"facility"`
	Severity   strField  `json:"severity"`
	Hostname   strField  `json:"keyword"`
	Appname    strField  `json:"appname"`
	Procid     strField  `json:"procid"`
	Msgid      strField  `json:"msgid"`
	Message    strField  `json:"message"`
	Treported  dateField `json:"time_reported"`
	Tgenerated dateField `json:"time_generated"`
}

func newMessageFields() messageFields {
	return messageFields{
		Facility:   newKeyword(),
		Severity:   newKeyword(),
		Hostname:   newKeyword(),
		Appname:    newKeyword(),
		Procid:     newKeyword(),
		Msgid:      newKeyword(),
		Message:    newTextField(),
		Treported:  newDateField(),
		Tgenerated: newDateField(),
	}
}

type AnyField interface{}

type strField struct {
	Typ   string `json:"type"`
	Store bool   `json:"store"`
}

func newKeyword() strField {
	return strField{
		Typ:   "keyword",
		Store: true,
	}
}

func newTextField() strField {
	return strField{
		Typ:   "text",
		Store: true,
	}
}

type dateField struct {
	Typ    string `json:"type"`
	Format string `json:"format"`
	Store  bool   `json:"store"`
}

func newDateField() dateField {
	return dateField{
		Typ:    "date",
		Format: "strict_date_time_no_millis||strict_date_time",
		Store:  true,
	}
}

type longField struct {
	Typ   string `json:"type"`
	Store bool   `json:"store"`
}

func newLongField() longField {
	return longField{
		Typ:   "long",
		Store: true,
	}
}

type dynamicTemplate struct {
	MatchMappingType string   `json:"match_mapping_type,omitempty"`
	Match            string   `json:"match,omitempty"`
	Mapping          AnyField `json:"mapping"`
}

func stringsAsKeywords() dynamicTemplate {
	return dynamicTemplate{
		MatchMappingType: "string",
		Mapping:          newKeyword(),
	}
}

func dynamicDatetime() dynamicTemplate {
	return dynamicTemplate{
		Match:   "*_datetime",
		Mapping: newDateField(),
	}
}

func dynamicNanoSeconds() dynamicTemplate {
	return dynamicTemplate{
		Match:   "*_ns",
		Mapping: newLongField(),
	}
}

func dynamicBytes() dynamicTemplate {
	return dynamicTemplate{
		Match:   "*_bytes",
		Mapping: newLongField(),
	}
}

func dynamicPID() dynamicTemplate {
	return dynamicTemplate{
		Match:   "*_pid",
		Mapping: newLongField(),
	}
}

type myLogger struct {
	logger log15.Logger
}

func (l *myLogger) Printf(format string, v ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, v...))
}

// createElasticsearchIndexCmd represents the createElasticsearchIndex command
var createElasticsearchIndexCmd = &cobra.Command{
	Use:   "create-elasticsearch-index",
	Short: "Create the index in Elasticsearch and the appropriate mapping to store logs",
	Long: `With the Elasticsearch destination, the collected logs are sent to some
	Elasticsearch index. The command creates this index and a mapping to store
	the logs.`,
	Run: func(cmd *cobra.Command, args []string) {
		options := newOpts(shards, replicas, check, time.Duration(refreshInterval)*time.Second)
		optionsBody := options.Marshal()
		if test {
			fmt.Fprintln(os.Stdout, optionsBody)
			os.Exit(0)
		}
		logger := log15.New()
		logger.SetHandler(log15.StderrHandler)

		elasticOpts := []elastic.ClientOptionFunc{}
		elasticOpts = append(elasticOpts, elastic.SetURL(url))
		elasticOpts = append(elasticOpts, elastic.SetErrorLog(&myLogger{logger: logger}))

		if strings.HasPrefix(url, "https://") {
			elasticOpts = append(elasticOpts, elastic.SetScheme("https"))
		}
		if len(username) > 0 && len(password) > 0 {
			elasticOpts = append(elasticOpts, elastic.SetBasicAuth(username, password))
		}

		client, err := elastic.NewClient(elasticOpts...)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
		version, err := client.ElasticsearchVersion(url)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
		fmt.Fprintln(os.Stdout, "Elasticsearch version:", version)
		ctx := context.Background()
		exists, err := client.IndexExists(indexName).Do(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
		if !exists && !delete {
			createIndex, err := client.CreateIndex(indexName).BodyString(optionsBody).Do(ctx)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			if !createIndex.Acknowledged {
				fmt.Fprintln(os.Stderr, "Not acknowledged")
				os.Exit(-1)
			}
			fmt.Fprintln(os.Stdout, "Index created")
			os.Exit(0)
		}
		if exists && delete {
			resp, err := client.DeleteIndex(indexName).Do(ctx)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			if !resp.Acknowledged {
				fmt.Fprintln(os.Stderr, "Not acknowledged")
				os.Exit(-1)
			}
			fmt.Fprintln(os.Stdout, "Index deleted")
			os.Exit(0)
		}
		if exists && !delete {
			fmt.Fprintln(os.Stdout, "Index already exists")
		}
		if !exists && delete {
			fmt.Fprintln(os.Stdout, "Index does not exist")
		}
		os.Exit(0)
	},
}

func init() {
	RootCmd.AddCommand(createElasticsearchIndexCmd)
	createElasticsearchIndexCmd.Flags().IntVarP(&shards, "shards", "s", 1, "number of shards for the index")
	createElasticsearchIndexCmd.Flags().IntVarP(&replicas, "replicas", "r", 0, "number of replicas for the index")
	createElasticsearchIndexCmd.Flags().BoolVarP(&check, "check", "c", false, "whether to check the index on startup")
	createElasticsearchIndexCmd.Flags().IntVarP(&refreshInterval, "fresh", "f", 1, "refresh interval in seconds")
	createElasticsearchIndexCmd.Flags().StringVarP(&url, "url", "u", "http://127.0.0.1:9200", "elasticsearch url")
	createElasticsearchIndexCmd.Flags().StringVarP(&username, "username", "", "", "username for HTTP Basic Auth")
	createElasticsearchIndexCmd.Flags().StringVarP(&password, "password", "", "", "password for HTTP Basic Auth")
	createElasticsearchIndexCmd.Flags().BoolVarP(&test, "test", "t", false, "only display the configuration body")
	createElasticsearchIndexCmd.Flags().StringVarP(&indexName, "name", "n", "skewer", "elasticsearch index name")
	createElasticsearchIndexCmd.Flags().BoolVarP(&delete, "delete", "", false, "delete index instead of create")

}
