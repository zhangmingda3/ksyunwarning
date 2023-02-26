package ksyunwarning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PostWarningToFeishu 向飞书发报警
func (s *Supervisor) PostWarningToFeishu(warnRule WarnRule, feishuUrl string, warnResource WarnResource, warnValue, lastPointValue float64, firstAlarmTimeStamp, warnTypeStr string, alarmTimes int) {
	urlObj, err := url.Parse(feishuUrl)
	if err != nil {
		s.fileLogger.Error("url.Parse Error:%v", err)
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		s.fileLogger.Error(" time.LoadLocation Error:", err)
	}
	var locationTime time.Time
	// 飞书富文本消息结构体
	//	message := `{"msg_type": "post",
	//    "content": {
	//		"post":{
	//			"zh_cn": {
	//				"title": "我是一个标题",
	//				"content": [
	//					[{
	//							"tag": "text",
	//							"text": "第一行 :"
	//						},
	//						{
	//							"tag": "a",
	//							"href": "http://www.feishu.cn",
	//							"text": "超链接"
	//						},
	//						{
	//							"tag": "at",
	//							"user_id": "ou_1avnmsbv3k45jnk34j5",
	//							"user_name": "tom"
	//						}
	//					],
	//					[{
	//						"tag": "img",
	//						"image_key": "img_7ea74629-9191-4176-998c-2e603c9c5e8g"
	//					}],
	//					[{
	//							"tag": "text",
	//							"text": "第二行:"
	//						},
	//						{
	//							"tag": "text",
	//							"text": "文本测试"
	//						}
	//					],
	//					[{
	//						"tag": "img",
	//						"image_key": "img_7ea74629-9191-4176-998c-2e603c9c5e8g"
	//					}]
	//				]
	//			}
	//		}
	//	}
	//}`

	var marshalBody []byte
	switch rule := warnRule.(type) {
	case Rule:
		//飞书富文本消息
		ruleTypeChinese := s.GetRuleTypeChinese(rule.rule_type)
		ruleTextChinese := s.GetRuleTextChinese(rule)
		change := ""
		if rule.rule_type == 1 {
			change = "一分钟内变化"
		}
		var resource Resource
		switch v := warnResource.(type) {
		case Resource:
			resource = v
		}
		title := fmt.Sprintf("%s： %s 监控项:%s %s值:%v\n", warnTypeStr, resource.name, rule.metric_name, change, warnValue)
		var textMSG string
		if rule.rule_type == 0 {
			loc, err := time.LoadLocation("Asia/Shanghai")
			if err != nil {
				s.fileLogger.Error(" time.LoadLocation Error:", err)
			}
			locationTime, err := time.ParseInLocation("2006-01-02 15:04:05", firstAlarmTimeStamp, loc)
			duration := fmt.Sprintf("持续时间：%s\n", time.Now().Sub(locationTime).String()) // 持续时间
			textMSG = fmt.Sprintf("时间：%s\n资源名称：%s\n监控项：%s\n当前%s值：%v\n%sUID：%s\n实例ID：%s\n地区：%s\nNameSpace：%s\n报警策略名称：%s\n策略类型：%s\n规则：统计%v分钟%s %s\n已报警次数：第%v次/最多%v次",
				firstAlarmTimeStamp, resource.name, rule.metric_name, change, warnValue, duration,
				resource.uid, resource.instance_id, s.GetRegionChinese(resource.region),
				rule.namespace, rule.alarm_name, ruleTypeChinese, rule.period, s.GetAggregateChinese(rule.aggregate), ruleTextChinese, alarmTimes, rule.max_alarm_times)
		} else if rule.rule_type == 1 {
			textMSG = fmt.Sprintf("时间：%s\n资源名称：%s\n监控项：%s\n%s值：%v\n当前值：%v\nUID：%s\n实例ID：%s\n地区：%s\nNameSpace：%s\n报警策略名称：%s\n策略类型：%s\n规则：%s",
				firstAlarmTimeStamp, resource.name, rule.metric_name, change, warnValue, lastPointValue,
				resource.uid, resource.instance_id, s.GetRegionChinese(resource.region),
				rule.namespace, rule.alarm_name, ruleTypeChinese, ruleTextChinese)
		}
		// 如果查询数数据都为空。也报一下失败
		if warnValue == -8888.8888 && lastPointValue == -8888.8888 {
			title = fmt.Sprintf("%s！ 资源类型：%s 资源名称：%s 监控项：%s 连续3次 3个周期内数据查询结果为null\n", warnTypeStr, resource.namespace, resource.name, rule.metric_name)
			textMSG = fmt.Sprintf("实例ID:%s\nUID:%s\nRegion:%s", resource.instance_id, resource.uid, s.GetRegionChinese(resource.region))
		}
		textTag := Tag{
			Tag:  "text",
			Text: textMSG,
		}
		atAllTag := Tag{
			Tag:    "at",
			UserId: "all",
		}
		imageTag := Tag{
			Tag:      "img",
			ImageKey: "img_7ea74629-9191-4176-998c-2e603c9c5e8g",
		}
		var tagLine []Tag //一行
		var tagLines [][]Tag
		tagLine = append(tagLine, textTag, atAllTag)    // 一行内容
		imageLine := []Tag{imageTag}                    // 图片行
		tagLines = append(tagLines, tagLine, imageLine) //组合多行
		// 结构体组合
		zhcn := ZhCN{
			Title:   title,
			Content: tagLines,
		}
		post := Post{
			ZhCN: zhcn,
		}
		postOuter := PostOuter{
			Post: post,
		}
		bodyStruct := FeishuPostJsonBody{
			Msg_type: "post",
			Content:  postOuter,
		}
		//结构体组合完毕
		marshalBody, err = json.Marshal(bodyStruct)
	case IPPingLossRule:
		//飞书富文本消息
		var resource IPResource
		switch v := warnResource.(type) {
		case IPResource:
			resource = v
		}
		title := fmt.Sprintf("%s! 客户：%s  %s IP: %s 丢包率: %v%%\n", warnTypeStr, resource.customer_name, resource.name, resource.genericIPAddress, warnValue)
		locationTime, err = time.ParseInLocation("2006-01-02 15:04:05", firstAlarmTimeStamp, loc)
		duration := fmt.Sprintf("持续时间：%s\n", time.Now().Sub(locationTime).String()) // 持续时间
		textMSG := fmt.Sprintf("时间：%s\n资源名称：%s\nIP地址：%s\n丢包率：%v%%\n%s归属地：%s\n报警策略名称：%s\n 规则：连续%v分钟丢包%s超过%v%%\n当前丢包率：%v%%\n已报警次数：第%v次/最多%v次",
			firstAlarmTimeStamp, resource.name, resource.genericIPAddress, warnValue, duration, resource.location, rule.alarm_name, rule.period, s.GetAggregateChinese(rule.aggregate), rule.upper_limit, lastPointValue, alarmTimes, rule.max_alarm_times)

		textTag := Tag{
			Tag:  "text",
			Text: textMSG,
		}
		atAllTag := Tag{
			Tag:    "at",
			UserId: "all",
		}
		imageTag := Tag{
			Tag:      "img",
			ImageKey: "img_7ea74629-9191-4176-998c-2e603c9c5e8g",
		}
		var tagLine []Tag //一行
		var tagLines [][]Tag
		tagLine = append(tagLine, textTag, atAllTag)    // 一行内容
		imageLine := []Tag{imageTag}                    // 图片行
		tagLines = append(tagLines, tagLine, imageLine) //组合多行
		// 结构体组合
		zhcn := ZhCN{
			Title:   title,
			Content: tagLines,
		}
		post := Post{
			ZhCN: zhcn,
		}
		postOuter := PostOuter{
			Post: post,
		}
		bodyStruct := FeishuPostJsonBody{
			Msg_type: "post",
			Content:  postOuter,
		}
		//结构体组合完毕
		marshalBody, err = json.Marshal(bodyStruct)
	case *HttpUrlRule:
		var httpCode string
		if warnValue == -8888 {
			httpCode = "timeout"
		} else {
			httpCode = strconv.Itoa(int(warnValue))
		}
		healthy := "不健康"
		if warnTypeStr == "警报解除" {
			healthy = "恢复健康！"
		}
		title := fmt.Sprintf("%s! HTTP：%s   %s 状态码: %s\n", warnTypeStr, rule.url_name, healthy, httpCode)
		textMSG := fmt.Sprintf("时间：%s\n资源名称：%s\n地址：%s\n状态码：%v\n检测间隔：%v秒\n健康阈值：%v次\n不健康阈值：%v次\n已报警次数：第%v次/最多%v次\n",
			firstAlarmTimeStamp, rule.url_name, rule.url, httpCode, rule.test_interval_second, rule.healthy_times, rule.unhealthy_times, alarmTimes, rule.max_alarm_times)
		//title := fmt.Sprintf("%s! 客户：%s  %s IP: %s 丢包率: %v%%\n", warnTypeStr, resource.customer_name, resource.name, resource.genericIPAddress, warnValue)
		//locationTime, err = time.ParseInLocation("2006-01-02 15:04:05", timeStamp, loc)
		//duration := fmt.Sprintf("持续时间：%s\n", time.Now().Sub(locationTime).String()) // 持续时间
		//textMSG := fmt.Sprintf("时间：%s\n资源名称：%s\nIP地址：%s\n丢包率：%v%%\n%s归属地：%s\n报警策略名称：%s\n 规则：连续%v分钟丢包%s超过%v%%\n当前丢包率：%v%%\n已报警次数：第%v次/最多%v次",
		//	timeStamp, resource.name, resource.genericIPAddress, warnValue, duration, resource.location, rule.alarm_name, rule.period, s.GetAggregateChinese(rule.aggregate), rule.upper_limit, lastPointValue, alarmTimes, rule.max_alarm_times)

		textTag := Tag{
			Tag:  "text",
			Text: textMSG,
		}
		atAllTag := Tag{
			Tag:    "at",
			UserId: "all",
		}
		imageTag := Tag{
			Tag:      "img",
			ImageKey: "img_7ea74629-9191-4176-998c-2e603c9c5e8g",
		}
		var tagLine []Tag //一行
		var tagLines [][]Tag
		tagLine = append(tagLine, textTag, atAllTag)    // 一行内容
		imageLine := []Tag{imageTag}                    // 图片行
		tagLines = append(tagLines, tagLine, imageLine) //组合多行
		// 结构体组合
		zhcn := ZhCN{
			Title:   title,
			Content: tagLines,
		}
		post := Post{
			ZhCN: zhcn,
		}
		postOuter := PostOuter{
			Post: post,
		}
		bodyStruct := FeishuPostJsonBody{
			Msg_type: "post",
			Content:  postOuter,
		}
		//结构体组合完毕
		marshalBody, err = json.Marshal(bodyStruct)
	case Announcement:
		var title string
		switch alarmTimes {
		case 1:
			title = "割接/升级/变更通知：1小时后有如下升级或变更\n"
		case 13:
			title = "割接/升级/变更通知：13小时后有如下升级或变更\n"
		case 72:
			title = "割接/升级/变更通知：近日有如下升级或变更\n"

		}
		textMSG := rule.content
		textTag := Tag{
			Tag:  "text",
			Text: textMSG,
		}
		atAllTag := Tag{
			Tag:    "at",
			UserId: "all",
		}
		imageTag := Tag{
			Tag:      "img",
			ImageKey: "img_7ea74629-9191-4176-998c-2e603c9c5e8g",
		}
		var tagLine []Tag //一行
		var tagLines [][]Tag
		tagLine = append(tagLine, textTag, atAllTag)    // 一行内容
		imageLine := []Tag{imageTag}                    // 图片行
		tagLines = append(tagLines, tagLine, imageLine) //组合多行
		// 结构体组合
		zhcn := ZhCN{
			Title:   title,
			Content: tagLines,
		}
		post := Post{
			ZhCN: zhcn,
		}
		postOuter := PostOuter{
			Post: post,
		}
		bodyStruct := FeishuPostJsonBody{
			Msg_type: "post",
			Content:  postOuter,
		}
		//结构体组合完毕
		marshalBody, err = json.Marshal(bodyStruct)
	}
	//bbb := []byte(message)/////////////////发送飞书报警///////////////////////////////
	request, err := http.NewRequest("POST", urlObj.String(), bytes.NewBuffer(marshalBody))
	if err != nil {
		s.fileLogger.Error("Fei shu http.NewRequest Error:%v", err)
	}
	defer request.Body.Close()
	request.Header.Set("Content-Type", "application/json")
	var r *http.Response
	r, err = http.DefaultClient.Do(request)
	if err != nil {
		s.fileLogger.Error("Feishu http.DefaultClient.Do Error:%v", err)
	}
	var recvBytes []byte
	if r.Body != nil {
		recvBytes, err = io.ReadAll(r.Body)
		if err != nil {
			if err != nil {
				s.fileLogger.Error("Read Feishu r.Body Error: %v", err)
			}
		}
		r.Body.Close()
	}
	bodyStr := string(recvBytes)
	if r.StatusCode != 200 {
		s.fileLogger.Error("向飞书%s 请求返回status:%s, code: %d , body: %s", feishuUrl, r.Status, r.StatusCode, bodyStr)
	} else {
		s.fileLogger.Info("向飞书%s 请求返回status:%s, code: %d , body: %s", feishuUrl, r.Status, r.StatusCode, bodyStr)
	}
}

// PostWarningToWechat 向微信发报警
func (s *Supervisor) PostWarningToWechat(warnRule WarnRule, feishuUrl string, warnResource WarnResource, warnValue, lastPointValue float64, firstAlarmTimeStamp, warnTypeStr string, alarmTimes int) {
	parsedUrl, err := url.Parse(feishuUrl)
	if err != nil {
		s.fileLogger.Error("url.Parse Error:%v", err)
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		s.fileLogger.Error(" time.LoadLocation Error:", err)
	}
	warnColor := "red"
	if warnTypeStr == "警报解除" {
		warnColor = "green"
	}
	var locationTime time.Time
	//要发送的字节
	var marshalBody []byte
	switch rule := warnRule.(type) {
	case Rule:
		ruleTypeChinese := s.GetRuleTypeChinese(rule.rule_type)
		ruleTextChinese := s.GetRuleTextChinese(rule)
		change := ""
		if rule.rule_type == 1 {
			change = "一分钟内变化"
		}
		var resource Resource
		switch v := warnResource.(type) {
		case Resource:
			resource = v
		}
		title := fmt.Sprintf("%s： %s 监控项:%s %s值:%v\n", warnTypeStr, resource.name, rule.metric_name, change, warnValue)
		var textMSG string
		if rule.rule_type == 0 {
			locationTime, err = time.ParseInLocation("2006-01-02 15:04:05", firstAlarmTimeStamp, loc)
			duration := fmt.Sprintf("持续时间：%s\n", time.Now().Sub(locationTime).String()) // 持续时间
			textMSG = fmt.Sprintf("时间：%s\n资源名称：%s\n监控项：%s\n当前%s值：%v\n%sUID：%s\n实例ID：%s\n地区：%s\nNameSpace：%s\n报警策略名称：%s\n策略类型：%s\n规则：统计%v分钟%s %s\n已报警次数：第%v次/最多%v次",
				firstAlarmTimeStamp, resource.name, rule.metric_name, change, warnValue, duration,
				resource.uid, resource.instance_id, s.GetRegionChinese(resource.region),
				rule.namespace, rule.alarm_name, ruleTypeChinese, rule.period, s.GetAggregateChinese(rule.aggregate), ruleTextChinese, alarmTimes, rule.max_alarm_times)
		} else if rule.rule_type == 1 {
			textMSG = fmt.Sprintf("时间：%s\n资源名称：%s\n监控项：%s\n%s值：%v\n当前值：%v\nUID：%s\n实例ID：%s\n地区：%s\nNameSpace：%s\n报警策略名称：%s\n策略类型：%s\n规则：%s",
				firstAlarmTimeStamp, resource.name, rule.metric_name, change, warnValue, lastPointValue,
				resource.uid, resource.instance_id, s.GetRegionChinese(resource.region),
				rule.namespace, rule.alarm_name, ruleTypeChinese, ruleTextChinese)
		}
		// 如果查询数数据都为空。也报一下失败
		if warnValue == -8888.8888 && lastPointValue == -8888.8888 {
			title = fmt.Sprintf("%s！ 资源类型：%s 资源名称：%s 监控项：%s 连续3次 3个周期内数据查询结果为null\n", warnTypeStr, resource.namespace, resource.name, rule.metric_name)
			textMSG = fmt.Sprintf("实例ID:%s\nUID:%s\nRegion:%s", resource.instance_id, resource.uid, s.GetRegionChinese(resource.region))
		}
		colorTitle := "<font color=\"" + warnColor + "\">" + title + "</font>"
		markDown := WechatMarkDown{Content: colorTitle + textMSG}
		weChatData := WechatData{MsgType: "markdown",
			MarkDown: markDown}
		marshalBody, err = json.Marshal(weChatData)
	case IPPingLossRule:
		var resource IPResource
		switch v := warnResource.(type) {
		case IPResource:
			resource = v
		}
		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			s.fileLogger.Error(" time.LoadLocation Error:", err)
		}
		locationTime, err := time.ParseInLocation("2006-01-02 15:04:05", firstAlarmTimeStamp, loc)
		duration := fmt.Sprintf("持续时间：%s\n", time.Now().Sub(locationTime).String()) // 持续时间
		title := fmt.Sprintf("%s! 客户：%s  %s IP: %s 丢包率: %v%%\n", warnTypeStr, resource.customer_name, resource.name, resource.genericIPAddress, warnValue)
		textMSG := fmt.Sprintf("<font size=\"18\" face=\"verdana\">时间：%s\n资源名称：%s\nIP地址：%s\n丢包率：%v%%\n归属地：%s\n%s</font>报警策略名称：%s\n 规则：连续%v分钟丢包%s超过%v%%\n已报警次数：第%v次/最多%v次",
			firstAlarmTimeStamp, resource.name, resource.genericIPAddress, warnValue, resource.location, duration, rule.alarm_name, rule.period, s.GetAggregateChinese(rule.aggregate), rule.upper_limit, alarmTimes, rule.max_alarm_times)
		colorTitle := "<font color=\"" + warnColor + "\">" + title + "</font>"
		markDown := WechatMarkDown{Content: colorTitle + textMSG}
		weChatData := WechatData{MsgType: "markdown",
			MarkDown: markDown}
		marshalBody, err = json.Marshal(weChatData)
	case *HttpUrlRule:
		//var resource HttpUrlRule
		//switch v := warnResource.(type) {
		//case HttpUrlRule:
		//	resource = v
		//}
		//
		healthy := "不健康"
		if warnTypeStr == "警报解除" {
			healthy = "恢复健康！"
		}
		//locationTime, err = time.ParseInLocation("2006-01-02 15:04:05", timeStamp, loc)
		var httpCode string
		if warnValue == -8888 {
			httpCode = "timeout"
		} else {
			httpCode = strconv.Itoa(int(warnValue))
		}

		title := fmt.Sprintf("%s! HTTP：%s   %s 状态码: %s\n", warnTypeStr, rule.url_name, healthy, httpCode)
		textMSG := fmt.Sprintf("<font size=\"18\" face=\"verdana\">时间：%s\n资源名称：%s\n地址：%s\n状态码：%v\n检测间隔：%v秒\n健康阈值：%v次\n不健康阈值：%v次\n已报警次数：第%v次/最多%v次</font>",
			firstAlarmTimeStamp, rule.url_name, rule.url, httpCode, rule.test_interval_second, rule.healthy_times, rule.unhealthy_times, alarmTimes, rule.max_alarm_times)
		colorTitle := "<font color=\"" + warnColor + "\">" + title + "</font>"
		markDown := WechatMarkDown{Content: colorTitle + textMSG}
		weChatData := WechatData{MsgType: "markdown",
			MarkDown: markDown}
		marshalBody, err = json.Marshal(weChatData)

	case Announcement:
		warnColor = "black"
		var title string
		switch alarmTimes {
		case 1:
			title = "割接/升级/变更通知：1小时后有如下升级或变更\n"
		case 13:
			title = "割接/升级/变更通知：13小时后有如下升级或变更\n"
		case 72:
			title = "割接/升级/变更通知：近日有如下升级或变更\n"

		}
		content := "<font size=\"18\" color=\"" + warnColor + "\">" + title + "</font>" + rule.content
		markDown := WechatMarkDown{Content: content}
		weChatData := WechatData{MsgType: "markdown",
			MarkDown: markDown}
		marshalBody, err = json.Marshal(weChatData)

	}

	////////////////////////////////////////////////////////////
	//发送请求
	//bbb := []byte(message)
	request, err := http.NewRequest("POST", parsedUrl.String(), bytes.NewBuffer(marshalBody))
	if err != nil {
		s.fileLogger.Error("Wechat http.NewRequest Error:%v", err)
	}
	defer request.Body.Close()
	request.Header.Set("Content-Type", "application/json")
	var response *http.Response
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		s.fileLogger.Error("Wechat http.DefaultClient.Do Error:%v", err)
	}
	var recvBytes []byte
	if response.Body != nil {
		recvBytes, err = io.ReadAll(response.Body)
		if err != nil {
			if err != nil {
				s.fileLogger.Error("Read Wechat r.Body Error: %v", err)
			}
		}
		response.Body.Close()
	}
	bodyStr := string(recvBytes)
	if response.StatusCode != 200 {
		s.fileLogger.Error("向企微信%s 请求返回status:%s, code: %d , body: %s", feishuUrl, response.Status, response.StatusCode, bodyStr)
	} else {
		s.fileLogger.Info("向企微信%s 请求返回status:%s, code: %d , body: %s", feishuUrl, response.Status, response.StatusCode, bodyStr)
	}
}

// WarningToWebhook 发送报警
func (s *Supervisor) WarningToWebhook(warnRule WarnRule, warnResource WarnResource, warnValue, lastPointValue float64, timeStamp string, warnType, alarmTimes int) {
	var warnTypeStr string
	if warnType == 1 {
		warnTypeStr = "警报"
	} else if warnType == 0 {
		warnTypeStr = "警报解除"
	}
	var ruleBondWebhooks []Webhook
	var err error
	//fmt.Println("判断是啥类型")
	switch r := warnRule.(type) {
	//不同的资源绑定的webhook表不一样
	case Rule:
		ruleBondWebhooks, err = s.GetRuleBondWebhooks(r.id)
		if err != nil {
			s.fileLogger.Error("GetRuleBondWebhook Error: %v", err)
		}
	case IPPingLossRule:
		//fmt.Println("是IPping报警规则")
		ruleBondWebhooks, err = s.GetIPPingLossRuleBondWebhooks(r.id)
		//fmt.Println("ruleBondWebhooks:", ruleBondWebhooks)
		if err != nil {
			s.fileLogger.Error("GetIPPingLossRuleBondWebhooks Error: %v", err)
		}
	case *HttpUrlRule:
		ruleBondWebhooks, err = s.GetHttpUrlRuleBondWebhook(r.id)
		//fmt.Println(ruleBondWebhooks)
		if err != nil {
			s.fileLogger.Error("GetHttpUrlRuleBondWebhook Error: %v", err)
		}
	}
	//发送报警
	for _, webhook := range ruleBondWebhooks {
		if webhook.webhook_type == 0 {
			feishuUrl := webhook.url
			s.PostWarningToFeishu(warnRule, feishuUrl, warnResource, warnValue, lastPointValue, timeStamp, warnTypeStr, alarmTimes)
		} else if webhook.webhook_type == 1 {
			wechatUrl := webhook.url
			s.PostWarningToWechat(warnRule, wechatUrl, warnResource, warnValue, lastPointValue, timeStamp, warnTypeStr, alarmTimes)
		}
	}
}
