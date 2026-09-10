package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/safe_socket"
)

// Opcodes
const (
	// SendBet     byte = 0x01
	CodeSendBatch   byte = 0x01
	CodeGetWinners  byte = 0x02
	CodeSendWinners byte = 0x03
	CodeLastBatch   byte = 0x04
	CodeAck         byte = 0x05
	CodeExit        byte = 0x00
)

type ClientProtocol struct {
	rw io.ReadWriter
}

func NewClientProtocol(rw io.ReadWriter) *ClientProtocol {
	return &ClientProtocol{rw: rw}
}

func (p *ClientProtocol) SendBatch(agencyID uint16, lines []string) error {
    if len(lines) == 0 {
        return nil
    }

    // header: [Opcode (1B)][AgencyID (2B)][BatchCount (2B)]
    header := make([]byte, 5)
    header[0] = CodeSendBatch
    binary.BigEndian.PutUint16(header[1:3], agencyID)
    binary.BigEndian.PutUint16(header[3:5], uint16(len(lines)))

    if err := safe_socket.SendAll(p.rw, header); err != nil {
        return err
    }

    // Enviar cada registro con su longitud
    for _, line := range lines {
        payload := []byte(line)
        lenBuf := make([]byte, 2)
        binary.BigEndian.PutUint16(lenBuf, uint16(len(payload)))

        if err := safe_socket.SendAll(p.rw, lenBuf); err != nil {
            return err
        }
        if err := safe_socket.SendAll(p.rw, payload); err != nil {
            return err
        }
    }

    // Esperar confirmación (ACK) del servidor
    ackBuf, err := safe_socket.RecvAll(p.rw, 1)
    if err != nil {
        return fmt.Errorf("error esperando ACK: %w", err)
    }
    if ackBuf[0] != CodeAck {
        return errors.New("el servidor no confirmó el procesamiento del batch")
    }

    return nil
}

func (p *ClientProtocol) SendCodeLastBatch(agencyID uint16) error {
    buf := make([]byte, 3)
    buf[0] = CodeLastBatch
    binary.BigEndian.PutUint16(buf[1:3], agencyID)
    return safe_socket.SendAll(p.rw, buf)
}

// RequestWinners solicita el ganador del sorteo enviando el opcode GET_WINNERS
func (p *ClientProtocol) GetWinners() error {
	header := []byte{CodeGetWinners}
	return safe_socket.SendAll(p.rw, header)
}

// ReceiveWinners lee la respuesta del servidor con los DNI de los ganadores
func (p *ClientProtocol) ReceiveWinners() ([]string, error) {
	// Header: Opcode (1B) + Cantidad de ganadores (2B) = 3 bytes
	header, err := safe_socket.RecvAll(p.rw, 3)
	if err != nil {
		return nil, err
	}

	opcode := header[0]
	if opcode != CodeSendWinners {
		return nil, errors.New("opcode de respuesta inesperado")
	}

	winnerCount := binary.BigEndian.Uint16(header[1:3])
	winners := make([]string, 0, winnerCount)

	for i := 0; i < int(winnerCount); i++ {
		// Leer largo del DNI (uint16 -> 2 bytes)
		lenBuf, err := safe_socket.RecvAll(p.rw, 2)
		if err != nil {
			return nil, err
		}
		docLen := binary.BigEndian.Uint16(lenBuf)

		// Leer los bytes del DNI
		docBuf, err := safe_socket.RecvAll(p.rw, int(docLen))
		if err != nil {
			return nil, err
		}

		winners = append(winners, string(docBuf))
	}

	return winners, nil
}

// SendExit envía el opcode para avisar al servidor que finalizó la sesión
func (p *ClientProtocol) SendExit() error {
	return safe_socket.SendAll(p.rw, []byte{CodeExit})
}