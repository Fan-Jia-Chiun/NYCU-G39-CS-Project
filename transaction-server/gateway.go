package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
)

const (
	defaultMSPID         = "Org1MSP"
	defaultChannelName   = "mychannel"
	defaultChaincodeName = "transaction"
	defaultPeerEndpoint  = "dns:///localhost:7051"
	defaultGatewayPeer   = "peer0.org1.example.com"
	defaultFabricUser    = "TransactionUser@org1.example.com"
)

type FabricGateway struct {
	Gateway          *client.Gateway
	Network          *client.Network
	Contract         *client.Contract
	ClientConnection *grpc.ClientConn
	ChannelName      string
	ChaincodeName    string
}

func InitFabricGateway() (*FabricGateway, error) {
	paths, err := defaultFabricPaths()
	if err != nil {
		return nil, err
	}

	mspID := envOrDefault("FABRIC_MSP_ID", defaultMSPID)
	channelName := envOrDefault("FABRIC_CHANNEL_NAME", defaultChannelName)
	chaincodeName := envOrDefault("FABRIC_CHAINCODE_NAME", defaultChaincodeName)
	peerEndpoint := envOrDefault("FABRIC_PEER_ENDPOINT", defaultPeerEndpoint)
	gatewayPeer := envOrDefault("FABRIC_GATEWAY_PEER", defaultGatewayPeer)

	clientConnection, err := newGrpcConnection(paths.TLSCertPath, peerEndpoint, gatewayPeer)
	if err != nil {
		return nil, err
	}
	if err := waitForGrpcReady(clientConnection, 5*time.Second); err != nil {
		clientConnection.Close()
		return nil, err
	}

	id, err := newIdentity(mspID, paths.CertPath)
	if err != nil {
		clientConnection.Close()
		return nil, err
	}

	sign, err := newSign(paths.KeyPath)
	if err != nil {
		clientConnection.Close()
		return nil, err
	}

	gateway, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithHash(hash.SHA256),
		client.WithClientConnection(clientConnection),
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		clientConnection.Close()
		return nil, fmt.Errorf("failed to connect to Fabric Gateway: %w", err)
	}

	network := gateway.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)

	return &FabricGateway{
		Gateway:          gateway,
		Network:          network,
		Contract:         contract,
		ClientConnection: clientConnection,
		ChannelName:      channelName,
		ChaincodeName:    chaincodeName,
	}, nil
}

func (fg *FabricGateway) Close() {
	if fg == nil {
		return
	}
	if fg.Gateway != nil {
		fg.Gateway.Close()
	}
	if fg.ClientConnection != nil {
		_ = fg.ClientConnection.Close()
	}
}

type fabricPaths struct {
	CertPath    string
	KeyPath     string
	TLSCertPath string
}

func defaultFabricPaths() (fabricPaths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fabricPaths{}, fmt.Errorf("failed to get user home directory: %w", err)
	}

	cryptoPath := envOrDefault(
		"FABRIC_CRYPTO_PATH",
		filepath.Join(homeDir, "fabric", "fabric-samples", "test-network", "organizations", "peerOrganizations", "org1.example.com"),
	)
	fabricUser := envOrDefault("FABRIC_USER_NAME", defaultFabricUser)

	return fabricPaths{
		CertPath: envOrDefault(
			"FABRIC_CERT_PATH",
			filepath.Join(cryptoPath, "users", fabricUser, "msp", "signcerts"),
		),
		KeyPath: envOrDefault(
			"FABRIC_KEY_PATH",
			filepath.Join(cryptoPath, "users", fabricUser, "msp", "keystore"),
		),
		TLSCertPath: envOrDefault(
			"FABRIC_TLS_CERT_PATH",
			filepath.Join(cryptoPath, "peers", "peer0.org1.example.com", "tls", "ca.crt"),
		),
	}, nil
}

func newGrpcConnection(tlsCertPath string, peerEndpoint string, gatewayPeer string) (*grpc.ClientConn, error) {
	certificatePEM, err := os.ReadFile(tlsCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS certificate file: %w", err)
	}

	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TLS certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)

	connection, err := grpc.NewClient(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	return connection, nil
}

func waitForGrpcReady(connection *grpc.ClientConn, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	connection.Connect()
	for {
		state := connection.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if !connection.WaitForStateChange(ctx, state) {
			return fmt.Errorf("gRPC connection not ready before timeout; last state: %s", state)
		}
	}
}

func newIdentity(mspID string, certPath string) (*identity.X509Identity, error) {
	certificatePEM, err := readFirstFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read identity certificate file: %w", err)
	}

	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse identity certificate: %w", err)
	}

	id, err := identity.NewX509Identity(mspID, certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to create X.509 identity: %w", err)
	}

	return id, nil
}

func newSign(keyPath string) (identity.Sign, error) {
	privateKeyPEM, err := readFirstFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create private key signer: %w", err)
	}

	return sign, nil
}

func readFirstFile(dirPath string) ([]byte, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no files found in %s", dirPath)
	}

	return os.ReadFile(filepath.Join(dirPath, entries[0].Name()))
}

func envOrDefault(name string, defaultValue string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return defaultValue
}
