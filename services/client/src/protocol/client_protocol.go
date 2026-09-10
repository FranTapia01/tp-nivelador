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

    // Calcular el tamaño total exacto del payload para alocar una sola vez
    // Header base: Opcode (1B) + AgencyID (2B) + BatchCount (2B) = 5B
    totalSize := 5
    for _, line := range lines {
        totalSize += 2 + len(line) // 2 bytes de longitud + contenido de la línea
    }

    // Construir el buffer contiguo
    buf := make([]byte, totalSize)
    buf[0] = CodeSendBatch
    binary.BigEndian.PutUint16(buf[1:3], agencyID)
    binary.BigEndian.PutUint16(buf[3:5], uint16(len(lines)))

    offset := 5
    for _, line := range lines {
        lineBytes := []byte(line)
        lineLen := len(lineBytes)

        binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(lineLen))
        offset += 2

        copy(buf[offset:offset+lineLen], lineBytes)
        offset += lineLen
    }

    // Enviar todo el lote consolidado en una sola operacion de red
    if err := safe_socket.SendAll(p.rw, buf); err != nil {
        return err
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

// Avisa que ya se mandaron todas las bets
func (p *ClientProtocol) SendCodeLastBatch(agencyID uint16) error {
    buf := make([]byte, 3)
    buf[0] = CodeLastBatch
    binary.BigEndian.PutUint16(buf[1:3], agencyID)
    return safe_socket.SendAll(p.rw, buf)
}

// solicita el ganador del sorteo
func (p *ClientProtocol) GetWinners() error {
	header := []byte{CodeGetWinners}
	return safe_socket.SendAll(p.rw, header)
}

// Lee la respuesta del servidor con los ganadores
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
		// Leer largo (uint16 -> 2 bytes)
		lenBuf, err := safe_socket.RecvAll(p.rw, 2)
		if err != nil {
			return nil, err
		}
		lineLen := binary.BigEndian.Uint16(lenBuf)

		// Leer los bytes de la linea
		lineBuf, err := safe_socket.RecvAll(p.rw, int(lineLen))
		if err != nil {
			return nil, err
		}

		winners = append(winners, string(lineBuf))
	}

	return winners, nil
}

// Env0a el opcode para avisar al servidor que finalizó la sesión
func (p *ClientProtocol) SendExit() error {
	return safe_socket.SendAll(p.rw, []byte{CodeExit})
}