package main

import (
	myLogger "github.com/zhangmingda3/ksyun-warning/myloggerBackground"
	"time"
)

// startScanRule 客户资源监控开始
func (s *Supervisor) startScanRule() {
	for {
		rules, err := s.GetRules()
		if err != nil {
			s.fileLogger.Error("supervisor.GetRules() Error:%v", err)
		}
		s.fileLogger.Debug("supervisor.GetRules : %v", rules)
		for _, r := range rules {
			//fmt.Println(r)
			resources, err := s.GetRuleBondResource(r.id)
			if err != nil {
				s.fileLogger.Error("supervisor.GetRuleBondResource Error: %v", err)
			}
			if len(resources) > 0 {
				//查询数据的时间窗对比当前时间提前分钟数
				leadTime := 1              // 查询当前时间提前几分钟数据
				timeWindow := 3            // 聚合周期再往前几分钟，查询数据起始时间窗
				lastPointNullMaxRetry := 2 // HTTP获取数据为null最大重试次数
				for _, resource := range resources {
					go s.StartComparing(r, resource, leadTime, timeWindow, lastPointNullMaxRetry)
				}
			}
		}
		time.Sleep(time.Second * 60)
	}

}

func main() {
	logfilePath := "D:\\Go-Study\\src\\github.com\\zhangmingda3\\ksyun-warning\\logs"
	logLevel := "Debug"
	maxLogSize := 10 * 1024 * 1024
	logName := "ksyun-warning.log"
	logFile := myLogger.NewFileLogger(logLevel, logfilePath, logName, int64(maxLogSize))
	dbAddr := "admin:Wyf!1314@tcp(192.168.236.128:3306)/monitor_test"
	db, err := initDB(dbAddr)
	if err != nil {
		logFile.Fatal("initDB Fatal: %v", err)
	}
	logFile.Info("admin:Wyf!1314@tcp(192.168.236.128:3306)/monitor_test connection OK")
	// 构建监控者对象
	supervisor := NewSupervisor(logFile, db)
	//// 启动云监控报警策略
	go supervisor.startScanRule()
	//启动ping监控数据上报
	go supervisor.StartFpingBondIPsToDB(20, "40", "500")
	////启动ping报警策略
	go supervisor.StartTestPingLossRuleAllIPFromDB()
	//启动http探测
	go supervisor.startHttpTest(10)
	//启动公告发送 //每一分钟更新一次
	supervisor.StartNoticeAnnouncement(1)

	// 测试修改数据库bool
	//supervisor.UpdateHttpUrlRuleHealthy(5, 0)
	//// 准备要查询的资源信息
	//instanceId := "2f2de38b-59c3-450e-bcbb-1527095f77b9"
	//region := "cn-beijing-6"
	//ns := "nat"
	//uid := "73398680"
	//period := 2
	//metricName := "vpc.nat.public.utilization.in"
	//
	//result, err := supervisor.GetMetricStatisticsBatch(uid, ns, metricName, region, instanceId, period)
	//
	//fmt.Println(result.GetMetricStatisticsBatchResults[0].Datapoints.Member[0].Average)
	//fmt.Println(result.GetMetricStatisticsBatchResults[0].Datapoints.Member[0].Timestamp)
	//fmt.Println(result.GetMetricStatisticsBatchResults[0].Instance)
	//fmt.Println(result.GetMetricStatisticsBatchResults[0].Label)

	////测试日志
	//for {
	//	logFile.Debug("这是一条Debug日志")
	//	logFile.Trace("这是一条Trace日志")
	//	logFile.Info("这是一条Info日志")
	//	logFile.Warning("这是一条Warning日志")
	//	logFile.Error("这是一条Error日志:%s %v", "测试格式化输入错误", 1231)
	//	logFile.Fatal("这是一条Fatal日志")
	//	time.Sleep(time.Microsecond)
	//}
	time.Sleep(time.Second)
}
