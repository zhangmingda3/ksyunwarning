package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// MaxFloatValue 最小值
func MaxFloatValue(nums []float64) float64 {
	max := nums[0]
	for _, num := range nums {
		if num > max {
			max = num
		}
	}
	return max
}

// MinFloatValue 最大值
func MinFloatValue(nums []float64) float64 {
	min := nums[0]
	for _, num := range nums {
		if num < min {
			min = num
		}
	}
	return min
}

// AverageFloatValue 平均值
func AverageFloatValue(nums []float64) float64 {
	sum := float64(0)
	for _, num := range nums {
		sum += num
	}
	return sum
}

// ParseIP  解析IP地址
func (s *Supervisor) ParseIP(genericIPAddress string) (net.IP, int) {
	ip := net.ParseIP(genericIPAddress)
	if ip == nil {
		s.fileLogger.Error("ip nil.....")
	}
	for i := 0; i < len(genericIPAddress); i++ {
		switch genericIPAddress[i] {
		case '.':
			return ip, 4
		case ':':
			return ip, 6
		}
	}
	return nil, 0
}

// FpingExec ping  一次
func (s *Supervisor) FpingExec(genericIPAddress, count, interval string) (lossFloat float64) {
	lossFloat = 100 // 默认丢包100
	pingTools := "fping"
	ip, version := s.ParseIP(genericIPAddress)
	ipStr := string(ip)
	if version == 4 {
		pingTools = "fping"
	} else if version == 6 {
		pingTools = "fping6"
	}
	arg := fmt.Sprintf("%s -C%s -p%s  -b32 -t3000  %s | tail -n 1 | awk '{print $10}'", pingTools, count, interval, ipStr)
	cmd := exec.Command("/bin/bash", "-c", arg)
	stdout, err := cmd.Output() // 找出输出
	if err != nil {
		//fmt.Println(err)
		s.fileLogger.Error("fping %s error:%v", genericIPAddress, err)

	}
	result := string(stdout)
	if len(result) > 0 {
		//fmt.Println("result:", result)
		numStr := strings.TrimSpace(strings.Trim(result, "%\n"))
		//fmt.Println("numStr:", numStr)
		lossFloat, err = strconv.ParseFloat(numStr, 4)
		if err != nil {
			s.fileLogger.Error("strconv.ParseFloat Error:%v", err)
		}
		//fmt.Println("float num:", lossFloat)
	}
	return
}

// GetIPRuleBondIP 绑定的webhook
func (s *Supervisor) GetIPRuleBondIP(ipPingLossRuleId int) (ipResources []IPResource) {
	//获取绑定了哪些资源ID
	sqlRuleBoundResourceString := "select * from `monitor01_ippinglossrule_ips` where ippinglossrule_id=?;"
	boundRows, err := s.dbPool.Query(sqlRuleBoundResourceString, ipPingLossRuleId)
	if err != nil {
		s.fileLogger.Error("GetIPRuleBondIP -->%s Error: %v", sqlRuleBoundResourceString, err)
	}
	defer boundRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	var ipResourceIds []int
	for boundRows.Next() {
		var ruleIpBond IPPingLossRuleIPResourceBond
		err = boundRows.Scan(&ruleIpBond.id, &ruleIpBond.ippinglossrule_id, &ruleIpBond.ipresource_id)
		if err != nil {
			s.fileLogger.Error("GetIPRuleBondIP boundRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug(" GetIPRuleBondIP boundRows.Scan %v", ruleIpBond)
		ipResourceIds = append(ipResourceIds, ruleIpBond.ipresource_id)
	}
	//id 转换为字符串切片 , 到数据库内一次性查询所有IPResource 用
	var idsStrSlice []string //
	for _, id := range ipResourceIds {
		idsStrSlice = append(idsStrSlice, strconv.Itoa(id))
	}
	//fmt.Println("IP切片：", idsStrSlice)
	boundSum := len(idsStrSlice)

	//var IPresources []IPResource
	if boundSum > 0 {
		//组合查询各个资源ID的sql语句
		queryBoundResourceString := "select * from `monitor01_resource` where id="
		sqlIdsString := strings.Join(idsStrSlice, " or id=")
		queryBoundIPSql := queryBoundResourceString + sqlIdsString + ";"
		// 执行查询资源
		boundIPs, err := s.dbPool.Query(queryBoundIPSql)
		defer boundIPs.Close() // 多行返回结果，一定要记得关闭
		if err != nil {
			s.fileLogger.Error("GetIPRuleBondIP -->%s Error: %v", queryBoundIPSql, err)
		}
		//获取查询结果
		for boundIPs.Next() {
			var ipobj IPResource
			err = boundIPs.Scan(&ipobj.id, &ipobj.name, &ipobj.customer_name, &ipobj.uid, &ipobj.genericIPAddress, &ipobj.location)
			if err != nil {
				s.fileLogger.Error("GetIPRuleBondIP boundIPs.Scan Error: %v", err)
			}
			ipResources = append(ipResources, ipobj)
			s.fileLogger.Debug("boundIPs.Scan %v", ipobj)
		}
		s.fileLogger.Info("GetIPRuleBondIP rule id: %d, resources: %v", ipPingLossRuleId, ipResources)
	}
	return
}

// FpingToDB 发起一次ping测试值存到数据库count：ping一次的包数，interval：每个包的间隔时间,建议次方法20S执行一次
func (s *Supervisor) FpingToDB(ipResource IPResource, count, interval string) {
	value := s.FpingExec(ipResource.genericIPAddress, count, interval)
	insertSql := "insert into `monitor01_pinglossdata` (float_value, ip_resource_id) values(?,?);"
	result, err := s.dbPool.Exec(insertSql, value, ipResource.id)
	if err != nil {
		s.fileLogger.Error("FpingToDB sql : %s dbPool.Exec Error:%v", insertSql, err)
	}
	line, err := result.LastInsertId()
	if err != nil {
		s.fileLogger.Error("插入monitor01_pinglossdata数据LastInsertId失败：%v", err)
	} else {
		s.fileLogger.Debug("插入monitor01_pinglossdata行ID:%v ", line)
	}

}

// GetIPLossRuleBondWebhook 查询绑定webhook地址
func (s *Supervisor) GetIPLossRuleBondWebhook(ipRuleId int) (webhooks []Webhook, err error) {
	//获取绑定了哪些资源ID
	sqlRuleBoundResourceString := "select * from `monitor01_ippinglossrule_webhooks` where ippinglossrule_id=?;"
	boundRows, err := s.dbPool.Query(sqlRuleBoundResourceString, ipRuleId)
	if err != nil {
		s.fileLogger.Error("GetIPLossRuleBondWebhook -->%s Error: %v", sqlRuleBoundResourceString, err)
	}
	defer boundRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	var webhooksIds []int
	for boundRows.Next() {
		var rwb IPPingLossRuleWebhookBond
		err = boundRows.Scan(&rwb.id, &rwb.ippinglossrule_id, &rwb.webhook_id)
		if err != nil {
			s.fileLogger.Error("GetIPLossRuleBondWebhook boundRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug("GetIPLossRuleBondWebhook - boundRows.Scan %v", rwb)
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
			s.fileLogger.Error("GetIPLossRuleBondWebhook -->%s Error: %v", queryBoundWebhooksSql, err)
		}
		//获取查询结果
		for boundWebhooks.Next() {
			var webhook Webhook
			err = boundWebhooks.Scan(&webhook.id, &webhook.name, &webhook.webhook_type, &webhook.url)
			if err != nil {
				s.fileLogger.Error("GetIPLossRuleBondWebhook boundResources.Scan Error: %v", err)
			}
			webhooks = append(webhooks, webhook)
			s.fileLogger.Debug("boundWebhooks.Scan %v", webhook)
		}
		s.fileLogger.Info("GetIPLossRuleBondWebhook rule id: %d, resources: %v", ipRuleId, webhooks)
	}
	return
}

// GetIPPingLossRules 获取PING丢包报警规则
func (s *Supervisor) GetIPPingLossRules() (pinglossRules []IPPingLossRule, err error) {
	sqlRuleString := "select * from `monitor01_ippinglossrule`;"
	rows, err := s.dbPool.Query(sqlRuleString)
	defer rows.Close() // 多行返回结果，一定要记得关闭
	//var rules []Rule
	if err != nil {
		s.fileLogger.Error("query failed, err: %v", err)
	}
	// 多个结果中循环取值
	for rows.Next() {
		//准备查询结果的对象
		var pingLossRule IPPingLossRule
		err = rows.Scan(&pingLossRule.id, &pingLossRule.ctime,
			&pingLossRule.alarm_name, &pingLossRule.upper_limit,
			&pingLossRule.aggregate, &pingLossRule.period,
			&pingLossRule.alarm_interval, &pingLossRule.max_alarm_times)
		if err != nil {
			s.fileLogger.Error("GetIPPingLossRules rows.Scan Error: %v", err)
		}
		//fmt.Println(rule)
		pinglossRules = append(pinglossRules, pingLossRule)
	}
	return
}

// StartFpingBondIPsToDB 开始执行fping监控 go 起一个线程去跑死循环
// forIntervalSecond 多久循环一次，count: 一次ping多少个包 pinterval：每个包间隔多久
func (s *Supervisor) StartFpingBondIPsToDB(forIntervalSecond int, count, pinterval string) {
	for {
		pinglossRules, err := s.GetIPPingLossRules()
		if err != nil {
			s.fileLogger.Error("StartGetPingLoss Error: %v", err)
		}
		for _, pingLossRule := range pinglossRules {
			ips := s.GetIPRuleBondIP(pingLossRule.id)
			for _, ip := range ips {
				go s.FpingToDB(ip, count, pinterval)
			}
		}
		//
		time.Sleep(time.Second * time.Duration(forIntervalSecond))
	}
}

// GetPingLossData 获取监控数据
func (s *Supervisor) GetPingLossData(ipResource IPResource, period int) (pingLossDatas []PingLossData) {
	getPointSql := "select * from `monitor01_pinglossdata` where ctime>=? and ip_resource_id=?;"
	periodStartTime := time.Now().Add(time.Minute * time.Duration(-period)).Format("2006-01-02 13:04:05")
	pingLossRows, err := s.dbPool.Query(getPointSql, periodStartTime, ipResource.id)
	if err != nil {
		s.fileLogger.Error("GetPingLossData Error:%v", err)
	}
	for pingLossRows.Next() {
		var pingLossPoint PingLossData
		err = pingLossRows.Scan(&pingLossPoint.id, &pingLossPoint.ctime, &pingLossPoint.float_value, &pingLossPoint.ip_resource_id)
		if err != nil {
			s.fileLogger.Error("pingLossRows.Scan Error:%v", err)
		}
		pingLossDatas = append(pingLossDatas, pingLossPoint)
	}
	return
}

// CalcAggregateResult 计算聚合结果
func (s *Supervisor) CalcAggregateResult(pingLossRule IPPingLossRule, pointValues []float64) (calcResult float64) {
	switch pingLossRule.aggregate {
	case "Average":
		calcResult = AverageFloatValue(pointValues)
	case "Max":
		calcResult = MaxFloatValue(pointValues)
	case "Min":
		calcResult = MinFloatValue(pointValues)
	}
	return
}

// SearchIPPingLossAlarmList 查询告警记录
func (s *Supervisor) SearchIPPingLossAlarmList(pingLossRuleID, ipResourceID, nowStatus int) (ipPingLossAlarms []IPPingLossAlarmList) {
	//获取绑定了哪些资源ID
	queryAlarmsSql := "select * from `monitor01_ippinglossalarmlist` where alarm_rule_id=? and alarm_resource_id=? and now_status=?;"
	ipAlarmRows, err := s.dbPool.Query(queryAlarmsSql, pingLossRuleID, ipResourceID, nowStatus)
	if err != nil {
		s.fileLogger.Error("SearchIPPingLossAlarmList -->%s Error: %v", queryAlarmsSql, err)
	}
	defer ipAlarmRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	for ipAlarmRows.Next() {
		var alarm IPPingLossAlarmList
		err = ipAlarmRows.Scan(&alarm.id, &alarm.alarm_time, &alarm.last_alarm_time, &alarm.alarm_value, &alarm.recover_time,
			&alarm.hold_minutes, &alarm.now_status, &alarm.times, &alarm.alarm_resource_id, &alarm.alarm_rule_id)
		if err != nil {
			s.fileLogger.Error("SearchIPPingLossAlarmList alarmRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug("alarmRows.Scan %v", alarm)
		ipPingLossAlarms = append(ipPingLossAlarms, alarm)
	}
	return
}

// AddIPAlarm 插入IP丢包报警数据
func (s *Supervisor) AddIPAlarm(alarmTime, lastAlarm_time string, calcAggregateResult float64, nowStatus, times, alarmIPResourceID, alarmIPRuleID int) {
	sqlString := "insert into `monitor01_ippinglossalarmlist` (alarm_time,last_alarm_time,alarm_value,now_status,times,alarm_resource_id, alarm_rule_id) values(?,?,?,?,?,?,?);"
	_, err := s.dbPool.Exec(sqlString, alarmTime, lastAlarm_time, calcAggregateResult, nowStatus, times, alarmIPResourceID, alarmIPRuleID)
	if err != nil {
		s.fileLogger.Error("AddAlarm Error:%v", err)
	}
}

// UpdateExistIpAlarm 更新报警内容
func (s *Supervisor) UpdateExistIpAlarm(IpalarmID, alarmTimes int, nowStr string) {
	UpdateSql := "update `monitor01_ippinglossalarmlist` set times=?,last_alarm_time=? where id=?;"
	_, err := s.dbPool.Exec(UpdateSql, alarmTimes, nowStr, IpalarmID)
	if err != nil {
		s.fileLogger.Error("update UpdateExistIpAlarm Error:%v", err)
	}
	//fmt.Println("update UpdateExistIpAlarm: ", result)
}

// OverIPAlarmToSql 恢复时间
func (s *Supervisor) OverIPAlarmToSql(pingLossAlarmId int, nowStr string, holdMinutes, nowStatus int) {
	UpdateSql := "update `monitor01_ippinglossalarmlist` set hold_minutes=?,now_status=?,recover_time=? where id=?;"
	result, err := s.dbPool.Exec(UpdateSql, holdMinutes, nowStatus, nowStr, pingLossAlarmId)
	if err != nil {
		s.fileLogger.Error("OverAlarmToSql dbPool.Exec  Error:%v", err)
	} else {
		s.fileLogger.Debug("OverAlarmToSql result : ", result)
	}
}

// OneIPDataTest 测试一个IP
func (s *Supervisor) OneIPDataTest(pingLossRule IPPingLossRule, ip IPResource) {
	//一个IP数据对比、、、、、、、、、、、、、、、、、、、、
	var pointValues []float64
	pingLossDatas := s.GetPingLossData(ip, pingLossRule.period)
	for _, pingLossData := range pingLossDatas {
		pointValues = append(pointValues, pingLossData.float_value)
	}
	var lastPointValue float64
	if len(pointValues) > 0 {
		lastPointValue = pointValues[len(pointValues)-1]
	}

	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	calcAggregateResult := s.CalcAggregateResult(pingLossRule, pointValues)
	// 超过阈值启动判断逻辑
	if calcAggregateResult >= pingLossRule.upper_limit {
		// 查数据库里是否有报警记录：
		// 有 判断时间是否够间隔，
		// ---够： 报警：次数在已报警次数上增加1
		//1. 查数据库里是否有报警记录：
		ipAlarms := s.SearchIPPingLossAlarmList(pingLossRule.id, ip.id, 1)
		if len(ipAlarms) > 0 { //有报警未恢复
			for _, ipAlarm := range ipAlarms {
				// 数据库内时间转换为时间类型
				loc, err := time.LoadLocation("Asia/Shanghai")
				if err != nil {
					s.fileLogger.Error("time.LoadLocation(\"Asia/Shanghai\") Error:%v", err)
				}
				lastAlarmTime, err := time.ParseInLocation("2006-01-02 15:04:05", ipAlarm.last_alarm_time, loc)
				if err != nil {
					s.fileLogger.Error("time.Parse Error:%v", err)
				}
				if ipAlarm.times < pingLossRule.max_alarm_times { //判断报警次数是否超
					deltaTMin := time.Now().Sub(lastAlarmTime).Minutes()
					deltaTMinInt := int(deltaTMin)
					if deltaTMinInt > pingLossRule.alarm_interval { // 判断时间是否够间隔，
						warnType := 1 //1报警触发 0报警解除
						alarmTimes := ipAlarm.times + 1
						s.WarningToWebhook(pingLossRule, ip, calcAggregateResult, lastPointValue, nowStr, warnType, alarmTimes)
						s.UpdateExistIpAlarm(ipAlarm.id, alarmTimes, nowStr)
					}
				}
			}
		} else {
			//	没有就添加一条
			nowStatus := 1
			times := 1
			warnType := 1
			s.WarningToWebhook(pingLossRule, ip, calcAggregateResult, lastPointValue, nowStr, warnType, times)
			s.AddIPAlarm(nowStr, nowStr, calcAggregateResult, nowStatus, times, ip.id, pingLossRule.id)
		}
	} else {
		//没超过阈值看看是否有报警需要恢复
		pingLossAlarms := s.SearchIPPingLossAlarmList(pingLossRule.id, ip.id, 1)
		if len(pingLossAlarms) > 0 { //有
			for _, pingLossAlarm := range pingLossAlarms {
				nowStatus := 0
				warnType := 0 //0 报警解除 1报警触发
				alarmTimes := pingLossAlarm.times + 0
				s.WarningToWebhook(pingLossRule, ip, calcAggregateResult, lastPointValue, nowStr, warnType, alarmTimes)
				// 数据库内时间转换为时间类型
				firstAlarmTimeString := pingLossAlarm.alarm_time
				loc, err := time.LoadLocation("Asia/Shanghai")
				if err != nil {
					s.fileLogger.Error("time.LoadLocation(\"Asia/Shanghai\") Error:%v", err)
				}
				firstAlarmTime, err := time.ParseInLocation("2006-01-02 15:04:05", firstAlarmTimeString, loc)
				if err != nil {
					s.fileLogger.Error("time.Parse Error:%v", err)
				}
				//fmt.Println("firstAlarmTime:", firstAlarmTime)
				//fmt.Println("sqlAlarmTime:", sqlAlarmTime)
				holdMinutes := int(now.Sub(firstAlarmTime).Minutes())
				s.OverIPAlarmToSql(pingLossAlarm.id, nowStr, holdMinutes, nowStatus)
			}
		}
	}
}

// StartTestPingLossRuleAllIPFromDB 从数据库查询 丢包数据判断是否报警
func (s *Supervisor) StartTestPingLossRuleAllIPFromDB() {
	for {
		pingLossRules, err := s.GetIPPingLossRules()
		if err != nil {
			s.fileLogger.Error("StartGetPingLoss Error: %v", err)
		}
		for _, pingLossRule := range pingLossRules {
			ips := s.GetIPRuleBondIP(pingLossRule.id)
			for _, ip := range ips {
				go s.OneIPDataTest(pingLossRule, ip) // 每个IP检查一次开始
			}
		}
		time.Sleep(time.Second * 60)
	}
}

// GetIPPingLossRuleBondWebhooks 规则绑定webhook
func (s *Supervisor) GetIPPingLossRuleBondWebhooks(ruleId int) (webhooks []Webhook, err error) {
	//获取绑定了哪些webhook
	queryIPLossRuleBoundWebhookSql := "select * from `monitor01_ippinglossrule_webhooks` where ippinglossrule_id=?;"
	boundRows, err := s.dbPool.Query(queryIPLossRuleBoundWebhookSql, ruleId)
	if err != nil {
		s.fileLogger.Error("GetRuleBondResource -->%s Error: %v", queryIPLossRuleBoundWebhookSql, err)
	}
	defer boundRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	var webhooksIds []int
	for boundRows.Next() {
		var rwb IPPingLossRuleWebhookBond
		err = boundRows.Scan(&rwb.id, &rwb.ippinglossrule_id, &rwb.webhook_id)
		if err != nil {
			s.fileLogger.Error("GetRuleBondResource boundRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug("boundRows.Scan %v", rwb)
		webhooksIds = append(webhooksIds, rwb.webhook_id)
	}
	//id 转换为字符串切片
	var idsStrSlice []string
	for _, id := range webhooksIds {
		idsStrSlice = append(idsStrSlice, strconv.Itoa(id))
	}
	boundSum := len(idsStrSlice)
	if boundSum > 0 {
		//组合查询各个资源ID的sql语句
		queryBoundWebhooksString := "select * from `monitor01_ippinglossrule_webhooks` where id="
		sqlIdsString := strings.Join(idsStrSlice, " or id=")
		queryBoundWebhooksSql := queryBoundWebhooksString + sqlIdsString + ";"
		// 执行查询资源
		boundWebhooks, err := s.dbPool.Query(queryBoundWebhooksSql)
		defer boundWebhooks.Close() // 多行返回结果，一定要记得关闭
		if err != nil {
			s.fileLogger.Error("GetRuleBondWebhooks -->%s Error: %v", queryBoundWebhooksSql, err)
		}
		//获取查询结果
		for boundWebhooks.Next() {
			var webhook Webhook
			err = boundWebhooks.Scan(&webhook.id, &webhook.name, &webhook.webhook_type, &webhook.url)
			if err != nil {
				s.fileLogger.Error("GetIPPingLossRuleBondWebhook boundResources.Scan Error: %v", err)
			}
			webhooks = append(webhooks, webhook)
			s.fileLogger.Debug("boundWebhooks.Scan %v", webhook)
		}
		s.fileLogger.Info("GetIPPingLossRuleBondWebhook rule id: %d, webhooks: %v", ruleId, webhooks)
	}
	return
}

// noRedirect 请求禁止重定向工具函数
func noRedirect(req *http.Request, via []*http.Request) error {
	return errors.New("Don't redirect!")
}

// httpTestOnce http请求测试
func (s *Supervisor) httpTestOnce(serviceUrl string) (responseCode int) {
	//异常处理
	defer func() {
		//捕获异常
		err := recover()
		if err != nil { //条件判断，是否存在异常
			//存在异常,抛出异常
			//fmt.Println(err)
			s.fileLogger.Error("http请求serviceUrl 一次出错：%v", err)
			responseCode = -8888
		}
	}()
	responseCode = -8888
	urlstr, err := url.Parse(serviceUrl)
	if err != nil {
		s.fileLogger.Error("url.Parse(serviceUrl) Error: %v", err)
	}
	request, err := http.NewRequest("GET", urlstr.String(), bytes.NewBuffer([]byte{}))
	var response *http.Response
	// 客户端禁止重定向
	client := &http.Client{
		CheckRedirect: noRedirect,
	}
	//response, err = http.DefaultClient.Do(request)
	response, err = client.Do(request)
	if err != nil {
		s.fileLogger.Error("http探测含重定向：%v", err)
	} else {
		//防止空指针异常runtime error: invalid memory address or nil pointer dereference
		responseCode = response.StatusCode
	}
	s.fileLogger.Info("serviceUrl %v response Code: %v", serviceUrl, responseCode)
	return
}

// GetHttpUrlRuleBondWebhook 查询绑定webhook地址
func (s *Supervisor) GetHttpUrlRuleBondWebhook(httpTestRuleId int) (webhooks []Webhook, err error) {
	//获取绑定了哪些资源ID
	sqlRuleBoundResourceString := "select * from `monitor01_httptestrule_webhooks` where httptestrule_id=?;"
	boundRows, err := s.dbPool.Query(sqlRuleBoundResourceString, httpTestRuleId)
	if err != nil {
		s.fileLogger.Error("GetIPLossRuleBondWebhook -->%s Error: %v", sqlRuleBoundResourceString, err)
	}
	defer boundRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	var webhooksIds []int
	for boundRows.Next() {
		var rwb HttpTestRuleWebhookBond
		err = boundRows.Scan(&rwb.id, &rwb.httptestrule_id, &rwb.webhook_id)
		if err != nil {
			s.fileLogger.Error("GetIPLossRuleBondWebhook boundRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug("GetIPLossRuleBondWebhook - boundRows.Scan %v", rwb)
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
			s.fileLogger.Error("GetHttpUrlRuleBondWebhook -->%s Error: %v", queryBoundWebhooksSql, err)
		}
		//获取查询结果
		for boundWebhooks.Next() {
			var webhook Webhook
			err = boundWebhooks.Scan(&webhook.id, &webhook.name, &webhook.webhook_type, &webhook.url)
			if err != nil {
				s.fileLogger.Error("GetHttpUrlRuleBondWebhook boundResources.Scan Error: %v", err)
			}
			webhooks = append(webhooks, webhook)
			s.fileLogger.Debug("boundWebhooks.Scan %v", webhook)
		}
		s.fileLogger.Info("GetHttpUrlRuleBondWebhook rule id: %d, resources: %v", httpTestRuleId, webhooks)
	}
	return
}

// GetHttpTestRules 获取监控报警规则
func (s *Supervisor) GetHttpTestRules() (rules []HttpUrlRule, err error) {
	sqlurlString := "select * from `monitor01_httptestrule`;"
	rows, err := s.dbPool.Query(sqlurlString)
	defer rows.Close() // 多行多行返回结果，一定要记得关闭
	//var rules []Rule
	if err != nil {
		s.fileLogger.Error("query failed, err: %v", err)
	}
	// 多个结果中循环取值
	for rows.Next() {
		//准备查询结果的对象
		var urlObj HttpUrlRule
		err = rows.Scan(&urlObj.id, &urlObj.ctime,
			&urlObj.url_name, &urlObj.url,
			&urlObj.test_interval_second, &urlObj.healthy_times,
			&urlObj.unhealthy_times, &urlObj.alarm_interval,
			&urlObj.max_alarm_times, &urlObj.already_alarm_times,
			&urlObj.last_alarm_time, &urlObj.enable, &urlObj.healthy)
		if err != nil {
			s.fileLogger.Error("GetHttpTestRules rows.Scan Error: %v", err)
		}
		//fmt.Println(rule)
		rules = append(rules, urlObj)
	}
	return
}

// HttpCodeHealthy 判断HTTPCode 状态码是否健康
func (s *Supervisor) HttpCodeHealthy(code int) bool {
	codeStr := strconv.Itoa(code)
	return strings.HasPrefix(codeStr, "2") || strings.HasPrefix(codeStr, "3")
}

// UpdateHttpUrlRule 更新已报警次数和上次报警时间
func (s Supervisor) UpdateHttpUrlRule(urlObjID, alarmTimes int, nowStr string, healthy int) {
	UpdateSql := "update `monitor01_httptestrule` set already_alarm_times=?, last_alarm_time=?, healthy=? where id=?;"
	_, err := s.dbPool.Exec(UpdateSql, alarmTimes, nowStr, healthy, urlObjID)
	if err != nil {
		s.fileLogger.Error("update UpdateHttpUrlRule Error:%v", err)
	}
}

// UpdateHttpUrlRuleHealthy 更新已报警次数和上次报警时间
func (s Supervisor) UpdateHttpUrlRuleHealthy(urlObjID, healthy int) {
	UpdateSql := "update `monitor01_httptestrule` set  healthy=? where id=?;"
	_, err := s.dbPool.Exec(UpdateSql, healthy, urlObjID)
	if err != nil {
		s.fileLogger.Error("update UpdateHttpUrlRuleHealthy Error:%v", err)
	}
}

// AlwaysTestHttpUrlStatus 判断一个URL地址是否健康
func (s *Supervisor) AlwaysTestHttpUrlStatus(urlObj *HttpUrlRule) {
	//初始化健康检查计数
	healtyTimes := 0
	unhealtyTimes := 0
	//如果不结束检查就一直跑
	s.fileLogger.Debug("开始检测url:%v", urlObj.url)
	for {
		if urlObj.enable {
			httpCode := s.httpTestOnce(urlObj.url)
			s.fileLogger.Debug("test url %s response Code:%v", urlObj.url, httpCode)
			now := time.Now()
			nowStr := now.Format("2006-01-02 15:04:05")
			//如果本次检查不健康
			if !s.HttpCodeHealthy(httpCode) {
				unhealtyTimes += 1
				if unhealtyTimes >= urlObj.unhealthy_times {
					//// 报警发生
					//webhooks, err := s.GetHttpUrlRuleBondWebhook(urlObj.id)
					//if err != nil {
					//	s.fileLogger.Error("AlwaysTestHttpUrlStatus GetHttpUrlRuleBondWebhook Error:%v ", err)
					//}
					warnType := 1
					//是否发报警
					if urlObj.already_alarm_times < urlObj.max_alarm_times { //判断报警次数是否超
						//开始判断保健间隔时间是否够设置间隔分钟数
						// 数据库内时间转换为时间类型
						loc, err := time.LoadLocation("Asia/Shanghai")
						if err != nil {
							s.fileLogger.Error("time.LoadLocation(\"Asia/Shanghai\") Error:%v", err)
						}
						lastAlarmTime, err := time.ParseInLocation("2006-01-02 15:04:05", urlObj.last_alarm_time, loc)
						deltaTMin := time.Now().Sub(lastAlarmTime).Minutes()
						deltaTMinInt := int(deltaTMin)
						//够报警间隔分钟数---报警 发送
						if deltaTMinInt > urlObj.alarm_interval { // 判断时间是否够间隔，
							alarmTimes := urlObj.already_alarm_times + 1
							httpCodeFloat := float64(httpCode)
							healty := 0 //健康1，不健康0
							s.WarningToWebhook(urlObj, urlObj, httpCodeFloat, httpCodeFloat, nowStr, warnType,
								alarmTimes)
							s.UpdateHttpUrlRule(urlObj.id, alarmTimes, nowStr, healty)
							s.fileLogger.Debug("健康-->不健康 %s", urlObj.url)
						}
					}
				}
				//如果本次健康
			} else {
				//看当前记录健康状态，判断是否需要发送恢复健康通知
				if unhealtyTimes != 0 {
					healtyTimes += 1
					//判断从不健康需要过渡为健康状态
					if healtyTimes >= urlObj.healthy_times {
						//恢复健康
						healtyTimes = urlObj.healthy_times
						unhealtyTimes = 0
						//发送恢复通知
						warnType := 0 //0恢复 1报警
						httpCodeFloat := float64(httpCode)
						alarmTimes := urlObj.already_alarm_times
						s.WarningToWebhook(urlObj, urlObj, httpCodeFloat, httpCodeFloat, nowStr, warnType,
							alarmTimes)
						resetAlarmTimes := 0 //数据库内重置报警次数
						healty := 1          // 健康1，不健康0
						s.UpdateHttpUrlRule(urlObj.id, resetAlarmTimes, nowStr, healty)
						s.fileLogger.Debug("不健康--->健康 %s", urlObj.url)
					}
				} else {
					if urlObj.healthy == 0 {
						healthy := 1
						s.fileLogger.Debug("本就健康更新数据库为健康%s", urlObj.url)
						s.UpdateHttpUrlRuleHealthy(urlObj.id, healthy)
					}
				}
			}
		} else {
			s.fileLogger.Debug("URL检测暂停中url:%v %v后刷新", urlObj.url, urlObj.test_interval_second)
		}
		if urlObj.delete {
			s.fileLogger.Debug("删除检测url:%v 子线程已退出", urlObj.url)
			break
		}
		time.Sleep(time.Duration(urlObj.test_interval_second) * time.Second)
	}
}

// containsUrlId 判断数据库中是否存在已有URL。
func (s *Supervisor) containsUrlId(rules []HttpUrlRule, id int) bool {
	for _, urlObj := range rules {
		if urlObj.id == id {
			return true
		}
	}
	return false
}

// startHttpTest 开始http探测
func (s *Supervisor) startHttpTest(flashSecond int) {
	//内存中临时存储正在检测的http url 地址
	urlMap := make(map[int]*HttpUrlRule, 100)
	for {
		//fmt.Println(urlMap)
		s.fileLogger.Debug("内存中map:%v", urlMap)
		urlRules, err := s.GetHttpTestRules()
		if err != nil {
			s.fileLogger.Error("GetHttpTestRules Error:%v", err)
		}
		// 判断监控中的资源是否需要增加，或修改
		for _, testUrl := range urlRules {
			//判断当前检测中的有没有数据库中的实例
			_, ok := urlMap[testUrl.id]
			//map中没实例了，就增加
			if !ok {
				urlMap[testUrl.id] = &HttpUrlRule{id: testUrl.id, ctime: testUrl.ctime,
					url_name: testUrl.url_name, url: testUrl.url, test_interval_second: testUrl.test_interval_second,
					healthy_times: testUrl.healthy_times, unhealthy_times: testUrl.unhealthy_times, alarm_interval: testUrl.alarm_interval,
					max_alarm_times: testUrl.max_alarm_times, already_alarm_times: testUrl.already_alarm_times,
					last_alarm_time: testUrl.last_alarm_time, enable: testUrl.enable, healthy: testUrl.healthy, delete: false,
				}
				//启动单个url 拨测协程
				s.fileLogger.Debug("启动检测url:%v", testUrl.url)
				go s.AlwaysTestHttpUrlStatus(urlMap[testUrl.id])
			} else {
				//	map中有实例，就更新
				urlMap[testUrl.id].id = testUrl.id
				urlMap[testUrl.id].ctime = testUrl.ctime
				urlMap[testUrl.id].url_name = testUrl.url_name
				urlMap[testUrl.id].url = testUrl.url
				urlMap[testUrl.id].test_interval_second = testUrl.test_interval_second
				urlMap[testUrl.id].healthy_times = testUrl.healthy_times
				urlMap[testUrl.id].unhealthy_times = testUrl.unhealthy_times
				urlMap[testUrl.id].alarm_interval = testUrl.alarm_interval
				urlMap[testUrl.id].max_alarm_times = testUrl.max_alarm_times
				urlMap[testUrl.id].already_alarm_times = testUrl.already_alarm_times
				urlMap[testUrl.id].last_alarm_time = testUrl.last_alarm_time
				urlMap[testUrl.id].enable = testUrl.enable
				urlMap[testUrl.id].healthy = testUrl.healthy
				urlMap[testUrl.id].delete = false
			}
		}
		//判断监控中的资源是否需要删除
		for _, urlObj := range urlMap {
			//数据库中已不存在这个url，就删更新map里面的url对象，看是否停止检测
			//fmt.Println("urlObj:", urlObj)
			if !s.containsUrlId(urlRules, urlObj.id) {
				urlObj.enable = false
				urlObj.delete = true
				delete(urlMap, urlObj.id)
			}
		}
		//从数据库内获取要监控的url刷新频率
		time.Sleep(time.Second * time.Duration(flashSecond))
	}

}
