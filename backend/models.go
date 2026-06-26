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
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	RateLimitRPS      int
	RateLimitBurst    int
	APIToken          string
	BusinessTimeZone  string
	BusinessLocation  *time.Location
	WorkerInterval    time.Duration
	WorkerBatchSize   int
	WorkerMaxAttempts int
	WorkerRetryDelay  time.Duration
	DatabaseURL       string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBPingTimeout     time.Duration
	AutoMigrate       bool
	CacheTTL          time.Duration
	CacheMaxEntries   int
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	RedisKeyPrefix    string
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
	mu             sync.RWMutex
	stores         []Store
	guides         []Guide
	customers      []Customer
	handover       []HandoverItem
	touches        []TouchRule
	groups         []CustomerGroup
	groupMassTasks []GroupMassTask
	groupWelcomes  []GroupWelcome
	groupSOPs      []GroupSOP
	groupCalendar  []GroupCalendarEvent
	groupReminders []GroupReminder
	groupTagGroups []GroupTagGroup
	tags           []Tag
	tagGroups      []TagGroup
	autoRules      []AutoTagRule
	preTagRules    []PreTagRule
	events         []EventEnvelope
	tasks          []TaskRecord
	taskItems      []TaskItemRecord
	db             *sql.DB
	storageMode    string
	cache          cacheStore
	metrics        *apiMetrics
	wecomSynced    bool
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
