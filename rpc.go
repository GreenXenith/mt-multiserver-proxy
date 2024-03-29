package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/sourcegraph/jsonrpc2"
)

type RPCResult struct {
	Success bool
	Message string
	Data any
}

// RPC Methods
// TODO: Finish copying functions from https://github.com/HimbeerserverDE/mt-multiserver-chatcommands/blob/main/chatcommands.go
// TODO: Pass params/reply/error instead
// TODO: Check for params and valid JSON outside of method
func RPCShutdown(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	proc, err := os.FindProcess(os.Getpid())

	if err != nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInternalError, Message: "Could not find process: " + err.Error()}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	proc.Signal(os.Interrupt)
}

func RPCFind(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	params := struct {
		Name *string `json:"name"`
	}{}

	type CltInfo = struct {
		Server string
		Address string
	}

	if req.Params == nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidRequest, Message: "Missing 'params' field"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	// TODO: Use req.Params.UnmarshalJSON() instead
	err := json.Unmarshal(*req.Params, &params)
	if err != nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeParseError, Message: "Invalid JSON: " + err.Error()}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	if params.Name == nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Missing 'name' parameter"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	clt := Find(*params.Name)
	if clt != nil {
		if err := conn.Reply(ctx, req.ID, RPCResult {
			Success: true,
			Message: "",
			Data: CltInfo {
				Server: clt.ServerName(),
				Address: clt.RemoteAddr().String(),
			},
		}); err != nil {
			log.Println(err)
		}
	} else {
		if err := conn.Reply(ctx, req.ID, RPCResult{Success: true}); err != nil {
			log.Println(err)
		}
	}
}

func RPCAddr(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCAlert(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	params := struct {
		Msg *string `json:"msg"`
	}{}

	if req.Params == nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidRequest, Message: "Missing 'params' field"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	err := json.Unmarshal(*req.Params, &params)
	if err != nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeParseError, Message: "Invalid JSON: " + err.Error()}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	if params.Msg == nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Missing 'msg' parameter"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	for clt := range Clts() {
		clt.SendChatMsg(*params.Msg)
	}

	conn.Reply(ctx, req.ID, RPCResult {
		Success: true,
	})
}

func RPCSend(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	params := struct {
		Mode *string `json:"mode"`
		Target *string `json:"target"`
		Destination *string `json:"destination"`
	}{}

	if req.Params == nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidRequest, Message: "Missing 'params' field"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	err := json.Unmarshal(*req.Params, &params)
	if err != nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeParseError, Message: "Invalid JSON: " + err.Error()}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	// These repetetive checks will be replaced
	if params.Mode == nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Missing 'mode' parameter"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	if params.Target == nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Missing 'target' parameter"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	if params.Destination == nil {
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "Missing 'destination' parameter"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
		}

		return
	}

	switch *params.Mode {
	case "player":
		clt := Find(*params.Target)
		if clt == nil {
			conn.Reply(ctx, req.ID, RPCResult {
				Success: false,
				Message: "player not connected",
			})

			return
		}

		if *params.Destination == clt.ServerName() {
			conn.Reply(ctx, req.ID, RPCResult {
				Success: false,
				Message: "player already connected",
			})

			return
		}

		if err := clt.Hop(*params.Destination); err != nil {
			clt.Log("<-", err)

			if errors.Is(err, ErrNoSuchServer) {
				conn.Reply(ctx, req.ID, RPCResult {
					Success: false,
					Message: "server does not exist",
				})

				return
			} else if errors.Is(err, ErrNewMediaPool) {
				conn.Reply(ctx, req.ID, RPCResult {
					Success: false,
					Message: "server belongs to media pool not present on client",
				})

				return
			}

			conn.Reply(ctx, req.ID, RPCResult {
				Success: false,
				Message: "could not switch servers",
			})

			return
		}
	case "current":
		if *params.Target == *params.Destination {
			conn.Reply(ctx, req.ID, RPCResult {
				Success: false,
				Message: "target and destination identical",
			})

			return
		}

		for clt := range Clts() {
			if clt.ServerName() == *params.Target && clt.ServerName() != *params.Destination {
				if err := clt.Hop(*params.Destination); err != nil {
					clt.Log("<-", err)

					if errors.Is(err, ErrNoSuchServer) {
						conn.Reply(ctx, req.ID, RPCResult {
							Success: false,
							Message: "server does not exist",
						})

						return
					}

					conn.Reply(ctx, req.ID, RPCResult {
						Success: false,
						Message: "could not switch servers (" + err.Error() + ")",
					})

					return
				}
			}
		}
	case "all":
		for clt := range Clts() {
			if clt.ServerName() != *params.Destination {
				if err := clt.Hop(*params.Destination); err != nil {
					clt.Log("<-", err)

					if errors.Is(err, ErrNoSuchServer) {
						conn.Reply(ctx, req.ID, RPCResult {
							Success: false,
							Message: "server does not exist",
						})

						return
					}

					conn.Reply(ctx, req.ID, RPCResult {
						Success: false,
						Message: "could not switch servers (" + err.Error() + ")",
					})

					return
				}
			}
		}
	default:
		conn.Reply(ctx, req.ID, RPCResult {
			Success: false,
			Message: "unknown mode '" + *params.Mode + "'",
		})

		return
	}
}

func RPCGSend(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCPlayers(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCReload(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCGroup(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCPerms(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCGPerms(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCServer(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCGServer(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCKick(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCBan(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCUnban(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCUptime(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCHelp(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func RPCUsage(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

// Method Handler
type RPCHandler struct{}

func (h *RPCHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
	case "shutdown": RPCShutdown(ctx, conn, req)
	case "find": RPCFind(ctx, conn, req)
	case "addr": RPCAddr(ctx, conn, req)
	case "alert": RPCAlert(ctx, conn, req)
	case "send": RPCSend(ctx, conn, req)
	case "gsend": RPCGSend(ctx, conn, req)
	case "players": RPCPlayers(ctx, conn, req)
	case "reload": RPCReload(ctx, conn, req)
	case "group": RPCGroup(ctx, conn, req)
	case "perms": RPCPerms(ctx, conn, req)
	case "gperms": RPCGPerms(ctx, conn, req)
	case "server": RPCServer(ctx, conn, req)
	case "gserver": RPCGServer(ctx, conn, req)
	case "kick": RPCKick(ctx, conn, req)
	case "ban": RPCBan(ctx, conn, req)
	case "unban": RPCUnban(ctx, conn, req)
	case "uptime": RPCUptime(ctx, conn, req)
	case "help": RPCHelp(ctx, conn, req)
	case "usage": RPCUsage(ctx, conn, req)
	default:
		err := &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: "Method not found"}
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Println(err)
			return
		}
	}
}

// ReadWriteCloser implementation (used by jsonrpc2)
type RPCReadWriteCloser struct {
	r    io.Reader
	rw   io.ReadWriter
	done chan bool
}

func (rwc *RPCReadWriteCloser) Read(p []byte) (int, error) {
	return rwc.r.Read(p)
}

func (rwc *RPCReadWriteCloser) Write(p []byte) (int, error) {
	return rwc.rw.Write(p)
}

func (rwc *RPCReadWriteCloser) Close() error {
	rwc.done <- true

	return nil
}

func (rwc *RPCReadWriteCloser) Call() io.Reader {
	go jsonrpc2.NewConn(context.Background(), jsonrpc2.NewPlainObjectStream(rwc), &RPCHandler{})
	<- rwc.done

	return rwc.rw
}

func RPCRequest(r io.Reader) *RPCReadWriteCloser {
	return &RPCReadWriteCloser{r, &bytes.Buffer{}, make(chan bool)}
}

// HTTP Endpoint
func init() {
	http.HandleFunc("/api", func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		io.Copy(w, RPCRequest(req.Body).Call())
	})
}
