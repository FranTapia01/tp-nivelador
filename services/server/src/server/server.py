import os
import threading
import signal
import socket
import logger
from protocol.server_protocol import ServerProtocol, CODE_SEND_BATCH, CODE_GET_WINNERS, CODE_EXIT, CODE_LAST_BATCH
from lottery import Lottery


class Server:
    def __init__(self, server_host: str, server_port: int, storage_path: str = "bets.csv") -> None:
        self.server_host = server_host
        self.server_port = server_port
        self.lottery = Lottery(storage_path)

        self.quorum_min = int(os.getenv("AGENCY_QUORUM_MIN", "5"))

        self.storage_lock = threading.Lock()   # Exclusion mutua para bets.csv
        self.quorum_cv = threading.Condition() # Condition variable para liberar a los ganadores
        self.agencies_ready: set[int] = set()  # IDs de agencias que terminaron su carga

        self.running = True
        self.server_socket = None
        self.client_threads: list[threading.Thread] = []

        signal.signal(signal.SIGTERM, self._handle_signal)
        # signal.signal(signal.SIGINT, self._handle_signal)

    def _handle_signal(self, *args):
        """Handler graceful ante SIGTERM"""
        self.running = False

        # Despertar a cualquier hilo trabado en la condition variable por falta de quorum
        with self.quorum_cv:
            self.quorum_cv.notify_all()

        # Desbloquear accept() cerrando el socket pasivo
        if self.server_socket:
            try:
                self.server_socket.close()
            except OSError:
                pass

    def _handle_client(self, client_socket):
        action = "handle-client"
        protocol = ServerProtocol(client_socket)
        current_agency_id = None
        bets_received = 0
        try:
            while True:
                code = protocol.receive_action_code()

                if code == CODE_EXIT:
                    break

                if code == CODE_SEND_BATCH:
                    bets = protocol.receive_batch()
                    if bets and current_agency_id is None:
                        current_agency_id = bets[0].agency_id
                    
                    # Escritura concurrente con mutex
                    with self.storage_lock:
                        self.lottery.store_bets(bets)

                    protocol.send_ack()
                    bets_received += len(bets)

                elif code == CODE_LAST_BATCH:
                    current_agency_id = protocol.receive_agency_id()
                    with self.quorum_cv:
                        self.agencies_ready.add(current_agency_id)
                        if len(self.agencies_ready) >= self.quorum_min:
                            self.quorum_cv.notify_all()
                    
                elif code == CODE_GET_WINNERS:
                    # Si la agencia no mando sus bets pero consulta igual
                    if current_agency_id is None:
                        current_agency_id = protocol.receive_agency_id()

                    with self.quorum_cv:
                        # Si se apagó el servidor o se cumplió el quórum, salir de la espera
                        while len(self.agencies_ready) < self.quorum_min and self.running:
                            self.quorum_cv.wait()

                    if not self.running:
                        break

                    with self.storage_lock:
                        all_bets = list(self.lottery.load_bets())

                    winners = [
                        f"{b.first_name},{b.last_name},{b.document},{b.birthdate},{b.number}"
                        for b in all_bets
                        if b.agency_id == current_agency_id and self.lottery.has_won(b)
                    ]

                    protocol.send_winners(winners)
                    break
                else:
                    break

            logger.info(action, logger.LogResult.success, "bets", bets_received)

        except Exception as e:
            logger.error(action, logger.LogResult.fail, "error", str(e))
        finally:
            protocol.close()


    def run(self):
        action = "accept-connection"
        self.server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.server_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self.server_socket.bind((self.server_host, self.server_port))
        self.server_socket.listen()


        while self.running:
            try:
                logger.info(action, logger.LogResult.in_progress)
                client_socket, _ = self.server_socket.accept()
                logger.info(action, logger.LogResult.success)

                # Un client handler en un nuevo hilo por cada cliente aceptado
                t = threading.Thread(target=self._handle_client, args=(client_socket,))
                t.start()
                self.client_threads.append(t)

            except OSError:
                # Ocurre cuando _handle_signal cierra self.server_socket para cortar accept()
                break
   
        # Esperar a que los hilos activos finalicen su trabajo
        for t in self.client_threads:
            t.join(timeout=3.0)