package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRetentionDays    = 30
	defaultMaxRecentRecords = 10000
	defaultWriteQueueSize   = 4096
	defaultFlushInterval    = time.Second
	defaultQueryCacheTTL    = 2 * time.Second
	callFilePrefix          = "calls-"
	callFileSuffix          = ".jsonl"

	UsageSourceUpstream = "upstream"
	UsageSourceUnknown  = "unknown"
)

type Options struct {
	Dir              string
	RetentionDays    int
	MaxRecentRecords int
	Now              func() time.Time
}

type CallRecord struct {
	ID               string    `json:"id"`
	At               time.Time `json:"at"`
	ProviderID       string    `json:"provider_id"`
	Model            string    `json:"model,omitempty"`
	Endpoint         string    `json:"endpoint"`
	Method           string    `json:"method"`
	Status           int       `json:"status"`
	Success          bool      `json:"success"`
	Stream           bool      `json:"stream,omitempty"`
	LatencyMs        int64     `json:"latency_ms"`
	Attempts         int       `json:"attempts"`
	KeyID            string    `json:"key_id,omitempty"`
	PromptTokens     *int64    `json:"prompt_tokens,omitempty"`
	CompletionTokens *int64    `json:"completion_tokens,omitempty"`
	TotalTokens      *int64    `json:"total_tokens,omitempty"`
	UsageSource      string    `json:"usage_source"`
	Error            string    `json:"error,omitempty"`
}

type Store struct {
	dir              string
	retentionDays    int
	maxRecentRecords int
	now              func() time.Time
	mu               sync.RWMutex
	records          []CallRecord
	seq              atomic.Uint64
	dropped          atomic.Uint64
	lastCleanupDay   string
	commands         chan writeCommand
	writerDone       chan struct{}
	commandMu        sync.RWMutex
	closed           atomic.Bool
	closeOnce        sync.Once
	closeErr         error
	queryCacheMu     sync.Mutex
	recordsCache     map[recordsCacheKey]recordsCacheEntry
}

type writeCommand struct {
	record    CallRecord
	hasRecord bool
	flush     chan error
	close     chan error
}

type recordsCacheKey struct {
	sinceUnixNano int64
}

type recordsCacheEntry struct {
	expiresAt time.Time
	records   []CallRecord
}

func NewStore(options Options) (*Store, error) {
	if options.Dir == "" {
		return nil, fmt.Errorf("stats dir is required")
	}
	if options.RetentionDays <= 0 {
		options.RetentionDays = defaultRetentionDays
	}
	if options.MaxRecentRecords <= 0 {
		options.MaxRecentRecords = defaultMaxRecentRecords
	}
	if options.Now == nil {
		options.Now = time.Now
	}

	store := &Store{
		dir:              options.Dir,
		retentionDays:    options.RetentionDays,
		maxRecentRecords: options.MaxRecentRecords,
		now:              options.Now,
		commands:         make(chan writeCommand, defaultWriteQueueSize),
		writerDone:       make(chan struct{}),
	}
	if err := os.MkdirAll(store.dir, 0755); err != nil {
		return nil, fmt.Errorf("create stats dir: %w", err)
	}
	if err := store.cleanupExpiredFiles(); err != nil {
		return nil, err
	}
	if err := store.loadRecentRecords(); err != nil {
		return nil, err
	}
	store.lastCleanupDay = store.now().UTC().Format(time.DateOnly)
	go store.runWriter()
	return store, nil
}

func (s *Store) Append(record CallRecord) error {
	if s == nil {
		return nil
	}
	record = s.normalizeRecord(record)
	if err := s.cleanupIfDayChanged(record.At); err != nil {
		return err
	}

	s.commandMu.RLock()
	defer s.commandMu.RUnlock()
	if s.closed.Load() {
		return fmt.Errorf("stats store is closed")
	}
	select {
	case s.commands <- writeCommand{record: record, hasRecord: true}:
		s.addRecent(record)
		return nil
	default:
		s.dropped.Add(1)
		return nil
	}
}

func (s *Store) Recent(limit int) []CallRecord {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.records) {
		limit = len(s.records)
	}
	start := len(s.records) - limit
	out := make([]CallRecord, limit)
	copy(out, s.records[start:])
	return out
}

func (s *Store) nextID(at time.Time) string {
	return fmt.Sprintf("%d-%06d", at.UnixNano(), s.seq.Add(1))
}

func (s *Store) normalizeRecord(record CallRecord) CallRecord {
	if record.At.IsZero() {
		record.At = s.now().UTC()
	} else {
		record.At = record.At.UTC()
	}
	if record.ID == "" {
		record.ID = s.nextID(record.At)
	}
	if record.Attempts <= 0 {
		record.Attempts = 1
	}
	if record.UsageSource == "" {
		record.UsageSource = UsageSourceUnknown
	}
	normalizeUsageFields(&record)
	return record
}

func (s *Store) addRecent(record CallRecord) {
	s.mu.Lock()
	s.records = append(s.records, record)
	s.capRecentLocked()
	s.mu.Unlock()
}

func (s *Store) DroppedRecords() uint64 {
	if s == nil {
		return 0
	}
	return s.dropped.Load()
}

func (s *Store) QueueDepth() int {
	if s == nil || s.commands == nil {
		return 0
	}
	return len(s.commands)
}

func (s *Store) QueueCapacity() int {
	if s == nil || s.commands == nil {
		return 0
	}
	return cap(s.commands)
}

func (s *Store) filePath(at time.Time) string {
	return filepath.Join(s.dir, callFilePrefix+at.UTC().Format(time.DateOnly)+callFileSuffix)
}

func (s *Store) cleanupIfDayChanged(at time.Time) error {
	day := at.UTC().Format(time.DateOnly)
	s.mu.RLock()
	lastCleanupDay := s.lastCleanupDay
	s.mu.RUnlock()
	if day == lastCleanupDay {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if day == s.lastCleanupDay {
		return nil
	}
	if err := s.cleanupExpiredFiles(); err != nil {
		return err
	}
	s.pruneExpiredRecordsLocked(at)
	s.lastCleanupDay = day
	return nil
}

func (s *Store) cleanupExpiredFiles() error {
	files, err := filepath.Glob(filepath.Join(s.dir, callFilePrefix+"*"+callFileSuffix))
	if err != nil {
		return fmt.Errorf("scan stats files: %w", err)
	}
	cutoff := cutoffDate(s.now().UTC(), s.retentionDays)
	for _, file := range files {
		fileDate, ok := parseCallFileDate(filepath.Base(file))
		if !ok || !fileDate.Before(cutoff) {
			continue
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove expired stats file: %w", err)
		}
	}
	return nil
}

func (s *Store) loadRecentRecords() error {
	files, err := filepath.Glob(filepath.Join(s.dir, callFilePrefix+"*"+callFileSuffix))
	if err != nil {
		return fmt.Errorf("scan stats files: %w", err)
	}
	sort.Strings(files)
	sort.SliceStable(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	cutoff := cutoffDate(s.now().UTC(), s.retentionDays)
	// 启动时只加载最近 2 天的文件到内存，减少启动 I/O。
	// 更早的记录按需通过 recordsSinceFromFiles 从文件查询。
	loadCutoff := s.now().UTC().AddDate(0, 0, -1)
	records := make([]CallRecord, 0)
	for _, file := range files {
		fileDate, ok := parseCallFileDate(filepath.Base(file))
		if !ok || fileDate.Before(cutoff) {
			continue
		}
		if fileDate.Before(loadCutoff) {
			continue
		}
		if err := loadRecordsFromFile(file, func(record CallRecord) {
			records = append(records, record)
		}); err != nil {
			return err
		}
		if s.maxRecentRecords > 0 && len(records) >= s.maxRecentRecords {
			break
		}
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].At.Before(records[j].At)
	})
	s.records = records
	s.capRecentLocked()
	return nil
}

func (s *Store) recordsSinceFromFiles(since time.Time) ([]CallRecord, error) {
	if s == nil {
		return nil, nil
	}
	if err := s.Flush(); err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(s.dir, callFilePrefix+"*"+callFileSuffix))
	if err != nil {
		return nil, fmt.Errorf("scan stats files: %w", err)
	}
	sort.Strings(files)

	records := make([]CallRecord, 0)
	for _, file := range files {
		fileDate, ok := parseCallFileDate(filepath.Base(file))
		if !ok {
			continue
		}
		if fileDate.Add(24 * time.Hour).Before(since) {
			continue
		}
		if err := loadRecordsFromFile(file, func(record CallRecord) {
			if record.At.IsZero() || record.At.Before(since) {
				return
			}
			records = append(records, record)
		}); err != nil {
			return nil, err
		}
	}

	return records, nil
}

func (s *Store) Flush() error {
	if s == nil {
		return nil
	}
	if s.closed.Load() {
		<-s.writerDone
		return s.closeErr
	}

	s.commandMu.RLock()
	if s.closed.Load() {
		s.commandMu.RUnlock()
		<-s.writerDone
		return s.closeErr
	}
	done := make(chan error, 1)
	// 使用 select + default 防止 channel 满时阻塞调用方；
	// 极端竞态下 Close 可能已开始消费 channel，此时 flush 会被丢弃。
	select {
	case s.commands <- writeCommand{flush: done}:
		s.commandMu.RUnlock()
		return <-done
	default:
		s.commandMu.RUnlock()
		return nil
	}
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.commandMu.Lock()
		s.closed.Store(true)
		done := make(chan error, 1)
		s.commands <- writeCommand{close: done}
		s.commandMu.Unlock()
		s.closeErr = <-done
		<-s.writerDone
	})
	return s.closeErr
}

func (s *Store) runWriter() {
	ticker := time.NewTicker(defaultFlushInterval)
	defer ticker.Stop()
	defer close(s.writerDone)

	var file *os.File
	var buf *bufio.Writer
	var currentDay string
	var lastErr error

	closeFile := func() error {
		if file == nil {
			return nil
		}
		if buf != nil {
			_ = buf.Flush()
			buf = nil
		}
		err := file.Close()
		file = nil
		currentDay = ""
		return err
	}

	flushFile := func() error {
		if file == nil {
			return lastErr
		}
		if buf != nil {
			if err := buf.Flush(); err != nil {
				lastErr = err
				return err
			}
		}
		if err := file.Sync(); err != nil {
			lastErr = err
			return err
		}
		return lastErr
	}

	for {
		select {
		case cmd := <-s.commands:
			switch {
			case cmd.hasRecord:
				if err := s.writeRecord(&file, &buf, &currentDay, cmd.record); err != nil {
					lastErr = err
				}
			case cmd.flush != nil:
				cmd.flush <- flushFile()
			case cmd.close != nil:
				err := flushFile()
				if closeErr := closeFile(); err == nil {
					err = closeErr
				}
				cmd.close <- err
				return
			}
		case <-ticker.C:
			_ = flushFile()
		}
	}
}

func (s *Store) writeRecord(file **os.File, buf **bufio.Writer, currentDay *string, record CallRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode stats record: %w", err)
	}
	data = append(data, '\n')

	day := record.At.UTC().Format(time.DateOnly)
	if *file == nil || *currentDay != day {
		if *file != nil {
			if *buf != nil {
				_ = (*buf).Flush()
				*buf = nil
			}
			if err := (*file).Close(); err != nil {
				return fmt.Errorf("close stats file: %w", err)
			}
			*file = nil
		}
		if err := os.MkdirAll(s.dir, 0755); err != nil {
			return fmt.Errorf("create stats dir: %w", err)
		}
		nextFile, err := os.OpenFile(s.filePath(record.At), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("open stats file: %w", err)
		}
		*file = nextFile
		*buf = bufio.NewWriter(nextFile)
		*currentDay = day
	}

	if _, err := (*buf).Write(data); err != nil {
		return fmt.Errorf("write stats file: %w", err)
	}
	return nil
}

func loadRecordsFromFile(path string, add func(CallRecord)) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open stats file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record CallRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		normalizeUsageFields(&record)
		add(record)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stats file: %w", err)
	}
	return nil
}

func (s *Store) capRecentLocked() {
	if s.maxRecentRecords <= 0 || len(s.records) <= s.maxRecentRecords {
		return
	}
	start := len(s.records) - s.maxRecentRecords
	next := make([]CallRecord, s.maxRecentRecords)
	copy(next, s.records[start:])
	s.records = next
}

func (s *Store) pruneExpiredRecordsLocked(now time.Time) {
	cutoff := cutoffDate(now.UTC(), s.retentionDays)
	kept := s.records[:0]
	for _, record := range s.records {
		if record.At.IsZero() || !record.At.Before(cutoff) {
			kept = append(kept, record)
		}
	}
	s.records = kept
	s.capRecentLocked()
}

func cutoffDate(now time.Time, retentionDays int) time.Time {
	if retentionDays <= 0 {
		retentionDays = defaultRetentionDays
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -retentionDays+1)
}

func parseCallFileDate(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, callFilePrefix) || !strings.HasSuffix(name, callFileSuffix) {
		return time.Time{}, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(name, callFilePrefix), callFileSuffix)
	parsed, err := time.ParseInLocation(time.DateOnly, raw, time.UTC)
	return parsed, err == nil
}
