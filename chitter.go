package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type Msg struct {
	id   int
	cmd  string
	line []byte
}

func main() {
	isClient := flag.Bool("c", false, "Connect as a client")
	flag.Parse()
	if len(flag.Args()) == 1 {
		port := flag.Args()[0]
		if *isClient {
			conn, err := net.Dial("tcp", ":"+port)
			if err != nil {
				println("Failed to connect to server: " + err.Error())
				return
			}
			fmt.Printf("Connection established: %v <-> %v\n", conn.LocalAddr(), conn.RemoteAddr())
			chatClient(conn)
		} else {
			chatServer(port)
		}
	} else {
		fmt.Println("usage: go run chitter.go [-c] <port_number>")
	}
}

func chatClient(server net.Conn) {
	stdinChan := make(chan []byte)
	serverChan := make(chan []byte)
	quit := make(chan bool)
	go readSelect(bufio.NewReader(os.Stdin), stdinChan, quit)
	go readSelect(bufio.NewReader(server), serverChan, quit)
	defer server.Close()
	for {
		select {
		case line := <-stdinChan:
			server.Write(line)
		case serverResp := <-serverChan:
			fmt.Print(string(serverResp))
		case <-quit:
			fmt.Println("Connection was closed...")
			return
		}
	}
}

func chatServer(port string) {
	server, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic("Failed to start listening: " + err.Error())
	}

	fmt.Println("Listening on port " + port)
	id, idChannelMap := 0, make(map[int]chan []byte)
	newConnChan, sendChan := make(chan net.Conn), make(chan Msg)
	go acceptConnections(server, newConnChan)
	for {
		select {
		case client := <-newConnChan:
			idChannelMap[id] = make(chan []byte)
			fmt.Printf("New client %d\n", id)
			go handleClient(id, client, idChannelMap[id], sendChan)
			id++
		case msg := <-sendChan:
			handleMessage(msg, idChannelMap)
		}
	}
}

func acceptConnections(server net.Listener, newConnChan chan net.Conn) {
	for {
		client, err := server.Accept()
		if err != nil {
			fmt.Println("Couldn't accept: " + err.Error())
			continue
		}
		fmt.Printf("Connection established: %v <-> %v\n", client.LocalAddr(), client.RemoteAddr())
		newConnChan <- client
	}
}

func handleMessage(msg Msg, idChannelMap map[int]chan []byte) {
	senderId := strconv.Itoa(msg.id)
	switch msg.cmd {
	case "all":
		for channelID, channel := range idChannelMap {
			if channelID != msg.id {
				channel <- append([]byte(senderId+": "), msg.line...)
			}
		}
	case "whoami":
		idChannelMap[msg.id] <- []byte("chitter: " + senderId + "\n")
	case "close":
		delete(idChannelMap, msg.id)
	default:
		if recipientId, err := strconv.Atoi(msg.cmd); err == nil && msg.id != recipientId {
			if channel, ok := idChannelMap[recipientId]; ok {
				channel <- append([]byte(senderId+": "), msg.line...)
				fmt.Printf("Sent private message from %d to %d\n", msg.id, recipientId)
			} else {
				idChannelMap[msg.id] <- []byte("chitter: No active user with ID " + msg.cmd + "\n")
			}
		} else {
			fmt.Printf("Unrecognized command \"%s\" from client %d\n", msg.cmd, msg.id)
		}
	}
}

func handleClient(id int, client net.Conn, recvChan chan []byte, sendChan chan Msg) {
	clientChan, quit := make(chan []byte), make(chan bool)
	go readSelect(bufio.NewReader(client), clientChan, quit)
	defer client.Close()
	for {
		select {
		case sendMsg := <-clientChan:
			if cmdIndex := strings.Index(string(sendMsg), ":"); cmdIndex == -1 {
				sendChan <- Msg{id, "all", []byte(strings.TrimLeft(string(sendMsg), " \t"))}
			} else {
				cmd := strings.TrimSpace(string(sendMsg)[:cmdIndex])
				line := []byte(strings.TrimLeft(string(sendMsg)[cmdIndex+1:], " \t"))
				sendChan <- Msg{id, cmd, line}
			}
		case rcvdMsg := <-recvChan:
			client.Write(rcvdMsg)
		case <-quit:
			fmt.Printf("Client %d closed\n", id)
			//Request client entry in idChannelMap be removed
			sendChan <- Msg{id, "close", nil}
			return
		}
	}
}

func readSelect(reader *bufio.Reader, ch chan []byte, quitChan chan bool) {
	for {
		switch line, err := reader.ReadBytes('\n'); err {
		case nil:
			ch <- line
		case io.EOF:
			quitChan <- true
			return
		default:
			fmt.Println("Error while reading: ", err.Error())
		}
	}
}
