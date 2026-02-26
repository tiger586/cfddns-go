package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version   string
	buildTime string
)

func SetVersionInfo(v, t string) {
	version = v
	buildTime = t
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "顯示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Cloudflare DDNS Client\n")
		fmt.Printf("版本: %s\n", version)
		fmt.Printf("編譯時間: %s\n", buildTime)
		fmt.Printf("Go 版本: %s / %s-%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}
