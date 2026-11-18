package link

import (
	"errors"
	"net"
	"strings"
)

// Socket errors alone cannot prove that macOS privacy denied access. Explain
// both the permission and routing checks instead of reporting a peer as offline.
func connectionFailure(err error, platform string) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "no route to host") || strings.Contains(text, "network is unreachable") || strings.Contains(text, "permission denied") || strings.Contains(text, "operation not permitted") {
		if platform == "darwin" {
			return "无法访问局域网。请在 Mac 的系统设置 → 隐私与安全性 → 本地网络允许双游，更改后重新打开双游，并确认两台电脑在同一局域网。"
		}
		return "无法到达另一台电脑。请确认两台电脑在同一局域网，并允许双游通过防火墙。"
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "连接超时。请确认另一台电脑已打开双游，网络没有访客隔离，并允许双游通过防火墙。"
	}
	return "暂时无法连接。请确认另一台电脑已打开双游并允许局域网访问；已配对设备会自动重试。"
}
