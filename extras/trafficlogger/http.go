package trafficlogger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/apernet/hysteria/core/v2/server"
)

const (
	indexHTML = `<!DOCTYPE html><html lang="en"><head> <meta charset="UTF-8"> <meta name="viewport" content="width=device-width, initial-scale=1.0"> <title>Hysteria Traffic Stats API Server</title> <style>body{font-family: Arial, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; padding: 0; background-color: #f4f4f4;}.container{padding: 20px; background-color: #fff; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1); border-radius: 5px;}</style></head><body> <div class="container"> <p>This is a Hysteria Traffic Stats API server.</p><p>Check the documentation for usage.</p></div></body></html>`
)

// TrafficStatsServer implements both server.TrafficLogger and http.Handler
// to provide a simple HTTP API to get the traffic stats per user.
type TrafficStatsServer interface {
	server.TrafficLogger
	http.Handler
}

func NewTrafficStatsServer(secret string) TrafficStatsServer {
	return &trafficStatsServerImpl{
		StatsMap:  make(map[string]*trafficStatsEntry),
		KickMap:   make(map[string]struct{}),
		OnlineMap: make(map[string]int),
		Secret:    secret,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				IdleConnTimeout:     30 * time.Second,
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 2,
			},
		},
	}
}

type TrafficPushRequest struct {
	Data map[string][2]int64
}

// 定时提交用户流量情况
func (s *trafficStatsServerImpl) PushTrafficToV2boardInterval(url string, interval time.Duration) {
	fmt.Println("用户流量情况监控已启动,提交周期为:", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.PushTrafficToV2board(url); err != nil {
			fmt.Println("用户流量信息提交失败:", err)
		}
	}

}

// 向v2board 提交用户流量使用情况
func (s *trafficStatsServerImpl) PushTrafficToV2board(url string) error {
	var requestData map[string][2]int64

	s.Mutex.Lock()
	if len(s.StatsMap) == 0 {
		s.Mutex.Unlock()
		return nil
	}
	
	requestData = make(map[string][2]int64)
	for id, stats := range s.StatsMap {
		requestData[id] = [2]int64{int64(stats.Tx), int64(stats.Rx)}
	}
	
	// 无论成功与否先清空，避免累积。并且必须尽早释放写锁！
	// 旧代码在锁内存锁时进行了长时间 HTTP 网络请求，会导致 Hysteria 所有的流量统计 Goroutines 全部严重阻塞、挤压甚至 OOM 内存爆炸。
	s.StatsMap = make(map[string]*trafficStatsEntry)
	s.Mutex.Unlock() // 释放锁后，再进行网络通信

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}
	
	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("V2board panel API Push error:", err)
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // Read body to ensure HTTP connection reuse

	if resp.StatusCode != http.StatusOK {
		return errors.New("HTTP request failed with status code: " + resp.Status)
	}

	return nil
}

type trafficStatsServerImpl struct {
	Mutex      sync.RWMutex
	StatsMap   map[string]*trafficStatsEntry
	OnlineMap  map[string]int
	KickMap    map[string]struct{}
	Secret     string
	httpClient *http.Client
}

type trafficStatsEntry struct {
	Tx uint64 `json:"tx"`
	Rx uint64 `json:"rx"`
}

func (s *trafficStatsServerImpl) LogTraffic(id string, tx, rx uint64) (ok bool) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	_, ok = s.KickMap[id]
	if ok {
		delete(s.KickMap, id)
		return false
	}

	entry, ok := s.StatsMap[id]
	if !ok {
		entry = &trafficStatsEntry{}
		s.StatsMap[id] = entry
	}
	entry.Tx += tx
	entry.Rx += rx

	return true
}

// LogOnlineStateChanged updates the online state to the online map.
func (s *trafficStatsServerImpl) LogOnlineState(id string, online bool) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	if online {
		s.OnlineMap[id]++
	} else {
		s.OnlineMap[id]--
		if s.OnlineMap[id] <= 0 {
			delete(s.OnlineMap, id)
		}
	}
}

func (s *trafficStatsServerImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Secret != "" && r.Header.Get("Authorization") != s.Secret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/" {
		_, _ = w.Write([]byte(indexHTML))
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/traffic" {
		s.getTraffic(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/kick" {
		s.kick(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/online" {
		s.getOnline(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *trafficStatsServerImpl) getTraffic(w http.ResponseWriter, r *http.Request) {
	bClear, _ := strconv.ParseBool(r.URL.Query().Get("clear"))
	var jb []byte
	var err error
	if bClear {
		s.Mutex.Lock()
		jb, err = json.Marshal(s.StatsMap)
		s.StatsMap = make(map[string]*trafficStatsEntry)
		s.Mutex.Unlock()
	} else {
		s.Mutex.RLock()
		jb, err = json.Marshal(s.StatsMap)
		s.Mutex.RUnlock()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(jb)
}

func (s *trafficStatsServerImpl) getOnline(w http.ResponseWriter, r *http.Request) {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()

	jb, err := json.Marshal(s.OnlineMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(jb)
}

func (s *trafficStatsServerImpl) kick(w http.ResponseWriter, r *http.Request) {
	var ids []string
	err := json.NewDecoder(r.Body).Decode(&ids)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.Mutex.Lock()
	for _, id := range ids {
		// Only add to KickMap if the user is currently online
		if _, online := s.OnlineMap[id]; online {
			s.KickMap[id] = struct{}{}
		}
	}
	s.Mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

// 踢出用户名单
func (s *trafficStatsServerImpl) NewKick(id string) bool {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	// Only add to KickMap if the user is currently online
	if _, online := s.OnlineMap[id]; online {
		s.KickMap[id] = struct{}{}
	}
	return true
}
