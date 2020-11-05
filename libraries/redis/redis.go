package redis

import (
	"fmt"
	"gin-api/configs"
	"gin-api/libraries/logging"
	"strings"
	"time"

	"gin-api/libraries/config"
	util_error "gin-api/libraries/util/error"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
)

type RedisDB struct {
	pool   *redis.Pool
	Config *Config
}

var obj map[string]*RedisDB

func GetRedis(redisName string) *RedisDB {
	fileCfg := config.GetConfig("redis", redisName)

	hostCfg := fileCfg.Key("host").String()
	passwordCfg := fileCfg.Key("auth").String()
	portCfg, err := fileCfg.Key("port").Int()
	dbCfg, err := fileCfg.Key("db").Int()
	maxActiveCfg, err := fileCfg.Key("max_active").Int()
	maxIdleCfg, err := fileCfg.Key("max_idle").Int()
	execTime, err := fileCfg.Key("exec_timeout").Int64()
	util_error.Must(err)

	db, err := conn(redisName, hostCfg, passwordCfg, portCfg, dbCfg, maxActiveCfg, maxIdleCfg, execTime)
	util_error.Must(err)

	return db
}

func conn(conn, host, password string, port, dbNum, maxActive, maxIdle int, execTimeout int64) (db *RedisDB, err error) {
	if len(obj) == 0 {
		obj = make(map[string]*RedisDB)
	}
	if obj[conn] != nil {
		db = obj[conn]
		return
	}

	cfg := &Config{
		Host:        host,
		Port:        port,
		Password:    password,
		DB:          dbNum,
		MaxActive:   maxActive,
		MaxIdle:     maxIdle,
		ExecTimeout: execTimeout,
	}

	db = new(RedisDB)
	db.Config = cfg
	db.pool = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				"tcp",
				fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
				redis.DialPassword(cfg.Password),
				redis.DialDatabase(cfg.DB),
				redis.DialConnectTimeout(time.Second*2),
				redis.DialReadTimeout(time.Second*2),
				redis.DialWriteTimeout(time.Second*2),
			)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
		MaxIdle:     cfg.MaxIdle,   // 最大的空闲连接数，表示即使没有redis连接时依然可以保持N个空闲的连接，而不被清除，随时处于待命状态
		MaxActive:   cfg.MaxActive, // 最大的激活连接数，表示同时最多有N个连接 ，为0事表示没有限制
		IdleTimeout: time.Second,   //最大的空闲连接等待时间，超过此时间后，空闲连接将被关闭
		Wait:        true,          // 当链接数达到最大后是否阻塞，如果不的话，达到最大后返回错误
	}

	obj[conn] = db

	return
}

func errorsWrap(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}

// ConnPool 返回 redis.Pool.
// 除非必要一般不建议用这个函数, 用本库封装好的函数操作数据库.
func (db *RedisDB) ConnPool() *redis.Pool {
	return db.pool
}

// Close 释放连接资源.
func (db *RedisDB) Close() error {
	if db.pool != nil {
		return db.pool.Close()
	}
	return nil
}

// Do 执行 redis 命令
// NOTE 除非有必要(比如在一个函数内部需要执行多次 redis 操作), 否则请用该函数执行所有的操作, 这样能有效避免忘记释放资源.
func (db *RedisDB) Do(c *gin.Context, commandName string, args ...interface{}) (reply interface{}, err error) {
	if commandName == "PING" {
		return
	}
	var (
		ctx     = c.Request.Context()
		conn    = db.pool.Get()
		argsStr []string
	)

	defer conn.Close()

	header := &logging.LogHeader{
		LogId:     c.Writer.Header().Get(config.GetHeaderLogIdField(configs.LOG_SOURCE)),
		CallerIp:  c.ClientIP(),
		Port:      configs.SERVICE_PORT,
		Product:   configs.PRODUCT,
		Module:    "databus/redis",
		ServiceId: configs.SERVICE_NAME,
		UriPath:   c.Request.RequestURI,
		Env:       configs.ENV,
	}

	reply, err = conn.Do(commandName, args...)

	for _, arg := range args {
		argsStr = append(argsStr, fmt.Sprint(arg))
	}

	defer func() {
		ctx = logging.ContextWithLogHeader(ctx, header)

		if err != nil {
			logging.Errorf(header, "redis do:[%s], error: %s", fmt.Sprint(commandName, " ", strings.Join(argsStr, " ")), err)
		}
	}()

	return
}
