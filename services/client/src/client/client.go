package client

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/logger"
	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/protocol"
)

const CONNECTION_ATTEMPTS_MAX = 3
const CONNECTION_ATTEMPS_DELAY_MS = 200

type ClientConfig struct {
	ServerHost string
	ServerPort string
	AgencyId   string
	InputFile  string
	OutputFile string
	BatchSize  int
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

	// parseo el AgencyId a uint16 para el protocolo
	agencyIDNum, err := strconv.ParseUint(client.config.AgencyId, 10, 16)
	if err != nil {
		return fmt.Errorf("error parseando AgencyId: %w", err)
	}

	// Instancio el protocolo sobre la conexion existente
	protocol := protocol.NewClientProtocol(client.conn)

	//abro archivo input
	inputFile, err := os.Open(client.config.InputFile)
	if err != nil { return err }
	defer inputFile.Close()

	//abro/creo archivo output
	outputFile, err := os.Create(client.config.OutputFile)
	if err != nil {return err}
	defer outputFile.Close()

	scanner := bufio.NewScanner(inputFile)

	batch := make([]string, 0, client.config.BatchSize)

	// Enviar todas las apuestas línea por línea
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		batch = append(batch, line)

		if len(batch) >= client.config.BatchSize {
            if err := protocol.SendBatch(uint16(agencyIDNum), batch); err != nil {
                return fmt.Errorf("error enviando batch: %w", err)
            }
            batch = batch[:0] // Limpiar buffer reutilizando memoria
        }
	}

	// Enviar remanente si quedaron apuestas que no completaron un batch entero
    if len(batch) > 0 {
        if err := protocol.SendBatch(uint16(agencyIDNum), batch); err != nil {
            return fmt.Errorf("error enviando último batch: %w", err)
        }
    }


	if err := protocol.SendCodeLastBatch(uint16(agencyIDNum)); err != nil {
			return fmt.Errorf("error al enviar aviso de ultimo batch: %w", err)
		}

	if err := scanner.Err(); err != nil {
			return fmt.Errorf("error leyendo input file: %w", err)
		}

	// Solicitar los ganadores
	if err := protocol.GetWinners(); err != nil {
		return fmt.Errorf("error solicitando ganadores: %w", err)
	}

	// Recibir la lista de ganadores
	winners, err := protocol.ReceiveWinners()
	if err != nil {
		return fmt.Errorf("error recibiendo ganadores: %w", err)
	}

	// Escribir cada ganador recibido en el OutputFile
	for _, winnerDoc := range winners {
		if _, err := outputFile.WriteString(winnerDoc + "\n"); err != nil {
			return fmt.Errorf("error escribiendo en output file: %w", err)
		}
	}

	// Notificar fin de comunicación
	_ = protocol.SendExit()
		
	return nil
}
