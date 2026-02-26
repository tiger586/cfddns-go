package cmd

import (
	"cfddns/service"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "運行 DDNS 服務",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := getConfig()
		if err != nil {
			log.Fatalf("加載配置失敗: %v", err)
		}

		service.SetVerbose(verbose)

		ddnsService := service.NewDDNSService(cfg)

		fmt.Println("🌐 系統啟動")
		printSeparator(50)

		if err := ddnsService.Start(); err != nil {
			log.Fatalf("服務運行失敗: %v", err)
		}
	},
}
