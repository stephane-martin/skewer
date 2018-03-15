package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/olivere/elastic"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/utils/es"
)

var shards uint
var replicas uint
var check bool
var refreshInterval int
var url string
var test bool
var username string
var password string
var indexName string
var delete bool

// createElasticsearchIndexCmd represents the createElasticsearchIndex command
var createElasticsearchIndexCmd = &cobra.Command{
	Use:   "create-elasticsearch-index",
	Short: "Create the index in Elasticsearch and the appropriate mapping to store logs",
	Long: `With the Elasticsearch destination, the collected logs are sent to some
	Elasticsearch index. The command creates this index and a mapping to store
	the logs.`,
	Run: func(cmd *cobra.Command, args []string) {
		optionsBody := es.NewOpts(shards, replicas, check, time.Duration(refreshInterval)*time.Second).Marshal()
		if test {
			fmt.Fprintln(os.Stdout, optionsBody)
			os.Exit(0)
		}
		logger := log15.New()
		logger.SetHandler(log15.StderrHandler)

		elasticOpts := []elastic.ClientOptionFunc{}
		elasticOpts = append(elasticOpts, elastic.SetURL(url))
		elasticOpts = append(elasticOpts, elastic.SetErrorLog(&es.ESLogger{Logger: logger}))

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
	createElasticsearchIndexCmd.Flags().UintVarP(&shards, "shards", "s", 1, "number of shards for the index")
	createElasticsearchIndexCmd.Flags().UintVarP(&replicas, "replicas", "r", 0, "number of replicas for the index")
	createElasticsearchIndexCmd.Flags().BoolVarP(&check, "check", "c", false, "whether to check the index on startup")
	createElasticsearchIndexCmd.Flags().IntVarP(&refreshInterval, "refresh", "f", 1, "refresh interval in seconds")
	createElasticsearchIndexCmd.Flags().StringVarP(&url, "url", "u", "http://127.0.0.1:9200", "elasticsearch url")
	createElasticsearchIndexCmd.Flags().StringVarP(&username, "username", "", "", "username for HTTP Basic Auth")
	createElasticsearchIndexCmd.Flags().StringVarP(&password, "password", "", "", "password for HTTP Basic Auth")
	createElasticsearchIndexCmd.Flags().BoolVarP(&test, "test", "t", false, "only display the configuration body")
	createElasticsearchIndexCmd.Flags().StringVarP(&indexName, "name", "n", "skewer", "elasticsearch index name")
	createElasticsearchIndexCmd.Flags().BoolVarP(&delete, "delete", "", false, "delete index instead of create")
}
