package timeutil

import "time"

// 北京时间时区（UTC+8）
var BeijingLocation *time.Location

func init() {
	var err error
	BeijingLocation, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 如果加载失败，使用 FixedZone 创建 +8 时区
		BeijingLocation = time.FixedZone("CST", 8*3600)
	}
}

// FormatBeijingTime 将时间转换为北京时间并格式化为中文格式
// 格式：2006年01月02日 15时04分05秒
func FormatBeijingTime(t time.Time) string {
	if t.IsZero() {
		return "--"
	}
	bjTime := t.In(BeijingLocation)
	return bjTime.Format("2006年01月02日 15时04分05秒")
}

// FormatBeijingTimeShort 格式化为简短格式
// 格式：15时04分05秒
func FormatBeijingTimeShort(t time.Time) string {
	if t.IsZero() {
		return "--"
	}
	bjTime := t.In(BeijingLocation)
	return bjTime.Format("15时04分05秒")
}

// ToBeijingTime 转换为北京时间
func ToBeijingTime(t time.Time) time.Time {
	return t.In(BeijingLocation)
}

// NowBeijing 返回当前北京时间
func NowBeijing() time.Time {
	return time.Now().In(BeijingLocation)
}

// ParseBeijingTime 解析北京时间字符串
func ParseBeijingTime(layout, value string) (time.Time, error) {
	return time.ParseInLocation(layout, value, BeijingLocation)
}
