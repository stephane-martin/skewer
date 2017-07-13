package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// makeSecretCmd represents the makeSecret command
var makeSecretCmd = &cobra.Command{
	Use:   "make-secret",
	Short: "Generate a random secret that suitable for store.secret configuration.",
	Long: `The Store is an embedded database inside skewer that is used to
temporarily persist syslog messages, before they can be forwarded to Kafka. If
you want to avoid to store syslog messages "in the clear" on disk, you can
ask for the Store to encrypt its database. For that you need to provide an
encryption secret as the store.secret parameter.

The make-secret command generates a suitable secret.`,

	Run: func(cmd *cobra.Command, args []string) {
		secretb := make([]byte, 32)
		_, err := rand.Read(secretb)
		if err != nil {
			fmt.Println("Error happened", err)
			os.Exit(-1)
		} else {
			secret := base64.URLEncoding.EncodeToString(secretb)
			fmt.Println(string(secret))
		}
	},
}

func init() {
	RootCmd.AddCommand(makeSecretCmd)
}
