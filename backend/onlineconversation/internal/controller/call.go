package controller

import (
	"backend/common"
	"backend/onlineconversation/internal/dto"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

func (c *Controller) joinConversation(w http.ResponseWriter, r *http.Request) {
	slog.Info("try to make connection")
	memberIdRaw := r.Header.Get("X-User-Id")
	memberId, err := uuid.Parse(memberIdRaw)
	if err != nil {
		slog.Error("fail to parse member id from raw string",
			"err", err,
			"memberIdRaw", memberIdRaw)
		handleError(w, errors.New("fail to parse"))
		return
	}
	conversationId := r.URL.Query().Get("id")
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: false,
	})
	if err != nil {
		slog.Error("fail to accept connection", "err", err)
		handleError(w, errors.New("fail to accept ws connection"))
		return
	}
	c.csMutex.Lock()
	c.conns[memberId] = conn
	c.csMutex.Unlock()
	slog.Info("success to make connection",
		"number of current connection", len(c.conns))

	ip := c.podIP

	defer func() {
		destroy := context.Background()
		conn.Close(websocket.StatusNormalClosure, "")
		c.csMutex.Lock()
		delete(c.conns, memberId)
		c.csMutex.Unlock()
		c.service.RemoveServerIP(destroy, memberId)
		c.service.RemoveParticipant(destroy, conversationId, memberId)
		slog.Info("success to close connection",
			"number of current connection", len(c.conns))
	}()

	init := context.Background()

	err = c.service.SetServerIP(init, memberId, ip)
	if err != nil {
		handleWebsocketError(init, conn, err)
		return
	}
	pids, err := c.service.GetParticipantsWithoutMe(init, conversationId, memberId)
	if err != nil {
		handleWebsocketError(init, conn, err)
		return
	}
	if pids != nil {
		resp := dto.ConversationSignalResponse{FromIds: pids}
		p := common.Marshal(resp)
		err = conn.Write(init, websocket.MessageText, p)
		if err != nil {
			slog.Error("fail to write payload",
				"err", err,
			)
			return
		}
	}
	err = c.service.AddParticipant(init, conversationId, memberId)
	if err != nil {
		handleWebsocketError(init, conn, err)
		return
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var publishToIds [][]byte
	res := dto.ConversationSignalResponse{
		FromIds: []uuid.UUID{memberId},
	}
	resRaw := common.Marshal(res)
	for _, pid := range pids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.csMutex.RLock()
			p, ok := c.conns[pid]
			c.csMutex.RUnlock()
			var err1 error
			if ok {
				err1 = p.Write(init, websocket.MessageText, resRaw)
				if err1 != nil {
					slog.Error("fail to write payload",
						"err", err1,
					)
					err2 := p.CloseNow()
					if err2 != nil {
						slog.Error("fail to close zombie connection", "err", err2)
					}
				}
			}
			if !ok || err1 != nil {
				mu.Lock()
				publishToIds = append(publishToIds, pid[:])
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if publishToIds != nil {
		err = c.service.PublishConversationSignal(memberId, publishToIds, []byte{})
		if err != nil {
			slog.Error("fail to publish message", "err", err)
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
				err1 := conn.Ping(pingCtx)
				pingCancel()
				if err1 != nil {
					slog.Error("ping failed, client unresponsive", "err", err1)
					conn.Close(websocket.StatusInternalError, "ping timeout")
					return
				}
			}
		}
	}()

	for {
		ctx1 := context.Background()
		msgType, data, err1 := conn.Read(ctx1)
		if err1 != nil && err != io.EOF {
			if websocket.CloseStatus(err1) != -1 || errors.Is(err1, context.Canceled) {
				slog.Info("Connection closed smoothly", "err", err1)
				return
			}
			slog.Error("read error", "err", err1)
			handleWebsocketError(ctx, conn, errors.New("read error"))
			return
		}
		if msgType != websocket.MessageText {
			slog.Error("incorrect payload types",
				"msgType", msgType,
				"data", data)
			handleWebsocketError(ctx1, conn, errors.New("incorrect payload types"))
			return
		}
		var req dto.ConversationSignalRequest
		err1 = json.Unmarshal(data, &req)
		if err1 != nil {
			slog.Error("fail to unmarshalling data",
				"err", err1)
			handleWebsocketError(ctx1, conn, errors.New("incorrect data"))
			return
		}
		publishToIds = nil
		res = dto.ConversationSignalResponse{
			FromIds: []uuid.UUID{memberId},
			Signal:  req.Signal,
		}
		resRaw, _ = json.Marshal(res)
		for _, toId := range req.ToIds {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.csMutex.RLock()
				to, ok := c.conns[toId]
				c.csMutex.RUnlock()
				var err2 error
				if ok {
					err2 = to.Write(ctx1, websocket.MessageText, resRaw)
					if err2 != nil {
						slog.Error("fail to write payload",
							"err", err2,
						)
						err3 := to.CloseNow()
						if err3 != nil {
							slog.Error("fail to close zombie connection", "err", err3)
						}
					}
				}
				if !ok || err2 != nil {
					mu.Lock()
					publishToIds = append(publishToIds, toId[:])
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if publishToIds != nil {
			err1 = c.service.PublishConversationSignal(memberId, publishToIds, req.Signal)
			if err1 != nil {
				handleWebsocketError(ctx1, conn, errors.New("fail to publish"))
				slog.Error("fail to publish", "err", err1)
				return
			}
		}
	}
}

func (c *Controller) RelaySignal(ctx context.Context, toIds []uuid.UUID, fromId uuid.UUID, signal []byte) {
	var wg sync.WaitGroup
	res := dto.ConversationSignalResponse{
		FromIds: []uuid.UUID{fromId},
		Signal:  signal,
	}
	resRaw := common.Marshal(res)
	for _, toId := range toIds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.csMutex.RLock()
			wsc, ok := c.conns[toId]
			c.csMutex.RUnlock()
			var err error
			if ok {
				err = wsc.Write(ctx, websocket.MessageText, resRaw)
				if err != nil {
					slog.Error("fail to write payload", "err", err)
					err1 := wsc.CloseNow()
					if err1 != nil {
						slog.Error("fail to close zombie connection", "err", err)
					}
				}
				return
			}
		}()
	}
	wg.Wait()
}

// getPodIp will replace with k8s configmap pod ip
//func getPodIP() (string, error) {
//	addrs, err := net.InterfaceAddrs()
//	if err != nil {
//		return "", err
//	}
//	for _, addr := range addrs {
//		ipNet, ok := addr.(*net.IPNet)
//		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
//			return ipNet.IP.String(), nil
//		}
//	}
//	return "", errors.New("IP not found")
//}
