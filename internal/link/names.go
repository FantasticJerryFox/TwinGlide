package link

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
)

// A friendly, persistent nickname avoids broadcasting the OS account/hostname.
// The suffix distinguishes namesakes; certificates remain the device identity.
func randomDeviceName() (string, error) {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate device nickname: %w", err)
	}
	adjectives := [...]string{"轻快", "自在", "晴朗", "悠游", "灵巧", "温暖", "清风", "星光", "晨曦", "远行", "明亮", "安静", "活力", "云间", "微光", "夏日"}
	animals := [...]string{"海豚", "白鲸", "水獭", "飞鸟", "松鼠", "鲸鱼", "小鹿", "海鸥", "雪狐", "熊猫", "企鹅", "雨燕", "海豹", "月兔", "白鹭", "小猫"}
	suffix := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[2:])[:4]
	return adjectives[int(b[0])%len(adjectives)] + animals[int(b[1])%len(animals)] + "-" + suffix, nil
}
