package server

/*
 * A tcp.Handler implements redis protocol
 */

import (
	"context"
	"io"
	"net"
	_ "personalCode/goRedis/config"
	database2 "personalCode/goRedis/database"
	"personalCode/goRedis/interface/database"
	"personalCode/goRedis/lib/logger"
	"personalCode/goRedis/lib/sync/atomic"
	"personalCode/goRedis/redis/connection"
	"personalCode/goRedis/redis/parser"
	"personalCode/goRedis/redis/protocol"
	"strings"
	"sync"
)

var (
	unknownErrReplyBytes = []byte("-ERR unknown\r\n")
)

// Handler implements tcp.Handler and serves as a redis server
type Handler struct {
	activeConn sync.Map // *client -> placeholder
	db         database.DB
	closing    atomic.Boolean // refusing new client and new request
}

// MakeHandler creates a Handler instance
func MakeHandler() *Handler {
	var db database.DB
	db = database2.NewStandaloneServer()

	return &Handler{
		db: db,
	}
}

func (h *Handler) closeClient(client *connection.Connection) {
	_ = client.Close()
	h.db.AfterClientClose(client)
	h.activeConn.Delete(client)
}

// Handle receives and executes redis commands
func (h *Handler) Handle(ctx context.Context, conn net.Conn) {
	if h.closing.Get() {
		// closing handler refuse new connection
		_ = conn.Close()
		return
	}

	client := connection.NewConn(conn)
	h.activeConn.Store(client, 1)

	ch := parser.ParseStream(conn)
	for payload := range ch {
		if payload.Err != nil {
			if payload.Err == io.EOF ||
				payload.Err == io.ErrUnexpectedEOF ||
				strings.Contains(payload.Err.Error(), "use of closed network connection") {
				// connection closed
				h.closeClient(client)
				logger.Info("connection closed: " + client.RemoteAddr().String())
				return
			}
			// protocol err
			errReply := protocol.MakeErrReply(payload.Err.Error())
			err := client.Write(errReply.ToBytes())
			if err != nil {
				h.closeClient(client)
				logger.Info("connection closed: " + client.RemoteAddr().String())
				return
			}
			continue
		}
		if payload.Data == nil {
			logger.Error("empty payload")
			continue
		}
		r, ok := payload.Data.(*protocol.MultiBulkReply)
		if !ok {
			logger.Error("require multi bulk protocol")
			continue
		}
		result := h.db.Exec(client, r.Args)
		if result != nil {
			_ = client.Write(result.ToBytes())
		} else {
			_ = client.Write(unknownErrReplyBytes)
		}
	}
}

// Close stops handler
func (h *Handler) Close() error {
	logger.Info("handler shutting down...")
	h.closing.Set(true)
	// TODO: concurrent wait
	h.activeConn.Range(func(key interface{}, val interface{}) bool {
		client := key.(*connection.Connection)
		_ = client.Close()
		return true
	})
	h.db.Close()
	return nil
}
