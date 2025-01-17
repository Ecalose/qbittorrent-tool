package tool

import (
	"fengqi/qbittorrent-tool/config"
	"fengqi/qbittorrent-tool/qbittorrent"
	"fengqi/qbittorrent-tool/util"
	"fmt"
	"strings"
	"time"
)

// SeedingLimits 做种限制加强版
// 相比较于qb自带，增加根据标签、分类、关键字精确限制
func SeedingLimits(c *config.Config, torrent *qbittorrent.Torrent) {
	if !c.SeedingLimits.Enable || len(c.SeedingLimits.Rules) == 0 {
		return
	}

	action, limits := matchRule(torrent, c.SeedingLimits.Rules)
	if action == 0 {
		if !strings.Contains(torrent.State, "paused") || !c.SeedingLimits.Resume {
			return
		}
	}

	if action == 1 && strings.Contains(torrent.State, "paused") {
		return
	}

	fmt.Printf("action:%d %s\n", action, torrent.Name)
	executeAction(torrent, action, limits)
}

// 规则至少有一个生效，且生效的全部命中，action才有效，后面的规则会覆盖前面的
func matchRule(torrent *qbittorrent.Torrent, rules []config.SeedingLimitsRule) (int, *config.Limits) {
	action := 0
	var limits *config.Limits
	loc, _ := time.LoadLocation("Asia/Shanghai")

	for _, rule := range rules {
		score := 0

		// 分享率
		if rule.Ratio > 0 {
			if torrent.Ratio < rule.Ratio {
				continue
			}
			score += 1
		}

		// 做种时间，从下载完成算起
		if rule.SeedingTime > 0 {
			if torrent.CompletionOn <= 0 {
				continue
			}
			completionOn := time.Unix(int64(torrent.CompletionOn), 0).In(loc)
			deadOn := completionOn.Add(time.Minute * time.Duration(rule.SeedingTime))
			if time.Now().In(loc).Before(deadOn) {
				continue
			}
			score += 1
		}

		// 最后活动时间，上传下载等都算
		if rule.ActivityTime > 0 {
			activityOn := time.Unix(int64(torrent.LastActivity), 0).In(loc)
			deadOn := activityOn.Add(time.Minute * time.Duration(rule.ActivityTime))
			if time.Now().In(loc).Before(deadOn) {
				continue
			}
			score += 1
		}

		// 标签
		if len(rule.Tag) != 0 && torrent.Tags != "" {
			tags := strings.Split(torrent.Tags, ",")
			hit := false
		jump:
			for _, item := range rule.Tag {
				for _, item2 := range tags {
					if item == item2 {
						hit = true
						break jump
					}
				}
			}
			if !hit {
				continue
			}
			score += 1
		}

		// 分类
		if len(rule.Category) != 0 && torrent.Category != "" {
			if !util.InArray(torrent.Category, rule.Category) {
				continue
			}
			score += 1
		}

		// tracker  TODO 可能有多个tracker的情况要处理
		tracker, _ := torrent.GetTrackerHost()
		if len(rule.Tracker) != 0 && tracker != "" {
			if !util.InArray(tracker, rule.Tracker) {
				continue
			}
			score += 1
		}

		// 做种数大于
		if rule.SeedsGt > 0 {
			if torrent.NumComplete < rule.SeedsGt {
				continue
			}
			score += 1
		}

		// 做种数小于
		if rule.SeedsLt > 0 {
			if torrent.NumComplete > rule.SeedsLt {
				continue
			}
			score += 1
		}

		// 关键字
		if len(rule.Keyword) != 0 {
			if !util.ContainsArray(torrent.Name, rule.Keyword) {
				continue
			}
			score += 1
		}

		if score > 0 {
			action = rule.Action
		}
		if action == 0 && limits == nil {
			limits = rule.Limits
		}
	}

	return action, limits
}

func executeAction(torrent *qbittorrent.Torrent, action int, limits *config.Limits) {
	switch action {
	case 0:
		_ = qbittorrent.Api.ResumeTorrents(torrent.Hash)
		if limits == nil {
			break
		}
		if limits.Download != torrent.DlLimit {
			_ = qbittorrent.Api.SetDownloadLimit(torrent.Hash, limits.Download)
		}
		if limits.Upload != torrent.UpLimit {
			_ = qbittorrent.Api.SetUploadLimit(torrent.Hash, limits.Download)
		}

		flag := false
		radio := torrent.RatioLimit
		if limits.Ratio != torrent.RatioLimit {
			flag = true
			radio = limits.Ratio
		}
		seedingTimeLimit := torrent.SeedingTimeLimit
		if limits.SeedingTime != torrent.SeedingTimeLimit {
			flag = true
			seedingTimeLimit = limits.SeedingTime
		}
		inactiveSeedingTimeLimit := torrent.InactiveSeedingTimeLimit
		if limits.InactiveSeedingTime != torrent.InactiveSeedingTimeLimit {
			flag = true
			inactiveSeedingTimeLimit = limits.InactiveSeedingTime
		}
		if flag {
			_ = qbittorrent.Api.SetShareLimit(torrent.Hash, radio, seedingTimeLimit, inactiveSeedingTimeLimit)
		}

		break

	case 1:
		_ = qbittorrent.Api.PauseTorrents(torrent.Hash)
		break

	case 2:
		_ = qbittorrent.Api.DeleteTorrents(torrent.Hash, false)
		break

	case 3:
		_ = qbittorrent.Api.DeleteTorrents(torrent.Hash, true)
		break

	case 4:
		_ = qbittorrent.Api.SetSuperSeeding(torrent.Hash, true)
		break
	}
}
