import socket

# Excepcion para cierres inesperados
class ConnectionClosedError(Exception):
    pass

def recv_all(socket: socket.socket, size):
    chunks = []
    bytes_received = 0
    while bytes_received < size:
        chunk = socket.recv(size - bytes_received)
        if len(chunk) == 0:
            raise ConnectionClosedError("Socket connection closed during receive")
        chunks.append(chunk)
        bytes_received += len(chunk)

    return b"".join(chunks)


def send_all(socket: socket.socket, data: bytes):
    total_sent = 0
    while total_sent < len(data):
        sent = socket.send(data[total_sent:])

        total_sent += sent
