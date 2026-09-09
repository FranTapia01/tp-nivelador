package protocol

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/safe_socket"
)

// Opcodes
const (
	SendBet     byte = 0x01
	GetWinners  byte = 0x02
	SendWinners byte = 0x03
	Exit        byte = 0x00
)

type ClientProtocol struct {
	rw io.ReadWriter
}

func NewClientProtocol(rw io.ReadWriter) *ClientProtocol {
	return &ClientProtocol{rw: rw}
}

// SendBet envía: [Opcode (1B)][AgencyID (2B)][Len (4B)][Payload]
func (p *ClientProtocol) SendBet(agencyID uint16, csvLine string) error {
	payload := []byte(csvLine)
	header := make([]byte, 1+2+4) // 7 bytes totales

	header[0] = SendBet
	binary.BigEndian.PutUint16(header[1:3], agencyID)
	binary.BigEndian.PutUint32(header[3:7], uint32(len(payload)))

	// mandamos el header y despues el payload
	if err := safe_socket.SendAll(p.rw, header); err != nil {
		return err
	}
	return safe_socket.SendAll(p.rw, payload)
}

// RequestWinners solicita el ganador del sorteo enviando el opcode GET_WINNERS
func (p *ClientProtocol) GetWinners() error {
	header := []byte{GetWinners}
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
	if opcode != SendWinners {
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
	return safe_socket.SendAll(p.rw, []byte{Exit})
}