package ksyunwarning

import (
	"database/sql"
	"github.com/zhangmingda3/myLogger"
	//myLogger "github.com/zhangmingda3/ksyunwarning/myloggerBackground"
)

// =============↓↓响应结构体信息↓↓=====================
type point struct {
	Timestamp     string `json:"timestamp"`
	UnixTimestamp int    `json:"unixTimestamp"`
	Average       string `json:"average"`
	Max           string `json:"max"`
	Min           string `json:"min"`
	Sum           string `json:"sum"`
}
type points struct {
	Member []point `json:"member"`
}
type Result struct {
	Datapoints points `json:"datapoints"`
	Label      string `json:"label"`
	Instance   string `json:"instance"`
}
type metadata struct {
	RequestId string `json:"requestId"`
}

// statisticsBatchResult  定义接收数据的结构体，注意属性开头必须用大写，否则无法写入数据
type statisticsBatchResult struct {
	GetMetricStatisticsBatchResults []Result `json:"getMetricStatisticsBatchResults"`
	ResponseMetadata                metadata `json:"responseMetadata"`
}

//==========↑响应结构体信息↑=======================

// Supervisor 监控体定义
type Supervisor struct {
	fileLogger *myLogger.FileLogger
	dbPool     *sql.DB
}

//==========↓↓数据库表结构体↓↓==============
/* Python 报警规则表
# Rule 报警规则
class Rule(models.Model):
    ctime = models.DateTimeField(auto_now_add=True, null=True)
    namespace = models.CharField(max_length=32, verbose_name='资源类型(Namespace)', null=False )   # verbose_name表示admin管理时显示的中文
    alarm_name = models.CharField(max_length=32, verbose_name='报警名称', null=False)              # verbose_name表示admin管理时显示的中文
    metric_name = models.CharField(max_length=32, verbose_name='监控项(MetricName)', null=False)         # verbose_name表示admin管理时显示的中文
    instances = models.ManyToManyField('Resource',  verbose_name="实例列表", blank=True)
    warning_type_select = (
        (0, '阈值'),
        (1, '突增降'),
    )
    warning_type = models.IntegerField(choices=warning_type_select, verbose_name="告警类型", default=0)
    upper_limit = models.BigIntegerField(verbose_name="大于(阈值)|突增(比上一分钟)")
    lower_limit = models.BigIntegerField(verbose_name="小于(阈值)|突降(比上一分钟)")
    aggregate_type_select = (
        ('Average', '平均值'),
        ('Max', '最大值'),
        ('Min', '最小值'),
        ('Sum', '统计周期内数据之和'),
    )
    aggregate = models.CharField(max_length=32, choices=aggregate_type_select, default="Average", verbose_name="数据聚合的方法")
    period = models.IntegerField(verbose_name='统计周期(分钟)', default=1, null=False)     #verbose_name表示admin管理时显示的中文
    alarm_interval = models.IntegerField(verbose_name="报警间隔(分钟)", default=3, null=False)
    max_alarm_times = models.IntegerField(null=False, default=15, verbose_name="报警上限次数")
    webhooks = models.ManyToManyField('Webhook', verbose_name="报警通知地址列表webhooks", blank=True)     #  blank=True admin新建数据允许为空

    def __str__(self):
        return self.alarm_name

    # admin后台项目中文显示设置
    class Meta:
        verbose_name = "报警规则"
        verbose_name_plural = "报警规则"
*/

// Rule 报警规则结构体
type Rule struct {
	id              int
	ctime           string
	namespace       string
	alarm_name      string
	metric_name     string
	rule_type       int
	upper_limit     float64
	lower_limit     float64
	period          int
	alarm_interval  int
	max_alarm_times int
	aggregate       string
}

// Resource 监控资源结构体
type Resource struct {
	id          int
	name        string
	uid         string
	namespace   string
	instance_id string
	region      string
}

// AlarmObj 报警内容
type AlarmObj struct {
	id                int
	alarm_time        string
	alarm_value       float64
	last_alarm_time   string
	hold_minutes      *int
	now_status        int
	recover_time      *string
	times             int
	alarm_resource_id int
	alarm_rule_id     int
}

// RuleResourceBound 规则资源关系表
type RuleResourceBound struct {
	id          int
	rule_id     int
	resource_id int
}

// RuleWebhookBound 规则webhook关系表
type RuleWebhookBound struct {
	id         int
	rule_id    int
	webhook_id int
}

// Webhook 通知地址
type Webhook struct {
	id           int
	name         string
	webhook_type int
	url          string
}

//==========↑↑数据库表结构体↑↑==============

// 飞书消息body 结构体，给json系列化用
type Tag struct {
	Tag      string `json:"tag"`
	Text     string `json:"text"`
	Href     string `json:"href"`
	UserId   string `json:"user_id"`
	UserName string `json:"user_name"`
	ImageKey string `json:"image_key"`
	UnEscape bool   `json:"un_escape"`
}

// FeishuContent 飞书内容接口空接口
type FeishuContent interface {
}

// ZhCN 富文本结构体内中文内容
type ZhCN struct {
	Title   string  `json:"title"`
	Content [][]Tag `json:"content"`
}

// Post 富文本结构体内容
type Post struct {
	ZhCN ZhCN `json:"zh_cn"`
}

// PostOuter Content内Post 结构体外壳
type PostOuter struct {
	Post Post `json:"post"`
}

// FeishuPostJsonBody 富文本结构体外壳
type FeishuPostJsonBody struct {
	Msg_type string        `json:"msg_type"`
	Content  FeishuContent `json:"content"`
}

// WechatMarkDown 企业微信 推送消息结构体MarkDown 内容
type WechatMarkDown struct {
	Content string `json:"content"`
}

// WechatData 企业微信 推送消息结构体
type WechatData struct {
	MsgType  string         `json:"msgtype"`
	MarkDown WechatMarkDown `json:"markdown"`
}

// IPResource IP资源结构体
type IPResource struct {
	id               int
	name             string
	customer_name    string
	uid              string
	genericIPAddress string
	location         string
}

// PingLossData IP丢包数据结构体
type PingLossData struct {
	id             int
	ctime          string
	float_value    float64
	ip_resource_id int
}

// IPPingLossRule IP丢包规则
type IPPingLossRule struct {
	id              int
	ctime           string
	alarm_name      string
	upper_limit     float64
	aggregate       string
	period          int
	alarm_interval  int
	max_alarm_times int
}

// IPPingLossAlarmList 报警规则
type IPPingLossAlarmList struct {
	id                int
	alarm_time        string
	last_alarm_time   string
	alarm_value       float64
	recover_time      string
	hold_minutes      int
	now_status        int
	times             int //报警次数
	alarm_resource_id int
	alarm_rule_id     int
}

// IPPingLossRuleIPResourceBond IP报警规则绑定IP资源中间表
type IPPingLossRuleIPResourceBond struct {
	id                int
	ippinglossrule_id int
	ipresource_id     int
}

// IPPingLossRuleWebhookBond IP报警规则绑定Webhook中间表
type IPPingLossRuleWebhookBond struct {
	id                int
	ippinglossrule_id int
	webhook_id        int
}

// WarnRule 空接口报警用
type WarnRule interface {
}

// WarnResource 报警资源空接口报警用
type WarnResource interface {
}

type HttpUrlRule struct {
	id                   int
	ctime                string
	url_name             string
	url                  string
	test_interval_second int
	healthy_times        int
	unhealthy_times      int
	alarm_interval       int
	max_alarm_times      int
	already_alarm_times  int
	last_alarm_time      string
	enable               bool
	healthy              int
	delete               bool
}

// HttpTestRuleWebhookBond IP报警规则绑定Webhook中间表
type HttpTestRuleWebhookBond struct {
	id              int
	httptestrule_id int
	webhook_id      int
}

// Announcement 公告
type Announcement struct {
	id         int
	name       string
	start_time string
	end_time   string
	content    string
}

// AnnouncementBondWebhook 绑定webhook
type AnnouncementBondWebhook struct {
	id              int
	announcement_id int
	webhook_id      int
}

// AnnouncementNoticeHistory 公告通知历史
type AnnouncementNoticeHistory struct {
	id                            int
	notice_three_days_advance     int
	notice_thirteen_hours_advance int
	notice_one_hours_advance      int
	announcement_id               int
	webhook_id                    int
	notice_one_hours_time         sql.NullString
	notice_thirteen_hours_time    sql.NullString
	notice_three_days_time        string
}
