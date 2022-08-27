package database

import (
	"fmt"
	"personalCode/goRedis/aof"
	"personalCode/goRedis/config"
	"personalCode/goRedis/interface/database"
	"personalCode/goRedis/interface/redis"
	"personalCode/goRedis/lib/logger"
	"personalCode/goRedis/lib/utils"
	"personalCode/goRedis/redis/protocol"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// MultiDB is a set of multiple database set
type MultiDB struct {
	dbSet []*atomic.Value // *DB

	// handle aof persistence
	aofHandler *aof.Handler
}

func (mdb *MultiDB) AfterClientClose(c redis.Connection) {
	//TODO implement me
	logger.Info("After Client Close")
}

// NewStandaloneServer creates a standalone redis server, with multi database and all other funtions
func NewStandaloneServer() *MultiDB {
	mdb := &MultiDB{}
	if config.Properties.Databases == 0 {
		config.Properties.Databases = 16
	}
	mdb.dbSet = make([]*atomic.Value, config.Properties.Databases)
	for i := range mdb.dbSet {
		singleDB := makeDB()
		singleDB.index = i
		holder := &atomic.Value{}
		holder.Store(singleDB)
		mdb.dbSet[i] = holder
	}
	if config.Properties.AppendOnly {
		aofHandler, err := aof.NewAOFHandler(mdb, func() database.EmbedDB {
			return MakeBasicMultiDB()
		})
		if err != nil {
			panic(err)
		}
		mdb.aofHandler = aofHandler
		for _, db := range mdb.dbSet {
			singleDB := db.Load().(*DB)
			singleDB.addAof = func(line CmdLine) {
				mdb.aofHandler.AddAof(singleDB.index, line)
			}
		}
	}

	return mdb
}

// MakeBasicMultiDB create a MultiDB only with basic abilities for aof rewrite and other usages
func MakeBasicMultiDB() *MultiDB {
	mdb := &MultiDB{}
	mdb.dbSet = make([]*atomic.Value, config.Properties.Databases)
	for i := range mdb.dbSet {
		holder := &atomic.Value{}
		holder.Store(makeBasicDB())
		mdb.dbSet[i] = holder
	}
	return mdb
}

// Exec executes command
// parameter `cmdLine` contains command and its arguments, for example: "set key value"
func (mdb *MultiDB) Exec(c redis.Connection, cmdLine [][]byte) (result redis.Reply) {
	defer func() {
		if err := recover(); err != nil {
			logger.Warn(fmt.Sprintf("error occurs: %v\n%s", err, string(debug.Stack())))
			result = &protocol.UnknownErrReply{}
		}
	}()

	cmdName := strings.ToLower(string(cmdLine[0]))
	// authenticate
	if cmdName == "auth" {
		return Auth(c, cmdLine[1:])
	}
	if !isAuthenticated(c) {
		return protocol.MakeErrReply("NOAUTH Authentication required")
	}

	// special commands which cannot execute within transaction
	if cmdName == "save" {
		return SaveRDB(mdb, cmdLine[1:])
	} else if cmdName == "bgsave" {
		return BGSaveRDB(mdb, cmdLine[1:])
	} else if cmdName == "select" {
		if c != nil && c.InMultiState() {
			return protocol.MakeErrReply("cannot select database within multi")
		}
		if len(cmdLine) != 2 {
			return protocol.MakeArgNumErrReply("select")
		}
		return execSelect(c, mdb, cmdLine[1:])
	} else if cmdName == "copy" {
		if len(cmdLine) < 3 {
			return protocol.MakeArgNumErrReply("copy")
		}
		return execCopy(mdb, c, cmdLine[1:])
	}
	// todo: support multi database transaction

	// normal commands
	dbIndex := c.GetDBIndex()
	selectedDB, errReply := mdb.selectDB(dbIndex)
	if errReply != nil {
		return errReply
	}
	return selectedDB.Exec(c, cmdLine)
}

// Close graceful shutdown database
func (mdb *MultiDB) Close() {
	if mdb.aofHandler != nil {
		mdb.aofHandler.Close()
	}
}

func execSelect(c redis.Connection, mdb *MultiDB, args [][]byte) redis.Reply {
	dbIndex, err := strconv.Atoi(string(args[0]))
	if err != nil {
		return protocol.MakeErrReply("ERR invalid DB index")
	}
	if dbIndex >= len(mdb.dbSet) || dbIndex < 0 {
		return protocol.MakeErrReply("ERR DB index is out of range")
	}
	c.SelectDB(dbIndex)
	return protocol.MakeOkReply()
}

func (mdb *MultiDB) flushDB(dbIndex int) redis.Reply {
	if dbIndex >= len(mdb.dbSet) || dbIndex < 0 {
		return protocol.MakeErrReply("ERR DB index is out of range")
	}
	newDB := makeDB()
	mdb.loadDB(dbIndex, newDB)
	return &protocol.OkReply{}
}

func (mdb *MultiDB) loadDB(dbIndex int, newDB *DB) redis.Reply {
	if dbIndex >= len(mdb.dbSet) || dbIndex < 0 {
		return protocol.MakeErrReply("ERR DB index is out of range")
	}
	oldDB := mdb.mustSelectDB(dbIndex)
	newDB.index = dbIndex
	newDB.addAof = oldDB.addAof // inherit oldDB
	mdb.dbSet[dbIndex].Store(newDB)
	return &protocol.OkReply{}
}

func (mdb *MultiDB) flushAll() redis.Reply {
	for i := range mdb.dbSet {
		mdb.flushDB(i)
	}
	if mdb.aofHandler != nil {
		mdb.aofHandler.AddAof(0, utils.ToCmdLine("FlushAll"))
	}
	return &protocol.OkReply{}
}

func (mdb *MultiDB) selectDB(dbIndex int) (*DB, *protocol.StandardErrReply) {
	if dbIndex >= len(mdb.dbSet) || dbIndex < 0 {
		return nil, protocol.MakeErrReply("ERR DB index is out of range")
	}
	return mdb.dbSet[dbIndex].Load().(*DB), nil
}

func (mdb *MultiDB) mustSelectDB(dbIndex int) *DB {
	selectedDB, err := mdb.selectDB(dbIndex)
	if err != nil {
		panic(err)
	}
	return selectedDB
}

// ForEach traverses all the keys in the given database
func (mdb *MultiDB) ForEach(dbIndex int, cb func(key string, data *database.DataEntity, expiration *time.Time) bool) {
	mdb.mustSelectDB(dbIndex).ForEach(cb)
}

// ExecMulti executes multi commands transaction Atomically and Isolated
func (mdb *MultiDB) ExecMulti(conn redis.Connection, watching map[string]uint32, cmdLines []CmdLine) redis.Reply {
	selectedDB, errReply := mdb.selectDB(conn.GetDBIndex())
	if errReply != nil {
		return errReply
	}
	return selectedDB.ExecMulti(conn, watching, cmdLines)
}

// RWLocks lock keys for writing and reading
func (mdb *MultiDB) RWLocks(dbIndex int, writeKeys []string, readKeys []string) {
	mdb.mustSelectDB(dbIndex).RWLocks(writeKeys, readKeys)
}

// RWUnLocks unlock keys for writing and reading
func (mdb *MultiDB) RWUnLocks(dbIndex int, writeKeys []string, readKeys []string) {
	mdb.mustSelectDB(dbIndex).RWUnLocks(writeKeys, readKeys)
}

// GetUndoLogs return rollback commands
func (mdb *MultiDB) GetUndoLogs(dbIndex int, cmdLine [][]byte) []CmdLine {
	return mdb.mustSelectDB(dbIndex).GetUndoLogs(cmdLine)
}

// ExecWithLock executes normal commands, invoker should provide locks
func (mdb *MultiDB) ExecWithLock(conn redis.Connection, cmdLine [][]byte) redis.Reply {
	db, errReply := mdb.selectDB(conn.GetDBIndex())
	if errReply != nil {
		return errReply
	}
	return db.execWithLock(cmdLine)
}

// BGRewriteAOF asynchronously rewrites Append-Only-File
func BGRewriteAOF(db *MultiDB, args [][]byte) redis.Reply {
	go db.aofHandler.Rewrite()
	return protocol.MakeStatusReply("Background append only file rewriting started")
}

// RewriteAOF start Append-Only-File rewriting and blocked until it finished
func RewriteAOF(db *MultiDB, args [][]byte) redis.Reply {
	err := db.aofHandler.Rewrite()
	if err != nil {
		return protocol.MakeErrReply(err.Error())
	}
	return protocol.MakeOkReply()
}

// SaveRDB start RDB writing and blocked until it finished
func SaveRDB(db *MultiDB, args [][]byte) redis.Reply {
	if db.aofHandler == nil {
		return protocol.MakeErrReply("please enable aof before using save")
	}
	err := db.aofHandler.Rewrite2RDB()
	if err != nil {
		return protocol.MakeErrReply(err.Error())
	}
	return protocol.MakeOkReply()
}

// BGSaveRDB asynchronously save RDB
func BGSaveRDB(db *MultiDB, args [][]byte) redis.Reply {
	if db.aofHandler == nil {
		return protocol.MakeErrReply("please enable aof before using save")
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Error(err)
			}
		}()
		err := db.aofHandler.Rewrite2RDB()
		if err != nil {
			logger.Error(err)
		}
	}()
	return protocol.MakeStatusReply("Background saving started")
}

// GetDBSize returns keys count and ttl key count
func (mdb *MultiDB) GetDBSize(dbIndex int) (int, int) {
	db := mdb.mustSelectDB(dbIndex)
	return db.data.Len(), db.ttlMap.Len()
}
