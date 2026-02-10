package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/gogf/gf/v2/frame/g"
)

// ClickHouseManager ClickHouse连接管理器
type ClickHouseManager struct {
	conn   driver.Conn
	mutex  sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	chManager     *ClickHouseManager
	chManagerOnce sync.Once
	chInitOnce    sync.Once
	chInitErr     error
)

// GetClickHouseManager 获取ClickHouse管理器单例
func GetClickHouseManager() *ClickHouseManager {
	chManagerOnce.Do(func() {
		chManager = &ClickHouseManager{}
	})
	return chManager
}

// InitClickHouse 初始化ClickHouse连接（只执行一次）
func InitClickHouse(ctx context.Context) error {
	chInitOnce.Do(func() {
		host := g.Cfg().MustGet(ctx, "clickhouse.host").String()
		port := g.Cfg().MustGet(ctx, "clickhouse.port").Int()
		database := g.Cfg().MustGet(ctx, "clickhouse.database").String()
		username := g.Cfg().MustGet(ctx, "clickhouse.username").String()
		password := g.Cfg().MustGet(ctx, "clickhouse.password").String()

		chInitErr = GetClickHouseManager().Init(host, port, database, username, password)
	})
	return chInitErr
}

// Init 初始化ClickHouse连接
func (m *ClickHouseManager) Init(host string, port int, database, username, password string) error {
	m.ctx, m.cancel = context.WithCancel(context.Background())

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", host, port)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:      time.Second * 10,
		MaxOpenConns:     10,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
		BlockBufferSize:  10,
	})
	if err != nil {
		return fmt.Errorf("连接ClickHouse失败: %v", err)
	}

	if err := conn.Ping(m.ctx); err != nil {
		return fmt.Errorf("ClickHouse连接测试失败: %v", err)
	}

	m.mutex.Lock()
	m.conn = conn
	m.mutex.Unlock()

	g.Log().Infof(m.ctx, "ClickHouse连接初始化成功: %s:%d/%s", host, port, database)
	return nil
}

// GetConn 获取ClickHouse连接
func (m *ClickHouseManager) GetConn() driver.Conn {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.conn
}

// Close 关闭ClickHouse连接
func (m *ClickHouseManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			g.Log().Errorf(m.ctx, "关闭ClickHouse连接失败: %v", err)
			return err
		}
	}
	return nil
}
