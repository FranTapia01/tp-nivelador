import struct
import socket
from safe_socket import recv_all, send_all
from lottery import Bet

# Opcodes (1 byte)
SEND_BET = 0x01
GET_WINNERS = 0x02
SEND_WINNERS = 0x03
EXIT = 0x00

class ServerProtocol:
    def __init__(self, skt: socket.socket):
        self.skt = skt

    def receive_action_code(self) -> int:
        """Lee el opcode inicial"""
        bytes_received = recv_all(self.skt, 1)
        return bytes_received[0]

    def receive_bet(self) -> Bet:
        """
        Recibe una apuesta del cliente
        - Header de 6 bytes: Agency ID (uint16) + Payload Len (uint32)
        - Payload: contenido string
        """
        header = recv_all(self.skt, 6)
        # desempaqueta los bytes, !->Bgendian H->Short(2B) I->Integer(4B)
        agency_id, payload_len = struct.unpack("!HI", header) 

        payload_bytes = recv_all(self.skt, payload_len)
        csv_str = payload_bytes.decode("utf-8")

        fields = [f.strip() for f in csv_str.split(",")]
        return Bet(
            agency_id=int(agency_id),
            first_name=fields[0],
            last_name=fields[1],
            document=int(fields[2]),
            birthdate=fields[3],
            number=int(fields[4]),
        )

    def receive_agency_id(self) -> int:
        """Recibe el ID de la agencia en una consulta (2 bytes Big Endian)"""
        id_bytes = recv_all(self.skt, 2)
        return struct.unpack("!H", id_bytes)[0]

    def send_winners(self, winners: list[str]):
        """
        Envia los ganadores:
        - Opcode (1B)
        - Cantidad de ganadores (2B)
        - Por cada ganador: largo del payload (2B) + bytes del payload
        """
        # Opcode + Cantidad de ganadores
        header = struct.pack("!BH", SEND_WINNERS, len(winners))
        send_all(self.skt, header)

        for winner in winners:
            winner_bytes = winner.encode("utf-8")
            # uint16 con el tamaño del packete + el payload
            packet = struct.pack("!H", len(winner_bytes)) + winner_bytes
            send_all(self.skt, packet)

    def close(self):
        try:
            self.skt.shutdown(socket.SHUT_RDWR)
        except OSError:
            pass
        self.skt.close()