package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type windowSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
	X    uint16
	Y    uint16
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleWebsocket(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Panic(err)
	}

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Panic(err)
	}

	ctx := context.Background()
	execConfig := types.ExecConfig{
		AttachStderr: true,
		AttachStdin:  true,
		AttachStdout: true,
		Cmd:          []string{"/bin/sh"},
		Tty:          true,
		Detach:       false,
	}

	exec, err := cli.ContainerExecCreate(ctx, "159c0c12f6bb", execConfig)
	if err != nil {
		log.Panic(err)
	}

	execAttachConfig := types.ExecStartCheck{
		Detach: false,
		Tty:    true,
	}

	containerConn, err := cli.ContainerExecAttach(ctx, exec.ID, execAttachConfig)
	if err != nil {
		log.Panic(err)
	}

	buf := make([]byte, 1024)
	_, err = containerConn.Conn.Read(buf)
	if err != nil {
		if err.Error() != "EOF" {
			log.Panic(err)
		}
	}
	err = conn.WriteMessage(websocket.BinaryMessage, buf)
	if err != nil {
		log.Panic(err)
	}

	go func() {
		for {
			buf := make([]byte, 4096)
			_, err = containerConn.Reader.Read(buf)
			if err != nil {
				log.Panic(err)
			}
			err = conn.WriteMessage(websocket.BinaryMessage, buf)
			if err != nil {
				log.Panic(err)
			}
		}
	}()

	for {
		_, reader, err := conn.NextReader()
		if err != nil {
			log.Panic(err)
		}
		_, err = io.Copy(containerConn.Conn, reader)
		if err != nil {
			log.Panic(err)
		}
	}
}

func main() {
	var listen = flag.String("listen", ":8000", "Host:port to listen on")

	flag.Parse()
	r := mux.NewRouter()

	r.HandleFunc("/term", handleWebsocket)
	log.Fatal(http.ListenAndServe(*listen, r))
}
