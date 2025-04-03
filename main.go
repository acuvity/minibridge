package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.acuvity.ai/a3s/pkgs/bootstrap"
	"go.acuvity.ai/a3s/pkgs/conf"
	"go.acuvity.ai/a3s/pkgs/version"
	"go.acuvity.ai/minibridge/cmd"
)

func main() {

	cobra.OnInitialize(initCobra)

	rootCmd := &cobra.Command{
		Use:              "minibridge",
		Short:            "MCP production tunnel",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

			if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
				return err
			}

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}

			bootstrap.ConfigureLogger("minibridge", conf.LoggingConf{
				LogLevel:  viper.GetString("log-level"),
				LogFormat: "console",
			})

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if viper.GetBool("version") {
				fmt.Println(version.String("minibridge"))
				os.Exit(0)
			}
		},
	}
	rootCmd.PersistentFlags().String("log-level", "info", "Set the log level")

	rootCmd.AddCommand(
		cmd.Backend,
		cmd.Frontend,
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", color.RedString("error"), err)
		os.Exit(1)
	}
}
