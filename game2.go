package main

import (
    "io"
    "net/http"
    "log"
    "fmt"
    "golang.org/x/net/websocket"
    "encoding/binary"
    "bytes"
    "math"
)

const (
    specInitialize uint32 = 1
    specDisconnect uint32 = 2
    specAnnounceConnect uint32 = 3
    specUpdatePosition uint32 = 4

    nameLength uint = 16
)

type Client struct {
    id uint32
    ws *websocket.Conn
    ch chan []byte
    x, y, z float32
    name string
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
                spec := binary.LittleEndian.Uint32(data[0:4])
                // bytes 4-7 is the id.  skip that
                switch spec {
                case specAnnounceConnect:
                    c.name = string(data[8:8+nameLength])
                    c.x = Float32FromBytes(data[8+nameLength:12+nameLength])
                    c.y = Float32FromBytes(data[12+nameLength:16+nameLength])
                    c.z = Float32FromBytes(data[16+nameLength:20+nameLength])
                    fmt.Println(c.name, "entered the game at (", c.x, c.y, c.z, ")")
                case specUpdatePosition:
                    c.x = Float32FromBytes(data[8:12])
                    c.y = Float32FromBytes(data[12:16])
                    c.z = Float32FromBytes(data[16:20])
                    fmt.Println(c.name, "moved to (", c.x, c.y, c.z, ")")
                }
                sendAll(data)
            }
        }
    }
}

func Float32FromBytes(bytes []byte) float32 {
    bits := binary.LittleEndian.Uint32(bytes)
    f := math.Float32frombits(bits)
    return f
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
    c := &Client{maxId, ws, make(chan []byte, channelBufSize), 0, 0, 0, ""}
    maxId++
    log.Println("Added new client with id", c.id)
    // store it
    clients[c.id] = c
    log.Println("Now", len(clients), "clients connected.")
    log.Println("Sending client their id of", c.id)
    // send it the id that was assigned
    buf := &bytes.Buffer{}
    binary.Write(buf, binary.LittleEndian, specInitialize)
    binary.Write(buf, binary.LittleEndian, c.id)
    // send it the number of other players
    var otherPlayerCount uint32 = 0
    for _, player := range clients {
        if player.id == c.id {
            continue
        }
        if player.name == "" {
            continue
        }
        otherPlayerCount++
    }
    binary.Write(buf, binary.LittleEndian, otherPlayerCount)
    // send it the information for each other player
    for _, player := range clients {
        if player.id == c.id {
            continue
        }
        if player.name == "" {
            continue
        }
        binary.Write(buf, binary.LittleEndian, player.id)
        nameBytes := make([]byte, nameLength, nameLength)
        copy(nameBytes, []byte(player.name))
        binary.Write(buf, binary.LittleEndian, nameBytes)
        binary.Write(buf, binary.LittleEndian, player.x)
        binary.Write(buf, binary.LittleEndian, player.y)
        binary.Write(buf, binary.LittleEndian, player.z)
    }
    data := buf.Bytes()
    fmt.Println("Sending the new player", data, "(", string(data[:]), ")")
    c.ch <- data
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