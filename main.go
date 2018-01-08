package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	"github.com/jasonsoft/napnap"
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
		log.Print(err)
		return
	}

	cli, err := client.NewClient("tcp://10.200.252.123:2376", "v1.30", nil, nil)
	if err != nil {
		log.Print(err)
		return
	}

	cmd := r.FormValue("cmd")
	regexMultiSpace := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	cmd = strings.TrimSpace(cmd)
	cmd = regexMultiSpace.ReplaceAllString(cmd, " ")
	cmdArray := strings.Split(cmd, " ")
	log.Print(cmdArray)
	ctx := context.Background()
	execConfig := types.ExecConfig{
		AttachStderr: true,
		AttachStdin:  true,
		AttachStdout: true,
		Cmd:          cmdArray,
		Tty:          true,
		Detach:       false,
	}

	exec, err := cli.ContainerExecCreate(ctx, "a2e914945c4c", execConfig)
	if err != nil {
		log.Print(err)
		return
	}

	execAttachConfig := types.ExecStartCheck{
		Detach: false,
		Tty:    true,
	}

	containerConn, err := cli.ContainerExecAttach(ctx, exec.ID, execAttachConfig)
	if err != nil {
		log.Print(err)
		return
	}

	go func() {
		for {
			buf := make([]byte, 4096)
			_, err = containerConn.Reader.Read(buf)
			if err != nil {
				log.Print(err)
				conn.Close()
				return
			}
			err = conn.WriteMessage(websocket.BinaryMessage, buf)
			if err != nil {
				log.Print(err)
				conn.Close()
				return
			}
		}
	}()

	for {
		_, reader, err := conn.NextReader()
		if err != nil {
			log.Print(err)
			containerConn.Close()
			return
		}
		_, err = io.Copy(containerConn.Conn, reader)
		if err != nil {
			log.Print(err)
			containerConn.Close()
			return
		}
	}
}

func main() {
	var listen = flag.String("listen", ":8000", "Host:port to listen on")
	nap := napnap.New()
	flag.Parse()
	router := napnap.NewRouter()
	router.Get("/term", napnap.WrapHandler(http.HandlerFunc(handleWebsocket)))
	nap.Use(router)
	httpengine := napnap.NewHttpEngine(*listen)
	log.Fatal(nap.Run(httpengine))
}
