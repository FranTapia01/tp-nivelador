import socket
import logger
from protocol.server_protocol import ServerProtocol, SEND_BET, GET_WINNERS, EXIT
from lottery import Lottery
import traceback


class Server:
    def __init__(self, server_host: str, server_port: int, storage_path: str = "bets.csv") -> None:
        self.server_host = server_host
        self.server_port = server_port
        self.lottery = Lottery(storage_path)

    def _handle_client(self, client_socket):
        action = "handle-client"
        protocol = ServerProtocol(client_socket)
        bets_received = 0
        try:
            while True:
                code = protocol.receive_action_code()

                if code == EXIT:
                    break

                if code == SEND_BET:
                    bet = protocol.receive_bet()
                    self.lottery.store_bets([bet])
                    bets_received += 1

                elif code == GET_WINNERS:
                    # guardo a el/los ganador por su dni
                    winners = [
                        str(b.document)
                        for b in self.lottery.load_bets()
                        if self.lottery.has_won(b)
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
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as server_socket:
            server_socket.bind((self.server_host, self.server_port))
            server_socket.listen()
            while True:
                try:
                    logger.info(action, logger.LogResult.in_progress)
                    client_socket, _ = server_socket.accept()
                except Exception as e:
                    logger.error(action, logger.LogResult.fail)
                    raise e
                logger.info(action, logger.LogResult.success)

                self._handle_client(client_socket)
