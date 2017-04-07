package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

// TODO: Pass a flag in to denote a client instead of a server
// Currently, we try to connect and start a server if we can't
func main() {
	if len(os.Args) == 2 {
		port := os.Args[1]
		client, err := net.Dial("tcp", ":"+port)
		if err != nil {
			fmt.Println("Failed to connect to server: " + err.Error())
			fmt.Println("Starting a new one...")
			chatServer(port)
		} else {
			fmt.Printf("Connection established: %v <-> %v\n", client.LocalAddr(), client.RemoteAddr())
			chatClient(client)
		}
	} else {
		fmt.Println("usage: go run chitter.go <port_number>")
	}
}

func chatClient(client net.Conn) {
	input := bufio.NewReader(os.Stdin)
	response := bufio.NewReader(client)
	defer client.Close()
	for {
		line, err := input.ReadBytes('\n')
		if err != nil {
			fmt.Println("Error while reading from stdin: ", err.Error())
			continue
		}
		client.Write(line)
		serverResp, err := response.ReadBytes('\n')
		if err != nil {
			fmt.Println("Client error while reading from stream: ", err.Error())
			break
		}
		fmt.Print(string(serverResp))
	}
}

func chatServer(port string) {
	server, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic("Failed to start listening: " + err.Error())
	}

	fmt.Println("Listening on port " + port)
	for {
		client, err := server.Accept()
		if err != nil {
			fmt.Println("Couldn't accept: " + err.Error())
			continue
		}
		fmt.Printf("Connection established: %v <-> %v\n", client.LocalAddr(), client.RemoteAddr())
		go handleClient(client)
	}
}

func handleClient(client net.Conn) {
	reader := bufio.NewReader(client)
	defer client.Close()
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			fmt.Println("Server error reading stream: " + err.Error())
			break
		}
		client.Write(line)
	}
}
