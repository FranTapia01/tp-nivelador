import struct
import socket
from safe_socket import recv_all, send_all
from lottery import Bet

# Opcodes (1 byte)
CODE_SEND_BATCH = 0x01
CODE_GET_WINNERS = 0x02
CODE_SEND_WINNERS = 0x03
CODE_LAST_BATCH = 0x04
CODE_ACK = 0x05
CODE_EXIT = 0x00

class ServerProtocol:
    def __init__(self, skt: socket.socket):
        self.skt = skt

    def receive_action_code(self) -> int:
        """Lee el opcode inicial"""
        bytes_received = recv_all(self.skt, 1)
        return bytes_received[0]

    def receive_batch(self) -> list[Bet]:
        """
        Lee:
        - Agency ID (uint16) + Cantidad de apuestas (uint16) -> 4 bytes
        - Por cada apuesta: largo (uint16) + CSV bytes
        """
        header = recv_all(self.skt, 4)
        agency_id, count = struct.unpack("!HH", header)

        bets = []
        for _ in range(count):
            len_buf = recv_all(self.skt, 2)
            payload_len = struct.unpack("!H", len_buf)[0]

            csv_bytes = recv_all(self.skt, payload_len)
            csv_str = csv_bytes.decode("utf-8")

            fields = [f.strip() for f in csv_str.split(",")]
            bets.append(
                Bet(
                    agency_id=int(agency_id),
                    first_name=fields[0],
                    last_name=fields[1],
                    document=int(fields[2]),
                    birthdate=fields[3],
                    number=int(fields[4]),
                )
            )
        return bets

    def send_ack(self):
        """Envía el byte de confirmación del batch."""
        send_all(self.skt, bytes([CODE_ACK]))


    def receive_agency_id(self) -> int:
        """Recibe el ID de la agencia en una consulta (2 bytes Big Endian)"""
        id_bytes = recv_all(self.skt, 2)
        return struct.unpack("!H", id_bytes)[0]

    def send_winners(self, winners: list[str]):
        """
        Envia los ganadores en un único buffer consolidado:
        - Opcode (1B)
        - Cantidad de ganadores (2B)
        - Por cada ganador: largo del payload (2B) + bytes del payload
        """
        buf = bytearray()
        # Header: Opcode (1B) + Cantidad de ganadores (2B)
        buf.extend(struct.pack("!BH", CODE_SEND_WINNERS, len(winners)))

        for winner in winners:
            winner_bytes = winner.encode("utf-8")
            buf.extend(struct.pack("!H", len(winner_bytes)))
            buf.extend(winner_bytes)

        send_all(self.skt, bytes(buf))

    def close(self):
        try:
            self.skt.shutdown(socket.SHUT_RDWR)
        except OSError:
            pass
        self.skt.close()