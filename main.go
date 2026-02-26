package main

import (
	"cfddns/cmd"
)

// 版本信息，通過編譯時註入
var (
	Version   = "dev"     // 版本號
	BuildTime = "unknown" // 編譯時間
)

func main() {
	// 設置版本信息
	cmd.SetVersionInfo(Version, BuildTime)
	cmd.Execute()
}
