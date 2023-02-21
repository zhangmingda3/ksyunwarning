package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	myLogger "github.com/zhangmingda3/ksyun-warning/myloggerBackground"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NewSupervisor ==========构造监控者结构体===============
func NewSupervisor(logger *myLogger.FileLogger, db *sql.DB) (supervisor *Supervisor) {
	supervisor = &Supervisor{
		fileLogger: logger,
		dbPool:     db,
	}
	return
}

// GetMetricStatisticsBatch 获取云端监控数据
func (s *Supervisor) GetMetricStatisticsBatch(uid, ns, metricName, region, instanceId string, period, leadTime, timeWindow int) (res statisticsBatchResult, err error) {
	//异常处理
	defer func() {
		//捕获异常
		err := recover()
		if err != nil { //条件判断，是否存在异常
			//存在异常,抛出异常
			s.fileLogger.Error("获取监控数据错误：GetMetricStatisticsBatch Error: %v", err)
		}
	}()

	addr := "http://monitor.console.sdns.ksyun.com:80/v2/?Action=GetMetricStatisticsBatch"
	urlObj, err := url.Parse(addr)
	if err != nil {
		s.fileLogger.Error("url.Parse Error:%v", err)
	}
	//starttime := "2023-01-17T14:37:03Z"
	//endtime := "2023-01-17T14:38:03Z"
	now := time.Now()
	// 截止时间前一分钟
	endTime := now.Add(time.Duration(-leadTime) * time.Minute).Format("2006-01-02T15:04:05Z")

	//统计数据时间窗起点：统计周期再往前timeWindow分钟时间窗
	startTime := now.Add(time.Duration(-period-leadTime-timeWindow) * time.Minute).Format("2006-01-02T15:04:05Z")
	//fmt.Println(startTime, "--->", endTime)
	requestId := instanceId + "--" + metricName + "--" + now.String()
	periodStr := ""
	if period < 1 {
		s.fileLogger.Info("本次取原始数据，同期周期为0")
	} else {
		//数字转换为字符串
		periodValueStr := strconv.Itoa(period * 60)
		periodStr = fmt.Sprintf("\"Period\": \"%s\",", periodValueStr)
	}

	//fmt.Println(periodStr)
	jsonStr := fmt.Sprintf(`{
		%s
	  "aggregate": [
		"Average",
		"Max",
		"Min",
		"Sum"
	  ],
	  "namespace": "%s",
	  "starttime": "%s",
	  "endtime": "%s",
	  "metrics": [
		{
		  "instanceid": "%s",
		  "metricname": "%s"
		}
	  ]
	}`, periodStr, ns, startTime, endTime, instanceId, metricName)
	//fmt.Println(jsonStr)
	jsonBytes := []byte(jsonStr)
	//构造请求
	request, err := http.NewRequest("GET", urlObj.String(), bytes.NewBuffer(jsonBytes))
	s.fileLogger.Debug("prepare request : %s %s, uid:%s region:%s ns:%s metricName:%s instanceId:%s", request.Method, addr, uid, region, ns, metricName, instanceId)
	// 函数结束关闭请求体
	defer request.Body.Close()
	if err != nil {
		fmt.Println("http.NewRequest:", err)
		s.fileLogger.Error("http.NewRequest:", err)
	}
	request.Header.Add("accept", "application/json")
	request.Header.Add("content-type", "application/json")
	request.Header.Add("x-ksc-request-id", requestId)
	request.Header.Add("x-ksc-account-id", uid)
	request.Header.Add("x-ksc-region", region)
	var r *http.Response
	r, err = http.DefaultClient.Do(request)
	if err != nil {
		s.fileLogger.Error("http.DefaultClient.Do(request) ERROR: %v requestId: ", err, requestId)
	} else {
		s.fileLogger.Info("GetMetricStatisticsBatch http request successful recv_code:%v", r.Status)
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		s.fileLogger.Error("io.ReadAll ERROR:", err)
	}
	bodyString := string(bodyBytes)
	//fmt.Println(bodyString)
	s.fileLogger.Info("code: %s, uid:%s  Response body: %s", r.Status, uid, bodyString)
	//var res statisticsBatchResult
	err = json.Unmarshal(bodyBytes, &res)
	if err != nil {
		s.fileLogger.Error("Unmarshal bodyBytes Failed %v:", err)
	}
	return
}

// FromStatisticsBatchResultGetLastPointValue 解析获取的云端数据
func (s *Supervisor) FromStatisticsBatchResultGetLastPointValue(res statisticsBatchResult, aggregate string) (value float64, timestamp string) {
	//异常处理
	defer func() {
		//捕获异常
		err := recover()
		if err != nil { //条件判断，是否存在异常
			//存在异常,抛出异常
			s.fileLogger.Error("FromStatisticsBatchResultGetValue Error: %v", err)
		}
	}()
	value = -8888.8888 // 当返回值为null 默认返回这个奇葩的值
	var valueStr string
	if len(res.GetMetricStatisticsBatchResults) > 0 {
		memberLen := len(res.GetMetricStatisticsBatchResults[0].Datapoints.Member)
		lastMemberIndex := memberLen - 1
		timestamp = res.GetMetricStatisticsBatchResults[0].Datapoints.Member[lastMemberIndex].Timestamp
		switch aggregate {
		case "Average":
			valueStr = res.GetMetricStatisticsBatchResults[0].Datapoints.Member[lastMemberIndex].Average
		case "Max":
			valueStr = res.GetMetricStatisticsBatchResults[0].Datapoints.Member[lastMemberIndex].Max
		case "Min":
			valueStr = res.GetMetricStatisticsBatchResults[0].Datapoints.Member[lastMemberIndex].Min
		case "Sum":
			valueStr = res.GetMetricStatisticsBatchResults[0].Datapoints.Member[lastMemberIndex].Sum
		}
		var err error
		if valueStr != "null" {
			value, err = strconv.ParseFloat(valueStr, 10) // int 转换为int64
			if err != nil {
				s.fileLogger.Error("FromStatisticsBatchResultGetValue Error: %v", err)
			}
		}
	} else {
		s.fileLogger.Info("返回MetricStatisticsBatchResults数据为空")
	}
	return
}

// GetRules 获取监控报警规则
func (s *Supervisor) GetRules() (rules []Rule, err error) {
	sqlRuleString := "select * from `monitor01_rule`;"
	rows, err := s.dbPool.Query(sqlRuleString)
	defer rows.Close() // 多行多行返回结果，一定要记得关闭
	//var rules []Rule
	if err != nil {
		s.fileLogger.Error("query failed, err: %v", err)
	}
	// 多个结果中循环取值
	for rows.Next() {
		//准备查询结果的对象
		var rule Rule
		err = rows.Scan(&rule.id, &rule.ctime,
			&rule.namespace, &rule.alarm_name,
			&rule.metric_name, &rule.rule_type,
			&rule.upper_limit, &rule.lower_limit,
			&rule.period, &rule.alarm_interval,
			&rule.max_alarm_times, &rule.aggregate)
		if err != nil {
			s.fileLogger.Error("GetRules rows.Scan Error: %v", err)
		}
		//fmt.Println(rule)
		rules = append(rules, rule)
	}
	return
}

// GetRuleBondResource 获取Rule 绑定的资源
func (s *Supervisor) GetRuleBondResource(ruleId int) (resources []Resource, err error) {
	//获取绑定了哪些资源ID
	sqlRuleBoundResourceString := "select * from `monitor01_rule_instances` where rule_id=?;"
	boundRows, err := s.dbPool.Query(sqlRuleBoundResourceString, ruleId)
	if err != nil {
		s.fileLogger.Error("GetRuleBondResource -->%s Error: %v", sqlRuleBoundResourceString, err)
	}
	defer boundRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	var instancesIds []int
	for boundRows.Next() {
		var rrb RuleResourceBound
		err = boundRows.Scan(&rrb.id, &rrb.rule_id, &rrb.resource_id)
		if err != nil {
			s.fileLogger.Error("GetRuleBondResource boundRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug("boundRows.Scan %v", rrb)
		instancesIds = append(instancesIds, rrb.resource_id)
	}
	//id 转换为字符串切片
	var idsStrSlice []string //
	for _, id := range instancesIds {
		idsStrSlice = append(idsStrSlice, strconv.Itoa(id))
	}
	//fmt.Println("实例ID切片：", idsStrSlice)
	boundSum := len(idsStrSlice)
	//fmt.Println("绑定实例个数：", boundSum)
	// 获取实例信息
	//var resources []Resource
	if boundSum > 0 {
		//组合查询各个资源ID的sql语句
		queryBoundResourceString := "select * from `monitor01_resource` where id="
		sqlIdsString := strings.Join(idsStrSlice, " or id=")
		queryBoundResourceSql := queryBoundResourceString + sqlIdsString + ";"
		// 执行查询资源
		boundResources, err := s.dbPool.Query(queryBoundResourceSql)
		defer boundResources.Close() // 多行返回结果，一定要记得关闭
		if err != nil {
			s.fileLogger.Error("GetRuleBondResource -->%s Error: %v", queryBoundResourceSql, err)
		}
		//获取查询结果
		for boundResources.Next() {
			var resource Resource
			err = boundResources.Scan(&resource.id, &resource.name, &resource.uid, &resource.namespace, &resource.instance_id, &resource.region)
			if err != nil {
				s.fileLogger.Error("GetRuleBondResource boundResources.Scan Error: %v", err)
			}
			resources = append(resources, resource)
			s.fileLogger.Debug("boundResources.Scan %v", resource)
		}
		s.fileLogger.Info("GetRuleBondResource rule id: %d, resources: %v", ruleId, resources)
	}
	return
}

// SearchAlarmList 查询告警记录
func (s *Supervisor) SearchAlarmList(ruleId, resourceID, nowStatus int) []AlarmObj {
	//获取绑定了哪些资源ID
	queryAlarmsSql := "select * from `monitor01_alarmlist` where alarm_rule_id=? and alarm_resource_id=? and now_status=?;"
	alarmRows, err := s.dbPool.Query(queryAlarmsSql, ruleId, resourceID, nowStatus)
	if err != nil {
		s.fileLogger.Error("SearchAlarmList -->%s Error: %v", queryAlarmsSql, err)
	}
	defer alarmRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	var alarms []AlarmObj
	for alarmRows.Next() {
		var alarm AlarmObj
		err = alarmRows.Scan(&alarm.id, &alarm.alarm_time, &alarm.alarm_value, &alarm.last_alarm_time,
			&alarm.hold_minutes, &alarm.now_status, &alarm.recover_time, &alarm.times, &alarm.alarm_resource_id, &alarm.alarm_rule_id)
		if err != nil {
			s.fileLogger.Error("SearchAlarmList alarmRows.Scan Error: %v", err)
		}
		s.fileLogger.Debug("alarmRows.Scan %v", alarm)
		alarms = append(alarms, alarm)
	}
	return alarms
}

// UpdateExistAlarm 更新报警内容
func (s *Supervisor) UpdateExistAlarm(alarmID, alarmTimes int, nowStr string) {
	UpdateSql := "update `monitor01_alarmlist` set times=?,last_alarm_time=? where id=?;"
	_, err := s.dbPool.Exec(UpdateSql, alarmTimes, nowStr, alarmID)
	if err != nil {
		s.fileLogger.Error("update ExistAlarm Error:%v", err)
	}
	//fmt.Println("update ExistAlarm: ", result)
}

// AddAlarm 新增报警信息
func (s *Supervisor) AddAlarm(alarmTime string, alarmValue float64, lastAlarmTime string, nowStatus, times, alarmResourceID, alarmRuleID int) {
	sqlString := "insert into `monitor01_alarmlist` (alarm_time,alarm_value,last_alarm_time,now_status,times,alarm_resource_id, alarm_rule_id) values(?,?,?,?,?,?,?);"
	_, err := s.dbPool.Exec(sqlString, alarmTime, alarmValue, lastAlarmTime, nowStatus, times, alarmResourceID, alarmRuleID)
	if err != nil {
		s.fileLogger.Error("AddAlarm Error:%v", err)
	}
	//fmt.Println("AddAlarm: ", result)
}

// OverAlarmToSql 报警结束更新数据库
func (s *Supervisor) OverAlarmToSql(alarmID, holdMinutes, nowStatus int, sqlAlarmTime string) {
	UpdateSql := "update `monitor01_alarmlist` set hold_minutes=?,now_status=?,recover_time=? where id=?;"
	result, err := s.dbPool.Exec(UpdateSql, holdMinutes, nowStatus, sqlAlarmTime, alarmID)
	if err != nil {
		s.fileLogger.Error("OverAlarmToSql dbPool.Exec  Error:%v", err)
	}
	fmt.Println("OverAlarmToSql result : ", result)
}

// GetLastPointRepeat 获取最后一个点的数据
func (s *Supervisor) GetLastPointRepeat(r Rule, resource Resource, leadTime, timeWindow, lastPointNullMaxRetry int) (value float64, timstamp string) {
	statisticBatchResult, err := s.GetMetricStatisticsBatch(resource.uid,
		resource.namespace,
		r.metric_name,
		resource.region,
		resource.instance_id,
		r.period, leadTime, timeWindow)
	if err != nil {
		s.fileLogger.Error("supervisor.GetMetricStatisticsBatch Error:%v", err)
	}
	value, timestamp := s.FromStatisticsBatchResultGetLastPointValue(statisticBatchResult, r.aggregate)
	// value == "null" 已被转换为-8888.8888代替
	// 重试次数
	alreadyRetry := 0
	//值为null 反复查2次
	for value == -8888.8888 && lastPointNullMaxRetry != 0 {
		s.fileLogger.Debug("查询uid：%s namespace:%s 实例ID:%s  监控项%s 时间：%s 的值为null ,下面开始重试第%d次, \n",
			resource.uid, resource.namespace, resource.instance_id, r.metric_name, timestamp, alreadyRetry+1)
		statisticBatchResult, err = s.GetMetricStatisticsBatch(resource.uid,
			resource.namespace,
			r.metric_name,
			resource.region,
			resource.instance_id,
			r.period, leadTime, timeWindow)
		if err != nil {
			s.fileLogger.Error("supervisor.GetMetricStatisticsBatch Error:%v", err)
		}
		value, timestamp = s.FromStatisticsBatchResultGetLastPointValue(statisticBatchResult, r.aggregate) //
		lastPointNullMaxRetry -= 1
		alreadyRetry += 1
		time.Sleep(time.Second * 5) // 重试间隔
		if (alreadyRetry == lastPointNullMaxRetry) && value == -8888.8888 {
			s.fileLogger.Error("查询uid：%s namespace:%s 实例ID:%s  监控项%s  重试%d次都为null, 本次已放弃查询\n",
				resource.uid, resource.namespace, resource.instance_id, r.metric_name, alreadyRetry)
			s.WarningToWebhook(r, resource, value, value, timestamp, 1, 1)
		}
	}

	return value, timestamp
}

// StartComparing 开始对比
func (s *Supervisor) StartComparing(r Rule, resource Resource, leadTime, timeWindow, lastPointNullMaxRetry int) {
	value, timestamp := s.GetLastPointRepeat(r, resource, leadTime, timeWindow, lastPointNullMaxRetry)
	//fmt.Println(
	//	resource.uid,
	//	resource.region,
	//	resource.namespace,
	//	r.metric_name,
	//	resource.instance_id,
	//	"value:", value)
	// 本地时间转换为数据库存储一致的
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		s.fileLogger.Error("time.LoadLocation(\"Asia/Shanghai\") Error:%v", err)
	}
	valueAlarmTime, err := time.ParseInLocation("2006-01-02T15:04:05Z", timestamp, loc)
	sqlAlarmTime := valueAlarmTime.Format("2006-01-02 15:04:05")
	//阈值报警逻辑
	if r.rule_type == 0 {
		//对比代码字段
		if value != -8888.8888 && (value >= r.upper_limit || value <= r.lower_limit) {
			fmt.Println("报警触发：.....")
			// 查数据库里是否有报警记录：
			// 有 判断时间是否够间隔，
			// ---够： 报警：次数在已报警次数上增加1
			//1. 查数据库里是否有报警记录：
			alarms := s.SearchAlarmList(r.id, resource.id, 1)
			if len(alarms) > 0 { //有报警未恢复
				for _, alarm := range alarms {
					// 数据库内时间转换为时间类型
					lastAlarmTimeString := alarm.last_alarm_time
					loc, err := time.LoadLocation("Asia/Shanghai")
					if err != nil {
						s.fileLogger.Error("time.LoadLocation(\"Asia/Shanghai\") Error:%v", err)
					}
					lastAlarmTime, err := time.ParseInLocation("2006-01-02 15:04:05", lastAlarmTimeString, loc)
					if err != nil {
						s.fileLogger.Error("time.Parse Error:%v", err)
					}
					if alarm.times < r.max_alarm_times { //判断报警次数是否超
						deltaTMin := time.Now().Sub(lastAlarmTime).Minutes()
						deltaTMinInt := int(deltaTMin)
						if deltaTMinInt > r.alarm_interval { // 判断时间是否够间隔，
							warnType := 1 //1报警触发 0报警解除
							alarmTimes := alarm.times + 1
							s.WarningToWebhook(r, resource, value, value, sqlAlarmTime, warnType, alarmTimes)
							nowStr := time.Now().Format("2006-01-02 15:04:05")
							s.UpdateExistAlarm(alarm.id, alarmTimes, nowStr)
						}
					}
				}
			} else {
				//	没有就添加一条
				nowStatus := 1
				times := 1
				warnType := 1
				s.WarningToWebhook(r, resource, value, value, sqlAlarmTime, warnType, times)
				s.AddAlarm(sqlAlarmTime, value, sqlAlarmTime, nowStatus, times, resource.id, r.id)
			}
		} else {
			//没超过阈值看看是否有报警需要恢复
			alarms := s.SearchAlarmList(r.id, resource.id, 1)
			if len(alarms) > 0 { //有
				for _, alarm := range alarms {
					nowStatus := 0
					warnType := 0 //0 报警解除 1报警触发
					alarmTimes := alarm.times + 0
					s.WarningToWebhook(r, resource, value, value, sqlAlarmTime, warnType, alarmTimes)
					// 数据库内时间转换为时间类型
					firstAlarmTimeString := alarm.alarm_time
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
					holdMinutes := int(valueAlarmTime.Sub(firstAlarmTime).Minutes())
					s.OverAlarmToSql(alarm.id, holdMinutes, nowStatus, sqlAlarmTime)
				}
			}
		}
	} else if r.rule_type == 1 {
		prevValue, _ := s.GetLastPointRepeat(r, resource, leadTime+1, timeWindow, lastPointNullMaxRetry)
		//fmt.Println("前一分钟：", prevTimestamp, prevValue)
		//fmt.Println("当前最新数据", timestamp, value)
		subValue := value - prevValue
		if value != -8888.8888 && (subValue >= r.upper_limit || subValue <= r.lower_limit) {
			warnType := 1 //0解除，1警报，
			s.WarningToWebhook(r, resource, value, value, sqlAlarmTime, warnType, 1)
			nowStatus := 2 //0解除，1警报，2事件 sql中
			s.AddAlarm(sqlAlarmTime, value, sqlAlarmTime, nowStatus, 1, resource.id, r.id)
		}
	}
}

// GetRegionChinese 地区中文名称
func (s *Supervisor) GetRegionChinese(string2 string) (regionChinese string) {
	region := make(map[string]string)
	region["cn-beijing-6"] = "华北1（北京）"
	region["cn-shanghai-2"] = "华东1（上海）"
	region["cn-hongkong-2"] = "中国香港"
	region["cn-guangzhou-1"] = "华南1（广州）"
	region["ap-singapore-1"] = "新加坡"
	region["eu-east-1"] = "俄罗斯（莫斯科）"
	region["cn-beijing-fin"] = "华北金融1（北京）专区"
	region["eu-east-1"] = "华东金融1（上海）专区"
	region["cn-southwest-1"] = "西南1(重庆)"
	region["cn-central-1"] = "华中1(武汉)"
	region["cn-northwest-1"] = "西北1区（庆阳）"
	regionChinese = region[string2]
	return
}

// GetRuleTypeChinese 规则类型中文名称
func (s *Supervisor) GetRuleTypeChinese(ruleTypeIndex int) (RuleTypeChinese string) {
	ruleType := make(map[int]string)
	ruleType[0] = "阈值"
	ruleType[1] = "突增降"
	RuleTypeChinese = ruleType[ruleTypeIndex]
	return
}

// GetRuleTextChinese 规则内容
func (s *Supervisor) GetRuleTextChinese(rule Rule) (ruleText string) {
	//var ruleText string
	if rule.rule_type == 0 {
		ruleText = fmt.Sprintf("大于等于%v或小于等于%v", rule.upper_limit, rule.lower_limit)
	} else if rule.rule_type == 1 {
		ruleText = fmt.Sprintf("突增%v或突降%v", rule.upper_limit, rule.lower_limit)
	}
	return
}

// GetRuleBondWebhooks 查询绑定webhook地址
func (s *Supervisor) GetRuleBondWebhooks(ruleId int) (webhooks []Webhook, err error) {
	//获取绑定了哪些资源ID
	sqlRuleBoundResourceString := "select * from `monitor01_rule_webhooks` where rule_id=?;"
	boundRows, err := s.dbPool.Query(sqlRuleBoundResourceString, ruleId)
	if err != nil {
		s.fileLogger.Error("GetRuleBondResource -->%s Error: %v", sqlRuleBoundResourceString, err)
	}
	defer boundRows.Close() // 多行返回结果，一定要记得关闭
	//存储绑定的资源数据库内的ID
	var webhooksIds []int
	for boundRows.Next() {
		var rwb RuleWebhookBound
		err = boundRows.Scan(&rwb.id, &rwb.rule_id, &rwb.webhook_id)
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
				s.fileLogger.Error("GetRuleBondResource boundResources.Scan Error: %v", err)
			}
			webhooks = append(webhooks, webhook)
			s.fileLogger.Debug("boundWebhooks.Scan %v", webhook)
		}
		s.fileLogger.Info("GetRuleBondWebhook rule id: %d, resources: %v", ruleId, webhooks)
	}
	return
}

// GetAggregateChinese 聚合算法类型中文名称
func (s *Supervisor) GetAggregateChinese(aggregateTypeIndex string) (RuleTypeChinese string) {
	aggregateType := make(map[string]string)
	aggregateType["Average"] = "平均值"
	aggregateType["Max"] = "最大值"
	aggregateType["Min"] = "最小值"
	aggregateType["Sum"] = "总和"
	return aggregateType[aggregateTypeIndex]
}
