#!/bin/bash

# Variables
CA_CN="My CA"
SERVER_IP="172.168.0.16"
CLIENT_IP="172.168.0.17"
DAYS=365

# Create directories for storing certificates and keys
mkdir -p ca server client

# Step 1: Generate CA's private key and self-signed certificate
openssl genpkey -algorithm RSA -out ca/ca.key
openssl req -x509 -new -nodes -key ca/ca.key -sha256 -days $DAYS -out ca/ca.crt -subj "/CN=$CA_CN"

# Step 2: Generate server's private key and certificate signing request (CSR)
openssl genpkey -algorithm RSA -out server/server.key
openssl req -new -key server/server.key -out server/server.csr -subj "/CN=$SERVER_IP"

# Step 3: Create a config file for the server certificate with the IP SAN
cat > server/server.cnf <<EOF
[req]
distinguished_name=req_distinguished_name
[req_distinguished_name]

[ v3_req ]
subjectAltName = @alt_names

[alt_names]
IP.1 = $SERVER_IP
EOF

# Step 4: Sign the server CSR with the CA's private key to create the server certificate
openssl x509 -req -in server/server.csr -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial -out server/server.crt -days $DAYS -sha256 -extfile server/server.cnf -extensions v3_req

# Step 5: Generate client's private key and certificate signing request (CSR)
openssl genpkey -algorithm RSA -out client/client.key
openssl req -new -key client/client.key -out client/client.csr -subj "/CN=$CLIENT_IP"

# Step 6: Create a config file for the client certificate with the IP SAN
cat > client/client.cnf <<EOF
[req]
distinguished_name=req_distinguished_name
[req_distinguished_name]

[ v3_req ]
subjectAltName = @alt_names

[alt_names]
IP.1 = $CLIENT_IP
EOF

# Step 7: Sign the client CSR with the CA's private key to create the client certificate
openssl x509 -req -in client/client.csr -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial -out client/client.crt -days $DAYS -sha256 -extfile client/client.cnf -extensions v3_req

echo "Certificates and keys have been generated."
echo "CA Certificate: ca/ca.crt"
echo "Server Certificate: server/server.crt"
echo "Server Key: server/server.key"
echo "Client Certificate: client/client.crt"
echo "Client Key: client/client.key"
