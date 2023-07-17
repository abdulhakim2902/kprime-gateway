package redis

import (
	"fmt"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/log"
	"github.com/gomodule/redigo/redis"
)

var logger = log.Logger

type RedisConnection struct {
	redis.Conn
}

type RedisConnectionPool struct {
	*redis.Pool
}

func NewRedisConnectionPool(uri string) *RedisConnectionPool {
	pool := &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", uri)
			if err != nil {
				return nil, err
			}
			return c, err
		},
	}

	return &RedisConnectionPool{pool}
}

// Set sets a key to a given value
func (p *RedisConnectionPool) Set(key string, value string) error {
	conn := p.Get()
	defer conn.Close()

	ok, err := redis.String(conn.Do("SET", key, value))
	if err != nil {
		return err
	} else if ok != "OK" {
		return fmt.Errorf("Some error occurred while running SET command on key: %v", key)
	}
	return nil
}

// Set sets a key to a given value and expire
func (p *RedisConnectionPool) SetEx(key string, value string, expiry int) error {
	conn := p.Get()
	defer conn.Close()

	ok, err := redis.String(conn.Do("SET", key, value, "EX", expiry))
	if err != nil {
		return err
	} else if ok != "OK" {
		return fmt.Errorf("Some error occurred while running SET command on key: %v", key)
	}
	return nil
}

// Del removes a given key from Redis
// Cmd Returns: number of deletions and error
// Returns: error
func (p *RedisConnectionPool) Del(key string) error {
	conn := p.Get()
	defer conn.Close()

	_, err := redis.Int64(conn.Do("DEL", key))
	return err
}

func (p *RedisConnectionPool) GetValue(key string) (string, error) {
	conn := p.Get()
	defer conn.Close()

	value, err := redis.String(conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return "", nil
		}
		return "", err
	}

	return value, nil
}
