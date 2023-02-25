package ksyunwarning

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GetAnnouncements 获取所有公告，时间在当前之后的
func (s *Supervisor) GetAnnouncements() (announcements []Announcement) {
	now := time.Now()
	nowString := now.Format("2006-01-02 15:04:05")
	//fmt.Printf("now:%s", nowString)
	sql := "select * from `monitor01_announcement` where start_time>?;"
	rows, err := s.dbPool.Query(sql, nowString)
	if err != nil {
		s.fileLogger.Error("GetAnnouncements Error:%v", err)
	}
	for rows.Next() {
		var announcement Announcement
		err = rows.Scan(&announcement.id, &announcement.name, &announcement.start_time, &announcement.end_time, &announcement.content)
		if err != nil {
			s.fileLogger.Error("GetAnnouncements Error:%v", err)
		}
		announcements = append(announcements, announcement)
		//fmt.Println(announcement)
	}
	return
}

// GetAnnouncementBondWebhook 查询绑定webhook地址
func (s *Supervisor) GetAnnouncementBondWebhook(announcementId int) (webhooks []Webhook) {
	//获取绑定了哪些资源ID
	sqlRuleBoundResourceString := "select * from `monitor01_announcement_webhooks` where announcement_id=?;"
	boundRows, err := s.dbPool.Query(sqlRuleBoundResourceString, announcementId)
	if err != nil {
		s.fileLogger.Error("GetAnnouncementBondWebhook -->%s Error: %v", sqlRuleBoundResourceString, err)
	}
	defer boundRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	var webhooksIds []int
	for boundRows.Next() {
		var rwb AnnouncementBondWebhook
		err = boundRows.Scan(&rwb.id, &rwb.announcement_id, &rwb.webhook_id)
		if err != nil {
			s.fileLogger.Error("GetAnnouncementBondWebhook boundRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug("GetAnnouncementBondWebhook - boundRows.Scan %v", rwb)
		webhooksIds = append(webhooksIds, rwb.webhook_id)
	}
	//id 转换为字符串切片
	var idsStrSlice []string
	for _, id := range webhooksIds {
		idsStrSlice = append(idsStrSlice, strconv.Itoa(id))
	}
	//fmt.Println("实例ID切片：", idsStrSlice)
	boundSum := len(idsStrSlice)
	//fmt.Println("绑定实例个数：", boundSum)
	// 获取实例信息
	//var webhooks []Webhook
	if boundSum > 0 {
		//组合查询各个资源ID的sql语句
		queryBoundWebhooksString := "select * from `monitor01_webhook` where id="
		sqlIdsString := strings.Join(idsStrSlice, " or id=")
		queryBoundWebhooksSql := queryBoundWebhooksString + sqlIdsString + ";"
		// 执行查询资源============================///////////
		boundWebhooks, err := s.dbPool.Query(queryBoundWebhooksSql)
		defer boundWebhooks.Close() // 多行返回结果，一定要记得关闭
		if err != nil {
			s.fileLogger.Error("GetAnnouncementBondWebhook -->%s Error: %v", queryBoundWebhooksSql, err)
		}
		//获取查询结果
		for boundWebhooks.Next() {
			var webhook Webhook
			err = boundWebhooks.Scan(&webhook.id, &webhook.name, &webhook.webhook_type, &webhook.url)
			if err != nil {
				s.fileLogger.Error("GetAnnouncementBondWebhook boundResources.Scan Error: %v", err)
			}
			webhooks = append(webhooks, webhook)
			s.fileLogger.Debug("boundWebhooks.Scan %v", webhook)
		}
		s.fileLogger.Info("GetAnnouncementBondWebhook rule id: %d, resources: %v", announcementId, webhooks)
	}
	return
}

// GetAnnouncementNoticeHistory 获取通报历史
func (s *Supervisor) GetAnnouncementNoticeHistory(announcementId int) []AnnouncementNoticeHistory {
	//获取绑定了哪些资源ID
	sqlAnnouncementNoticeHistory := "select * from `monitor01_announcementnoticehistory` where announcement_id=?;"
	boundRows, err := s.dbPool.Query(sqlAnnouncementNoticeHistory, announcementId)
	if err != nil {
		s.fileLogger.Error("GetAnnouncementBondWebhook -->%s Error: %v", sqlAnnouncementNoticeHistory, err)
	}
	defer boundRows.Close() // 多行返回结果，一定要记得关闭
	var noticehistorys []AnnouncementNoticeHistory
	for boundRows.Next() {
		var anh AnnouncementNoticeHistory
		err = boundRows.Scan(&anh.id, &anh.notice_three_days_advance, &anh.notice_thirteen_hours_advance, &anh.notice_one_hours_advance,
			&anh.announcement_id, &anh.webhook_id, &anh.notice_one_hours_time, &anh.notice_thirteen_hours_time,
			&anh.notice_three_days_time)
		if err != nil {
			s.fileLogger.Error("GetAnnouncementNoticeHistory boundRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug("GetAnnouncementNoticeHistory - boundRows.Scan %v", anh)
		noticehistorys = append(noticehistorys, anh)
	}
	return noticehistorys
}

// ExistAnnouncementAndWebhook 判断是否已有记录
func (s *Supervisor) ExistAnnouncementAndWebhook(announcementNoticeHistorys []AnnouncementNoticeHistory, annID, webhookId int) bool {
	for _, announcementNoticeHistory := range announcementNoticeHistorys {
		if announcementNoticeHistory.announcement_id == annID && announcementNoticeHistory.webhook_id == webhookId {
			return true
		}
	}
	return false
}

// AddAnnouncementNoticeHistory 添加一条公告通知
func (s *Supervisor) AddAnnouncementNoticeHistory(announcementId, bondWebhookId int) {
	sqlStr := "insert into monitor01_announcementnoticehistory(announcement_id,webhook_id,notice_three_days_advance,notice_thirteen_hours_advance,notice_one_hours_advance)  values(?,?,?,?,?);"
	result, err := s.dbPool.Exec(sqlStr, announcementId, bondWebhookId, 0, 0, 0)
	if err != nil {
		s.fileLogger.Error("AddAnnouncementNoticeHistory Error:%v", err)
	}
	lineSum, err := result.LastInsertId()
	if err != nil {
		s.fileLogger.Error("获取受影响行错误：%v", err)
	}
	s.fileLogger.Debug("向monitor01_announcementnoticehistory插入了 %v行数据", lineSum)
}

// QueryOneAnnouncementNoticeHistory 查一条数据
func (s *Supervisor) QueryOneAnnouncementNoticeHistory(announcementId, bondWebhookId int) AnnouncementNoticeHistory {
	sqlStr := "select * from `monitor01_announcementnoticehistory` where announcement_id=? and webhook_id=?;"
	row := s.dbPool.QueryRow(sqlStr, announcementId, bondWebhookId)
	var noticeHistory AnnouncementNoticeHistory
	err := row.Scan(&noticeHistory.id, &noticeHistory.notice_three_days_advance, &noticeHistory.notice_thirteen_hours_advance,
		&noticeHistory.notice_one_hours_advance, &noticeHistory.announcement_id, &noticeHistory.webhook_id,
		&noticeHistory.notice_one_hours_time, &noticeHistory.notice_thirteen_hours_time,
		&noticeHistory.notice_three_days_time)
	if err != nil {
		s.fileLogger.Error("rowObj.Scan Failed: %v", err)
	}
	return noticeHistory
}

// UpdateNoticeHistory 更新历史
func (s *Supervisor) UpdateNoticeHistory(noticeHistoryId int, data map[string]string) {
	set := "set "
	var kv []string
	for key, value := range data {
		//fmt.Println(key, value)
		kv = append(kv, key+"=\""+value+"\"")
	}
	set = set + strings.Join(kv, ",")
	//fmt.Println(set)
	s.fileLogger.Debug(set)
	sql := fmt.Sprintf("update monitor01_announcementnoticehistory %s where id=?;", set)
	result, err := s.dbPool.Exec(sql, noticeHistoryId)
	if err != nil {
		s.fileLogger.Error("UpdateNoticeHistory Exec Error:%v", err)
	}
	_, err = result.RowsAffected()
	if err != nil {
		s.fileLogger.Error("UpdateNoticeHistory RowsAffected Error:%v", err)
	} else {
		s.fileLogger.Info("UpdateNoticeHistory Successful noticeHistoryId：%v", noticeHistoryId)
	}
}

// StartNoticeAnnouncement 开始扫描所有公告
func (s *Supervisor) StartNoticeAnnouncement(flushMin int) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		s.fileLogger.Error("time.LoadLocation(\"Asia/Shanghai\") Error:%v", err)
	}

	for {
		announcements := s.GetAnnouncements()
		now := time.Now()
		nowStr := now.Format("2006-01-02 15:04:05")
		for _, announcement := range announcements {
			startTime, _ := time.ParseInLocation("2006-01-02 15:04:05", announcement.start_time, loc)
			//startTime, err := time.Parse("2006-01-02 15:04:05", announcement.start_time)
			if err != nil {
				s.fileLogger.Error("time.Parse Error:%v", err)
			}
			timeDurationHours := startTime.Sub(now).Hours()
			bondWebhooks := s.GetAnnouncementBondWebhook(announcement.id)
			announcementNoticeHistorys := s.GetAnnouncementNoticeHistory(announcement.id)
			//fmt.Println(announcementNoticeHistorys)
			for _, bondWebhook := range bondWebhooks {
				//判断是否已有记录，没有就增加一条
				if !s.ExistAnnouncementAndWebhook(announcementNoticeHistorys, announcement.id, bondWebhook.id) {
					//	增加一条通知历史记录
					s.AddAnnouncementNoticeHistory(announcement.id, bondWebhook.id)
				}
				//
				noticeHistory := s.QueryOneAnnouncementNoticeHistory(announcement.id, bondWebhook.id)
				//fmt.Println(noticeHistory)
				if timeDurationHours < 1 && noticeHistory.notice_one_hours_advance == 0 {
					// 1小时内割接通知
					// 发通知后更新数据库
					if bondWebhook.webhook_type == 0 {
						feishuUrl := bondWebhook.url
						s.PostWarningToFeishu(announcement, feishuUrl, announcement, timeDurationHours, timeDurationHours, nowStr, "通知", 1)
					} else if bondWebhook.webhook_type == 1 {
						wechatUrl := bondWebhook.url
						s.PostWarningToWechat(announcement, wechatUrl, announcement, timeDurationHours, timeDurationHours, nowStr, "通知", 1)
					}
					data := make(map[string]string)
					data["notice_one_hours_advance"] = "1"
					data["notice_one_hours_time"] = nowStr
					// 更新都数据库
					s.UpdateNoticeHistory(noticeHistory.id, data)
				} else if timeDurationHours < 13 && noticeHistory.notice_thirteen_hours_advance == 0 {
					//13小时内割接通知
					if bondWebhook.webhook_type == 0 {
						feishuUrl := bondWebhook.url
						s.PostWarningToFeishu(announcement, feishuUrl, announcement, timeDurationHours, timeDurationHours, nowStr, "通知", 13)
					} else if bondWebhook.webhook_type == 1 {
						wechatUrl := bondWebhook.url
						s.PostWarningToWechat(announcement, wechatUrl, announcement, timeDurationHours, timeDurationHours, nowStr, "通知", 13)
					}
					data := make(map[string]string)
					data["notice_thirteen_hours_advance"] = "1"
					data["notice_thirteen_hours_time"] = nowStr
					// 更新都数据库
					s.UpdateNoticeHistory(noticeHistory.id, data)
				} else if timeDurationHours < 86 && noticeHistory.notice_three_days_advance == 0 {
					//	72小时内割接通知
					if bondWebhook.webhook_type == 0 {
						feishuUrl := bondWebhook.url
						s.PostWarningToFeishu(announcement, feishuUrl, announcement, timeDurationHours, timeDurationHours, nowStr, "通知", 72)
					} else if bondWebhook.webhook_type == 1 {
						wechatUrl := bondWebhook.url
						s.PostWarningToWechat(announcement, wechatUrl, announcement, timeDurationHours, timeDurationHours, nowStr, "通知", 72)
					}
					data := make(map[string]string)
					data["notice_three_days_advance"] = "1"
					data["notice_three_days_time"] = nowStr
					// 更新都数据库
					s.UpdateNoticeHistory(noticeHistory.id, data)
				}
			}

			//fmt.Printf("举例：%s 开始 还有%v小时", announcement.name, timeDurationHours)
		}
		time.Sleep(time.Second * time.Duration(flushMin) * 60)
	}
}
