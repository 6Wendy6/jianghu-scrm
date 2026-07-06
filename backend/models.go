package main

import (
	"database/sql"
	"encoding/json"
	redis "github.com/redis/go-redis/v9"
	"net/http"
	"sync"
	"time"
)

type Config struct {
	Addr                  string
	ReadHeaderTimeout     time.Duration
	ReadTimeout           time.Duration
	WriteTimeout          time.Duration
	IdleTimeout           time.Duration
	ShutdownTimeout       time.Duration
	RateLimitRPS          int
	RateLimitBurst        int
	APIToken              string
	DatabaseMode          string
	BusinessTimeZone      string
	BusinessLocation      *time.Location
	WorkerInterval        time.Duration
	WorkerBatchSize       int
	WorkerMaxAttempts     int
	WorkerRetryDelay      time.Duration
	DatabaseURL           string
	DBMaxOpenConns        int
	DBMaxIdleConns        int
	DBConnMaxLifetime     time.Duration
	DBPingTimeout         time.Duration
	AutoMigrate           bool
	CacheTTL              time.Duration
	CacheMaxEntries       int
	RedisAddr             string
	RedisPassword         string
	RedisDB               int
	RedisKeyPrefix        string
	SecretEncryptKey      string
	WeComCorpID           string
	WeComAgentID          string
	WeComSecret           string
	WeComCallbackToken    string
	WeComCallbackAESKey   string
	WeComTrustedIPHint    string
	WeComContactWayDryRun bool
	MonobaseBaseURL       string
	MonobaseToken         string
	MonobaseCorpID        string
	MonobasePageSize      int
}

type requestIDKey struct{}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type ipRateLimiter struct {
	mu        sync.Mutex
	rps       float64
	burst     float64
	clients   map[string]*tokenBucket
	lastSweep time.Time
}

type cacheEntry struct {
	body      []byte
	expiresAt time.Time
}

type responseCache struct {
	mu         sync.RWMutex
	ttl        time.Duration
	maxEntries int
	entries    map[string]cacheEntry
	lastSweep  time.Time
}

type cacheStore interface {
	Get(key string) ([]byte, bool)
	Set(key string, body []byte)
	DeletePrefix(prefix string)
	Stats() map[string]any
	Close() error
}

type redisResponseCache struct {
	client    *redis.Client
	ttl       time.Duration
	keyPrefix string
}

type endpointMetric struct {
	Requests      uint64
	Errors        uint64
	DurationSumMS float64
	Buckets       []uint64
}

type apiMetrics struct {
	mu        sync.Mutex
	startedAt time.Time
	bucketsMS []float64
	endpoints map[string]*endpointMetric
}

type API struct {
	mu               sync.RWMutex
	stores           []Store
	guides           []Guide
	customers        []Customer
	handover         []HandoverItem
	touches          []TouchRule
	groups           []CustomerGroup
	groupMassTasks   []GroupMassTask
	groupWelcomes    []GroupWelcome
	groupSOPs        []GroupSOP
	groupCalendar    []GroupCalendarEvent
	groupReminders   []GroupReminder
	groupTagGroups   []GroupTagGroup
	tags             []Tag
	tagGroups        []TagGroup
	autoRules        []AutoTagRule
	preTagRules      []PreTagRule
	opsCustomers     []OpsCustomer
	opsLifecycleLogs []OpsLifecycleLog
	opsTags          []OpsCustomerTag
	opsTagLinks      []OpsCustomerTagRelation
	opsFollowups     []OpsFollowUpRecord
	opsTasks         []OpsTask
	opsTaskLogs      []OpsTaskLog
	opsMaterials     []OpsMaterial
	opsTimelines     []OpsTimelineEvent
	opsExceptions    []OpsException
	events           []EventEnvelope
	tasks            []TaskRecord
	taskItems        []TaskItemRecord
	db               *sql.DB
	storageMode      string
	cache            cacheStore
	metrics          *apiMetrics
	wecomSynced      bool
	secretCipher     secretCipher
	config           Config
}

type PageMeta struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"pageSize"`
	Total      int  `json:"total"`
	TotalPages int  `json:"totalPages"`
	HasNext    bool `json:"hasNext"`
}

type PageResult[T any] struct {
	Data []T      `json:"data"`
	Page PageMeta `json:"page"`
}

type OpsCustomer struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Nickname            string   `json:"nickname"`
	Mobile              string   `json:"mobile"`
	Avatar              string   `json:"avatar"`
	SourceChannel       string   `json:"sourceChannel"`
	RegionID            string   `json:"regionId"`
	RegionName          string   `json:"regionName"`
	StoreID             string   `json:"storeId"`
	StoreName           string   `json:"storeName"`
	OwnerGuideID        string   `json:"ownerGuideId"`
	OwnerGuideName      string   `json:"ownerGuideName"`
	LifecycleStage      string   `json:"lifecycleStage"`
	IntentionLevel      string   `json:"intentionLevel"`
	Status              string   `json:"status"`
	AddWeComTime        string   `json:"addWecomTime"`
	LastFollowUpTime    string   `json:"lastFollowUpTime"`
	LastInteractionTime string   `json:"lastInteractionTime"`
	DealStatus          string   `json:"dealStatus"`
	Risk                string   `json:"risk"`
	Tags                []string `json:"tags"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

type OpsLifecycleLog struct {
	ID           string `json:"id"`
	CustomerID   string `json:"customerId"`
	StageBefore  string `json:"stageBefore"`
	StageAfter   string `json:"stageAfter"`
	Reason       string `json:"reason"`
	OperatorID   string `json:"operatorId"`
	OperatorName string `json:"operatorName"`
	CreatedAt    string `json:"createdAt"`
}

type OpsCustomerTag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Color     string `json:"color"`
	Source    string `json:"source"`
	IsEnabled bool   `json:"isEnabled"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type OpsCustomerTagRelation struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	TagID      string `json:"tagId"`
	Source     string `json:"source"`
	OperatorID string `json:"operatorId"`
	CreatedAt  string `json:"createdAt"`
}

type OpsFollowUpRecord struct {
	ID               string `json:"id"`
	CustomerID       string `json:"customerId"`
	StoreID          string `json:"storeId"`
	GuideID          string `json:"guideId"`
	FollowUpType     string `json:"followUpType"`
	Content          string `json:"content"`
	Result           string `json:"result"`
	NextFollowUpTime string `json:"nextFollowUpTime"`
	StageBefore      string `json:"stageBefore"`
	StageAfter       string `json:"stageAfter"`
	CreatedBy        string `json:"createdBy"`
	CreatedAt        string `json:"createdAt"`
}

type OpsTask struct {
	ID               string `json:"id"`
	TaskType         string `json:"taskType"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	CustomerID       string `json:"customerId"`
	CustomerName     string `json:"customerName"`
	RegionID         string `json:"regionId"`
	RegionName       string `json:"regionName"`
	StoreID          string `json:"storeId"`
	StoreName        string `json:"storeName"`
	AssignedToUserID string `json:"assignedToUserId"`
	AssignedToName   string `json:"assignedToName"`
	AssignedToRole   string `json:"assignedToRole"`
	Priority         string `json:"priority"`
	Status           string `json:"status"`
	DueTime          string `json:"dueTime"`
	CompletedAt      string `json:"completedAt"`
	Source           string `json:"source"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type OpsTaskLog struct {
	ID           string `json:"id"`
	TaskID       string `json:"taskId"`
	Action       string `json:"action"`
	OldStatus    string `json:"oldStatus"`
	NewStatus    string `json:"newStatus"`
	Remark       string `json:"remark"`
	OperatorID   string `json:"operatorId"`
	OperatorName string `json:"operatorName"`
	CreatedAt    string `json:"createdAt"`
}

type OpsTimelineEvent struct {
	ID           string `json:"id"`
	CustomerID   string `json:"customerId"`
	EventType    string `json:"eventType"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	RelatedID    string `json:"relatedId"`
	OperatorID   string `json:"operatorId"`
	OperatorName string `json:"operatorName"`
	CreatedAt    string `json:"createdAt"`
}

type OpsMaterial struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	MaterialType    string   `json:"materialType"`
	ApplicableStage string   `json:"applicableStage"`
	ApplicableTags  []string `json:"applicableTags"`
	Status          string   `json:"status"`
	CreatedBy       string   `json:"createdBy"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

type OpsException struct {
	ID            string `json:"id"`
	ExceptionType string `json:"exceptionType"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Severity      string `json:"severity"`
	RegionID      string `json:"regionId"`
	RegionName    string `json:"regionName"`
	StoreID       string `json:"storeId"`
	StoreName     string `json:"storeName"`
	CustomerID    string `json:"customerId"`
	CustomerName  string `json:"customerName"`
	TaskID        string `json:"taskId"`
	AssignedTo    string `json:"assignedTo"`
	Status        string `json:"status"`
	Suggestion    string `json:"suggestion"`
	CreatedAt     string `json:"createdAt"`
	ResolvedAt    string `json:"resolvedAt"`
}

type OpsBootstrap struct {
	Customers  []OpsCustomer    `json:"customers"`
	Tasks      []OpsTask        `json:"tasks"`
	Materials  []OpsMaterial    `json:"materials"`
	Exceptions []OpsException   `json:"exceptions"`
	Tags       []OpsCustomerTag `json:"tags"`
}

type Store struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	InternalCode string      `json:"internalCode"`
	ExternalCode string      `json:"externalCode"`
	Brand        string      `json:"brand"`
	Region       string      `json:"region"`
	BrandRegion  string      `json:"brandRegion"`
	Type         string      `json:"type"`
	GuideCount   int         `json:"guideCount"`
	PoolCount    int         `json:"poolCount"`
	Config       StoreConfig `json:"config"`
}

type StoreConfig struct {
	EntryMode    string `json:"entryMode"`
	ServiceGuide string `json:"serviceGuide"`
	Group        string `json:"group"`
	Welcome      string `json:"welcome"`
	Polling      string `json:"polling"`
}

type Guide struct {
	ID               string       `json:"id"`
	StoreID          string       `json:"storeId"`
	Name             string       `json:"name"`
	Code             string       `json:"code"`
	Count            string       `json:"count"`
	TotalPool        int          `json:"totalPool"`
	TodayPool        int          `json:"todayPool"`
	Status           string       `json:"status"`
	EmploymentStatus string       `json:"employmentStatus"`
	Paused           bool         `json:"paused"`
	Handling         string       `json:"handling"`
	Lifecycle        []AuditEvent `json:"lifecycle"`
}

type Relation struct {
	ID       string `json:"id"`
	GuideID  string `json:"guideId"`
	Guide    string `json:"guide"`
	Code     string `json:"code"`
	Store    string `json:"store"`
	LinkedAt string `json:"linkedAt"`
	EndedAt  string `json:"endedAt"`
	Main     bool   `json:"main"`
}

type Customer struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Wecom            string       `json:"wecom"`
	Mobile           string       `json:"mobile"`
	Owner            string       `json:"owner"`
	Source           string       `json:"source"`
	SourceStore      string       `json:"sourceStore"`
	Stage            string       `json:"stage"`
	TagGroup         string       `json:"tagGroup"`
	Tags             []string     `json:"tags"`
	WecomTags        []string     `json:"wecomTags"`
	LastActive       string       `json:"lastActive"`
	IntentScore      int          `json:"intentScore"`
	SalesStage       string       `json:"salesStage"`
	ConversionSource string       `json:"conversionSource"`
	DealAmount       string       `json:"dealAmount"`
	NextFollowUp     string       `json:"nextFollowUp"`
	Signals          []string     `json:"signals"`
	Relations        []Relation   `json:"relations"`
	Timeline         []AuditEvent `json:"timeline"`
}

type HandoverItem struct {
	ID             string `json:"id"`
	CustomerID     string `json:"customerId"`
	Customer       string `json:"customer"`
	Reason         string `json:"reason"`
	CurrentOwner   string `json:"currentOwner"`
	RequiredAction string `json:"requiredAction"`
	Receiver       string `json:"receiver"`
	Status         string `json:"status"`
}

type TouchRule struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Scope  string `json:"scope"`
	Status string `json:"status"`
}

type CustomerGroup struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Store      string   `json:"store"`
	Owner      string   `json:"owner"`
	Tags       []string `json:"tags"`
	Count      int      `json:"count"`
	TodayJoin  int      `json:"todayJoin"`
	TodayQuit  int      `json:"todayQuit"`
	CreatedAt  string   `json:"createdAt"`
	Status     string   `json:"status"`
	TodayEvent string   `json:"todayEvent"`
}

type GroupMassTask struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Groups  []string `json:"groups"`
	Content string   `json:"content"`
	Status  string   `json:"status"`
}

type GroupWelcome struct {
	ID      string `json:"id"`
	Group   string `json:"group"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

type GroupSOP struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Groups  []string `json:"groups"`
	Stage   string   `json:"stage"`
	Content string   `json:"content"`
	Status  string   `json:"status"`
}

type GroupCalendarEvent struct {
	ID     string `json:"id"`
	Group  string `json:"group"`
	Title  string `json:"title"`
	Date   string `json:"date"`
	Owner  string `json:"owner"`
	Status string `json:"status"`
}

type GroupReminder struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Trigger string `json:"trigger"`
	Owner   string `json:"owner"`
	Status  string `json:"status"`
}

type GroupTagGroup struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Tags   []string `json:"tags"`
	Scope  string   `json:"scope"`
	Owner  string   `json:"owner"`
	Status string   `json:"status"`
}

type Tag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	Status    string `json:"status"`
	Customers int    `json:"customers"`
	Wecom     string `json:"wecom"`
}

type TagGroup struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Stores []string `json:"stores"`
	Tags   []string `json:"tags"`
	Order  int      `json:"order"`
}

type AutoTagRule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Trigger string   `json:"trigger"`
	Actions []string `json:"actions"`
	Scope   string   `json:"scope"`
	Impact  int      `json:"impact"`
	Status  string   `json:"status"`
}

type PreTagRule struct {
	ID      string   `json:"id"`
	Entry   string   `json:"entry"`
	Tags    []string `json:"tags"`
	Trigger string   `json:"trigger"`
	Period  string   `json:"period"`
	Status  string   `json:"status"`
}

type AuditEvent struct {
	ID       string `json:"id"`
	Time     string `json:"time"`
	Action   string `json:"action"`
	Result   string `json:"result"`
	Operator string `json:"operator"`
	People   string `json:"people"`
	Detail   string `json:"detail"`
}

type QRDownload struct {
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	Content  string `json:"content"`
}

type EventEnvelope struct {
	ID             string          `json:"id"`
	Source         string          `json:"source"`
	Type           string          `json:"type"`
	ObjectID       string          `json:"objectId"`
	IdempotencyKey string          `json:"idempotencyKey"`
	Status         string          `json:"status"`
	Attempts       int             `json:"attempts"`
	ReceivedAt     string          `json:"receivedAt"`
	ProcessedAt    string          `json:"processedAt,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type TaskRecord struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Status       string          `json:"status"`
	TotalCount   int             `json:"totalCount"`
	SuccessCount int             `json:"successCount"`
	FailedCount  int             `json:"failedCount"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	CreatedBy    string          `json:"createdBy"`
	CreatedAt    string          `json:"createdAt"`
	StartedAt    string          `json:"startedAt,omitempty"`
	FinishedAt   string          `json:"finishedAt,omitempty"`
	LastError    string          `json:"lastError,omitempty"`
}

type TaskItemRecord struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"taskId"`
	TargetID  string          `json:"targetId"`
	Status    string          `json:"status"`
	Attempts  int             `json:"attempts"`
	Result    json.RawMessage `json:"result,omitempty"`
	LastError string          `json:"lastError,omitempty"`
	UpdatedAt string          `json:"updatedAt"`
}

type BatchTagTaskPayload struct {
	CustomerIDs []string `json:"customerIds"`
	Tags        []string `json:"tags"`
	Operator    string   `json:"operator"`
}
