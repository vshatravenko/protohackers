#!/usr/bin/env python3

import socket

HOST = "127.0.0.1"
PORT = 4269
BUF_SIZE = 1000

INIT_MESSAGE = b"miffler\n"
PAYLOAD = "So here's my address - 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX\n"

print("Acquiring socket")
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

print(f"Connecting to {HOST}:{PORT}")
sock.connect((HOST, PORT))

print(f"Sending {str(INIT_MESSAGE)}")
sock.send(INIT_MESSAGE)

sock.recv(BUF_SIZE)

for c in PAYLOAD:
    print(f"Sending {str(c)}")
    b = c.encode("utf-8")
    sock.send(b)

print("Closing the socket")
sock.close()
