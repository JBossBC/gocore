package DHT

import (
	"errors"
	"math"
	"net"
	"time"
)

const (
	StandardMode = iota
	CrawlMode
)

var (
	ErrNotReady                 = errors.New("dht is not ready")
	ErrOnGetPeersResponseNotSet = errors.New("OnGetPeersResponse is not set")
)

type Config struct {
	K                    int
	KBucketSize          int
	Network              string
	Address              string
	PrimeNodes           []string
	KBucketExpiredAfter  time.Duration
	NodeExpriedAfter     time.Duration
	CheckKBucketPeriod   time.Duration
	TokenExpiredAfter    time.Duration
	MaxTransactionCursor uint64
	MaxNodes             int
	OnGetPeers           func(string, string, int)
	OnGetPeersResponse   func(string, *Peer)
	OnAnnouncePeer       func(string, string, int)
	BlockedIPs           []string
	BlackListMaxSize     int
	Mode                 int
	Try                  int
	PacketJobLimit       int
	PacketWorkerLimit    int
	RefreshNodeNum       int
}

func NewStandardConfig() *Config {
	return &Config{
		K:           8,
		KBucketSize: 8,
		Network:     "udp4",
		Address:     ":6881",
		PrimeNodes: []string{
			"router.bittorrent.com:6881",
			"router.utorrent.com:6881",
			"dht.transmissionbt.com:6881",
		},
		NodeExpriedAfter:     time.Duration(time.Minute * 15),
		KBucketExpiredAfter:  time.Duration(time.Minute * 15),
		CheckKBucketPeriod:   time.Duration(time.Second * 30),
		TokenExpiredAfter:    time.Duration(time.Minute * 10),
		MaxTransactionCursor: math.MaxUint32,
		MaxNodes:             5000,
		BlockedIPs:           make([]string, 0),
		BlackListMaxSize:     65536,
		Try:                  2,
		Mode:                 StandardMode,
		PacketJobLimit:       1024,
		PacketWorkerLimit:    256,
		RefreshNodeNum:       8,
	}
}

func NewCrawlConfig() *Config {
	config := NewStandardConfig()
	config.NodeExpriedAfter = 0
	config.KBucketExpiredAfter = 0
	config.CheckKBucketPeriod = time.Second * 5
	config.KBucketSize = math.MaxInt32
	config.Mode = CrawlMode
	config.RefreshNodeNum = 256

	return config
}

type DHT struct {
	*Config
	node               *node
	conn               *net.UDPConn
	routingTable       *routingTable
	transactionManager *transactionManager
	peersManager       *peersManager
	tokenManager       *tokenManager
	blackList          *blackList
	Ready              bool
	packets            chan packet
	workerTokens       chan struct{}
}

func New(config *Config) *DHT {
	if config == nil {
		config = NewStandardConfig()
	}
	node, err := newNode(randomString(20), config.Network, config.Address)
}
