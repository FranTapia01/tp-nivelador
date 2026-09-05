package client

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/logger"
	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/safe_socket"
)

const CONNECTION_ATTEMPTS_MAX = 3
const CONNECTION_ATTEMPS_DELAY_MS = 200

const ECHO_CLIENT_BUFFER_SIZE = 512
const ECHO_CLIENT_MESSAGE_AMOUNT = 3
const ECHO_CLIENT_MESSAGE_DELAY_MS = 1000

type ClientConfig struct {
	ServerHost string
	ServerPort string
	AgencyId   string
	InputFile  string
	OutputFile string
}

type Client struct {
	conn   net.Conn
	config ClientConfig
}

func NewClient(config ClientConfig) (*Client, error) {
	conn, err := connectToServer(config.ServerHost, config.ServerPort)
	if err != nil {
		logger.Warn("connect-to-server", logger.Fail)
		return nil, err
	}

	client := &Client{conn: conn, config: config}
	return client, nil
}

func connectToServer(host, port string) (net.Conn, error) {
	const action = "connect-to-server"
	var err error
	var conn net.Conn

	logger.Info(action, logger.InProgress)
	for i := range CONNECTION_ATTEMPTS_MAX {
		conn, err = net.Dial("tcp", host+":"+port)
		if err != nil {
			logger.Warn(action, logger.Fail, "attempt", i)
			time.Sleep(CONNECTION_ATTEMPS_DELAY_MS * time.Millisecond)
			continue
		}

		logger.Info(action, logger.Success)
		break
	}

	return conn, err
}

func (client *Client) Run() error {
	defer client.conn.Close()

	//abro archivo input
	inputFile, err := os.Open(client.config.InputFile)
	if err != nil { return err }
	defer inputFile.Close()

	//abro/creo archivo output
	outputFile, err := os.Create(client.config.OutputFile)
	if err != nil {return err}
	defer outputFile.Close()

	scanner := bufio.NewScanner(inputFile)

	//proceso linea por linea
	for scanner.Scan() {
		line := scanner.Text()
        if len(line) == 0 {
            continue
        }

		//envio al server
		if err := safe_socket.SendAll(client.conn, []byte(line)); err != nil {
            return fmt.Errorf("error al enviar mensaje: %w", err)
        }

		//recibo respuesta
		responseBuffer, err := safe_socket.RecvAll(client.conn, len(line))
        if err != nil {
            return fmt.Errorf("error al recibir respuesta: %w", err)
        }

		//escribo en output la respuesta
		if _, err := outputFile.WriteString(string(responseBuffer) + "\n"); err != nil {
            return fmt.Errorf("error al escribir en output file: %w", err)
        }
	}

	if err := scanner.Err(); err != nil {
			return fmt.Errorf("error leyendo input file: %w", err)
		}
		
	return nil
}

// func (client *Client) Run() error {
// 	const mainAction = "test-echo-server"
// 	defer client.conn.Close()

// 	for messageId := range ECHO_CLIENT_MESSAGE_AMOUNT {
// 		messageArgs := []any{"agency-id", client.config.AgencyId, "message-id", messageId}
// 		logger.Info(mainAction, logger.InProgress, messageArgs...)

// 		clientMessage := client.config.AgencyId

// 		if err := safe_socket.SendAll(client.conn, []byte(clientMessage)); err != nil {
// 			logger.Error("send-message", logger.Fail, messageArgs...)
// 			return err
// 		}

// 		responseBuffer, err := safe_socket.RecvAll(client.conn, ECHO_CLIENT_BUFFER_SIZE)
// 		if err != nil {
// 			logger.Error("recv-response", logger.Fail, messageArgs...)
// 			return err
// 		}

// 		if string(responseBuffer) != clientMessage {
// 			logger.Error("check-response", logger.Fail, messageArgs...)
// 			return err
// 		}

// 		time.Sleep(ECHO_CLIENT_MESSAGE_DELAY_MS * time.Millisecond)
// 	}
// 	logger.Info(mainAction, logger.Success, "agency-id", client.config.AgencyId)

// 	return nil
// }
