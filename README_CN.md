# Godis

Godis 是一个用 Go 语言实现的 Redis 服务器。本项目旨在为尝试使用 Go 语言开发高并发中间件的朋友提供一些参考。

关键功能:
- 支持 string, list, hash, set, sorted set, bitmap 数据结构
- 自动过期功能(TTL)
- 发布订阅
- AOF 持久化及 AOF 重写
- 加载和导出 RDB 文件
- Multi 命令开启的事务具有`原子性`和`隔离性`. 若在执行过程中遇到错误, godis 会回滚已执行的命令
- 并行引擎, 无需担心您的操作会阻塞整个服务器.


# 运行 Godis

在 GitHub 的 release 页下载 Darwin(MacOS) 和 Linux 版可执行文件。使用命令行启动 Godis 服务器

```bash
./godis-darwin
./godis-linux
```


godis 默认监听 0.0.0.0:6399，可以使用 redis-cli 或者其它 redis 客户端连接 Godis 服务器。


godis 首先会从CONFIG环境变量中读取配置文件路径。若环境变量中未设置配置文件路径，则会尝试读取工作目录中的 redis.conf 文件。 若 redis.conf 文件不存在则会使用自带的默认配置。



```bash
redis-cli -p 6399
```

## 支持的命令

请参考 [commands.md](https://github.com/HDT3213/godis/blob/master/commands.md)

## 性能测试

环境:

Go version：1.16

System: macOS Catalina 10.15.7

CPU: 2.6GHz 6-Core Intel Core i7

Memory: 16 GB 2667 MHz DDR4

redis-benchmark 测试结果:

```
PING_INLINE: 87260.03 requests per second
PING_BULK: 89206.06 requests per second
SET: 85034.02 requests per second
GET: 87565.68 requests per second
INCR: 91157.70 requests per second
LPUSH: 90334.23 requests per second
RPUSH: 90334.23 requests per second
LPOP: 90334.23 requests per second
RPOP: 90415.91 requests per second
SADD: 90909.09 requests per second
HSET: 84104.29 requests per second
SPOP: 82918.74 requests per second
LPUSH (needed to benchmark LRANGE): 78247.26 requests per second
LRANGE_100 (first 100 elements): 26406.13 requests per second
LRANGE_300 (first 300 elements): 11307.10 requests per second
LRANGE_500 (first 450 elements): 7968.13 requests per second
LRANGE_600 (first 600 elements): 6092.73 requests per second
MSET (10 keys): 65487.89 requests per second
```


## 如何阅读源码

本项目的目录结构:

- 根目录: main 函数，执行入口
- config: 配置文件解析
- interface: 一些模块间的接口定义
- lib: 各种工具，比如logger、同步和通配符

建议按照下列顺序阅读各包:

- tcp: tcp 服务器实现
- redis: redis 协议解析器
- datastruct: redis 的各类数据结构实现
    - dict: hash 表
    - list: 链表
    - lock: 用于锁定 key 的锁组件
    - set： 基于hash表的集合
    - sortedset: 基于跳表实现的有序集合
- database: 存储引擎核心
    - database.go: 支持多数据库的单机版 redis 服务实例
    - single_db.go: 单个 database 的数据结构和基本功能
    - router.go: 将命令路由给响应的处理函数
    - keys.go: del、ttl、expire 等通用命令实现
    - string.go: get、set 等字符串命令实现
    - list.go: lpush、lindex 等列表命令实现
    - hash.go: hget、hset 等哈希表命令实现
    - set.go: sadd 等集合命令实现
    - sortedset.go: zadd 等有序集合命令实现
    - pubsub.go: 发布订阅命令实现
    - geo.go: GEO 相关命令实现
    - sys.go: Auth 等系统功能实现
    - transaction.go: 单机事务实现
- aof: AOF 持久化实现 

具体请查看原项目地址：
https://github.com/HDT3213/godis