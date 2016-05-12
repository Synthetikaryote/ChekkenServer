package main

import (
    "io"
    "net/http"
	"log"
	"fmt"
    "golang.org/x/net/websocket"
)

// Echo the data received on the WebSocket.
func EchoServer(ws *websocket.Conn) {
    io.Copy(ws, ws)
    log.Println("Incoming connection")
}

type T struct {
	Msg string
	Count int
}

func webHandler(ws *websocket.Conn) {
	// good reference to continue:
	// https://github.com/golang-samples/websocket/blob/master/websocket-chat/src/chat/server.go
	var data []byte
	websocket.Message.Receive(ws, &data)
	fmt.Println("Received: ", data, " (", string(data[:]), ")")
}

// This example demonstrates a trivial echo server.
func main() {
	// this custom handler skips the origin check that non-browser clients
	// don't send.  otherwise, it would automatically reply with 403 forbidden
    http.HandleFunc("/", func (w http.ResponseWriter, req *http.Request) {
        s := websocket.Server{Handler: websocket.Handler(webHandler)}
        s.ServeHTTP(w, req)
    });
    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        panic("ListenAndServe: " + err.Error())
    }
}