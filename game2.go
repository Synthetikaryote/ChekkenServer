package main

import (
    "io"
    "net/http"
    "log"
    "fmt"
    "golang.org/x/net/websocket"
    "encoding/binary"
    "bytes"
)

const (
    specIDAssign uint32 = 1
    specDisconnect uint32 = 2
)

type Client struct {
    id uint32
    ws *websocket.Conn
    ch chan []byte
}
var clients map[uint32]*Client = make(map[uint32]*Client)
var delCh = make(chan *Client)
var maxId uint32 = 0
var channelBufSize = 255

func (c *Client) listen() {
    for {
        select {
        case data := <-c.ch:
            err := websocket.Message.Send(c.ws, data)
            if err != nil {
                log.Println("Error:", err.Error())
                log.Println("Disconnecting client", c.id)
                c.ws.Close()
                delCh <- c;
                break
            }
        default:
            var data []byte
            err := websocket.Message.Receive(c.ws, &data)
            if err == io.EOF {
                // log.Println("Error:", err.Error())
            } else {
                fmt.Println("Received:", data, "(", string(data[:]), ")  Sending to all clients.")
                sendAll(data)
            }
        }
    }
}

func sendAll(data []byte) {
    for _, c := range clients {
        select {
            case c.ch <- data:
            default:
                delCh <- c
                err := fmt.Errorf("client %d is disconnected.", c.id)
                log.Println("Error:", err.Error())
        }
    }
}

func webHandler(ws *websocket.Conn) {
    // make a new client object
    c := &Client{maxId, ws, make(chan []byte, channelBufSize)}
    maxId++
    log.Println("Added new client with id", c.id)
    // store it
    clients[c.id] = c
    log.Println("Now", len(clients), "clients connected.")
    log.Println("Sending client their id of", c.id)
    // send it the id that was assigned
    buf := &bytes.Buffer{}
    binary.Write(buf, binary.LittleEndian, specIDAssign)
    binary.Write(buf, binary.LittleEndian, c.id)
    c.ch <- buf.Bytes()
    // start it listening
    c.listen()
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

    for {
        select {
        case c := <-delCh:
            delete(clients, c.id)
            buf := &bytes.Buffer{}
            binary.Write(buf, binary.LittleEndian, specDisconnect)
            binary.Write(buf, binary.LittleEndian, c.id)
            sendAll(buf.Bytes())
        }
    }
}