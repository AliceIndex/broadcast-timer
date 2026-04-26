package timecode

import (
	"fmt"
	"math"
)

type Config struct {
	FPS         float64
	IsDropFrame bool
}

// FramesToTC は通算フレーム数をタイムコード文字列（HH:mm:ss:ff）に変換します
func FramesToTC(frames int64, config Config) string {
	fpsRound := int64(math.Round(config.FPS))
	
	if config.IsDropFrame && (fpsRound == 30 || fpsRound == 60) {
		dropPerMinute := int64(2)
		if fpsRound == 60 {
			dropPerMinute = 4
		}
		totalMinutes := frames / (fpsRound * 60)
		// 10分ごとの例外処理を含むドロップフレーム補正
		dropFrames := dropPerMinute * (totalMinutes - totalMinutes/10)
		frames += dropFrames
	}

	// 24時間ロールオーバー処理
	frames = frames % (fpsRound * 3600 * 24)

	f := frames % fpsRound
	s := (frames / fpsRound) % 60
	m := (frames / (fpsRound * 60)) % 60
	h := (frames / (fpsRound * 3600)) % 24

	sep := ":"
	if config.IsDropFrame {
		sep = ";"
	}

	return fmt.Sprintf("%02d:%02d:%02d%s%02d", h, m, s, sep, f)
}