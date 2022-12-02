# GoRedis

GoRedis 是一个用 Go 语言实现的 简易Redis 服务器。

关键功能:
- 支持 string, list, hash, set, sorted set, bitmap 数据结构
- 自动过期功能(TTL)
- AOF 持久化及 AOF 重写
- Multi 命令开启的事务具有`原子性`和`隔离性`. 若在执行过程中遇到错误, godis 会回滚已执行的命令


# 运行 GoRedis

在 GitHub 的 release 页下载 Darwin(MacOS) 和 Linux 版可执行文件。使用命令行启动 GoRedis 服务器

```bash
./godis-darwin
./godis-linux
```


GoRedis 默认监听 0.0.0.0:6399，可以使用 redis-cli 或者其它 redis 客户端连接 GoRedis 服务器。


GoRedis 首先会从CONFIG环境变量中读取配置文件路径。若环境变量中未设置配置文件路径，则会尝试读取工作目录中的 redis.conf 文件。 若 redis.conf 文件不存在则会使用自带的默认配置。



```bash
redis-cli -p 6399
```

## 支持的命令

请参考 [commands.md](https://github.com/Zerlina-ysl/goRedis/blob/main/commands.md)



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
    - sys.go: Auth 等系统功能实现
    - transaction.go: 单机事务实现
- aof: AOF 持久化实现 
