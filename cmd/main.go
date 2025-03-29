package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"endobit.io/metal"
	"endobit.io/metal/logging"
	"endobit.io/mops"
	"endobit.io/mops/internal/handlers"
	"endobit.io/mops/middleware"
)

var version string

//	@Title		Metal Operations Server
//	@Host		localhost:8888
//	@Version	1
//	@Accept		json
//	@Produces	json, text

func main() {
	cmd := newRootCmd()
	cmd.Version = version

	if err := cmd.Execute(); err != nil {
		os.Exit(-1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		metalUser, metalPass, metalServer string
		port                              int
		logOpts                           *logging.Options
	)

	cmd := cobra.Command{
		Use:   "mopsd",
		Short: "Metal Operations Server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger, err := logOpts.NewLogger()
			if err != nil {
				return err
			}

			logger.Info("Starting Metal Operations Server", "version", cmd.Version, "port", port)

			creds := credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
				MinVersion:         tls.VersionTLS12,
			})

			metalDialer := func() (*metal.Client, error) {
				conn, err := grpc.NewClient(metalServer, grpc.WithTransportCredentials(creds))
				if err != nil {
					return nil, err
				}

				client := metal.NewClient(conn, logger.WithGroup("metal"))

				if err := client.Authorize(metalUser, metalPass); err != nil {
					return nil, err
				}

				return client, nil
			}

			reporter := handlers.Reporter{
				Logger:      logger.WithGroup("reporter"),
				MetalDialer: metalDialer,
			}

			mux := http.NewServeMux()

			chain := middleware.Chain(
				middleware.Recovery(logger),
				middleware.RequestID,
				middleware.Logging(logger),
				middleware.DefaultJSON)

			mux.Handle("GET /report/{name}", chain(&reporter))

			s := &http.Server{
				ReadTimeout: time.Second * 5,
				Addr:        fmt.Sprintf(":%d", port),
				Handler:     mux,
			}

			if err := s.ListenAndServe(); err != nil {
				return err
			}

			return nil
		},
	}

	logging.DefaultJSON = true
	logOpts = logging.NewOptions(cmd.Flags())

	cmd.Flags().IntVar(&port, "port", mops.DefaultPort, "port to listen on")
	cmd.Flags().StringVar(&metalUser, "metal-user", "admin", "username for authentication")
	cmd.Flags().StringVar(&metalPass, "metal-pass", "admin", "password for authentication")
	cmd.Flags().StringVar(&metalServer, "metal", "localhost:"+strconv.Itoa(metal.DefaultPort),
		"address of the metal server")

	return &cmd
}
