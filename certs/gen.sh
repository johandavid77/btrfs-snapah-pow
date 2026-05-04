#!/bin/bash
set -e
cd "$(dirname "$0")"

echo "🔐 Generando CA..."
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
  -subj "/C=CO/ST=Valle/L=Cali/O=SnapahPow/CN=SnapahPow-CA"

echo "🔐 Generando certificado servidor..."
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr \
  -subj "/C=CO/ST=Valle/L=Cali/O=SnapahPow/CN=snapah-server"
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt \
  -extfile <(echo "subjectAltName=DNS:localhost,DNS:snapah-server,IP:127.0.0.1")

echo "🔐 Generando certificado agente..."
openssl genrsa -out agent.key 2048
openssl req -new -key agent.key -out agent.csr \
  -subj "/C=CO/ST=Valle/L=Cali/O=SnapahPow/CN=snapah-agent"
openssl x509 -req -days 365 -in agent.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out agent.crt \
  -extfile <(echo "subjectAltName=DNS:localhost,DNS:snapah-agent,IP:127.0.0.1")

rm -f *.csr *.srl
echo "✅ Certificados generados en certs/"
ls -la *.crt *.key
