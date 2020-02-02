package main

import (
	"context"
	"fmt"
	"os"
	"sync"
    "flag"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	discovery "github.com/libp2p/go-libp2p-discovery"
	p2pd "github.com/libp2p/go-libp2p-daemon"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	maddr "github.com/multiformats/go-multiaddr"

    log "github.com/sirupsen/logrus"
)



const pubsubTopicFormat = "/libp2p/bluehat/%s/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA/chat/1.0.0" // This defines the pubsub message, and as such the room
const RendezvousString = "interplantery_chat" // This is used for DHT discovery across entire chat realm
var defaultBootstrapPeers []string = []string{
	"/dns4/interplanetary-chat.francecentral.cloudapp.azure.com/tcp/3000/p2p/QmSaJb3ZAbFhKPECDp489ZgkoZwjVaSqWgNZfFWqRTeT7V",
}

type InterplanteryChat struct {
    chat_room string

    p2p_host host.Host
    dht *kaddht.IpfsDHT
    ps *pubsub.PubSub
    subscription *pubsub.Subscription
    dhtRoutingDiscovery *discovery.RoutingDiscovery
	defaultBootstrapPeers []string

    pubsubTopic string

    protocol *InterplanteryProtocol
    crypto *InterplanteryCrypto
}

func CreateTopicFromRoomName(chat_room string) string {
    return fmt.Sprintf(pubsubTopicFormat, chat_room)
}

func NewInterplanteryChat(chat_room string, passphrase string) *InterplanteryChat {
    var crypto *InterplanteryCrypto // no crypto by default
    if passphrase != "" { // If passphrase for chat room, init crypto
        // Init crypto
        crypto = NewInterplanteryCrypto(passphrase, chat_room, NewBHRNG())
    }
    protocol := NewInterplanteryProtocol(crypto)

    return &InterplanteryChat {
        chat_room: chat_room,
        protocol: protocol,
        crypto: crypto,
    }
}

func (ip *InterplanteryChat) Init(ctx context.Context) (err error) {
	DHTRoutingFactory := func(h host.Host) (routing.PeerRouting, error) {
        // Set up routing, based on our dht member
		var err error
		ip.dht, err = kaddht.New(ctx, h)
		return ip.dht, err
	}
	routing := libp2p.Routing(DHTRoutingFactory)

	// Get identity from file, if possible
	private_key, err := p2pd.ReadIdentity(".bootstrapper_identity")
	if err != nil {
		fmt.Println("Generating new identity.")
		private_key, _, _ = crypto.GenerateKeyPair(crypto.RSA, 2048) // possibly a throwaway
		p2pd.WriteIdentity(private_key, ".bootstrapper_identity") // cache key
	}

	// Setup libp2p host object, with our required options
	ip.p2p_host, err = libp2p.New(
		ctx,
		routing,
		libp2p.Identity(private_key),
	)
	if err != nil {
        log.WithError(err).Fatal("Could not create libp2p host object.")
        return err
	}

    log.WithFields(log.Fields{
        "NodeID": ip.p2p_host.ID().Pretty(),
    }).Info("Host object created.")

	// Setup Gossipsub instance (for distributed pubsub messaging)
	ip.ps, err = pubsub.NewGossipSub(ctx, ip.p2p_host)
	if err != nil {
        log.WithError(err).Fatal("Could not create libp2p pubsub object.")
        return err
	}

    ip.pubsubTopic = CreateTopicFromRoomName(ip.chat_room)
	// Subscribe to our topic (chat room name)
	ip.subscription, err = ip.ps.Subscribe(ip.pubsubTopic)
	if err != nil {
        log.WithError(err).Fatal("Could not subscribe to topic.")
        return err
	}

    log.Debug("Interplantery chat object initialized.")

    return nil
}

func (ip* InterplanteryChat) Discover(ctx context.Context) (err error) {
    // Discover new peers using the dht routing discovery object
    // Find arbitrary amount of initial peers
    log.Debug("Starting discovery cycle.")
	peerChan, err := ip.dhtRoutingDiscovery.FindPeers(ctx, RendezvousString)
	if err != nil {
        log.WithError(err).Error("Could not initiate peer discovery.")
        return err
	}

    // Wait for all peers to return from the channel
	var wg sync.WaitGroup
	for peerInfo := range peerChan {
		// New peer found, connect to it using the routing host
        wg.Add(1)
		go func() {
			defer wg.Done()
            err := ip.p2p_host.Connect(ctx, peerInfo)
			if err != nil {
                log.WithError(err).Debug("Could not connect to peer.")
			} else {
                log.WithFields(log.Fields{
                    "NodeID": peerInfo.ID.Pretty(),
                }).Info("Connected to new peer.")
			}
		}()
	}
    wg.Wait()
    log.Debug("Discovery cycle finished.")

    return nil
}

func (ip *InterplanteryChat) Bootstrap(ctx context.Context) (err error) {
	// Bootstrap DHT 
	err = ip.dht.Bootstrap(ctx)
	if err != nil {
        log.WithError(err).Error("Could not bootstrap DHT object.")
        return err
	}

    log.Debug("Bootstrapped, connecting to default bootstrap node set")
    // Connect to all our bootstrap nodes
	var wg sync.WaitGroup
	for _, peerAddr := range defaultBootstrapPeers {
		peerMAddr, _ := maddr.NewMultiaddr(peerAddr)
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerMAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
            err := ip.p2p_host.Connect(ctx, *peerinfo)
			if err != nil {
                log.WithError(err).Debug("Could not bootstrap DHT object.")
			} else {
                log.WithFields(log.Fields{
                    "NodeID": peerinfo.ID.Pretty(),
                }).Info("Connected to new bootstrap node.")
			}
		}()
	}
	wg.Wait()

    // Set up discovery and advertise
	ip.dhtRoutingDiscovery = discovery.NewRoutingDiscovery(ip.dht)
	discovery.Advertise(ctx, ip.dhtRoutingDiscovery, RendezvousString)

    // Do initial discovery
    go ip.Discover(ctx)

    return nil
}

func (ip *InterplanteryChat) SendMessage(msg string) {
    // Preprocess message here
    log.WithFields(log.Fields{"msg": msg}).Debug("Sending message...")
    msg_bytes := ip.protocol.NewInterplanteryMessage(msg)
    ip.ps.Publish(ip.pubsubTopic, msg_bytes)
}

func (ip *InterplanteryChat) outboundMessageLoop(ctx context.Context, outboundMessageChannel chan string) {
    // For symmetrys sake, feed in from outbound message channel until context dies
    log.Debug("Starting outbound message loop.")
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-outboundMessageChannel:
            ip.SendMessage(msg)
        }
    }
}

func (ip *InterplanteryChat) inboundMessageLoop(ctx context.Context, inboundMessageChannel chan string) {
    log.Debug("Starting inbound message loop.")
    // This is the routine responsible for handling incoming messages.
    for {
        // TODO - check what happens when this context is cancelled
        select {
        case <-ctx.Done():
            log.Debug("Message loop ended.")
            // were done
            return
        default:
            // Still running, accept messages
            log.Debug("Waiting for msg")
            msg, err := ip.subscription.Next(ctx)
            if err != nil {
                log.WithError(err).Warn("Message receive failed.")
                continue
            }
            log.Debug("msg received")

            // Process message here
            member_id := msg.GetFrom().Pretty()
            member_handle := member_id[len(member_id)-8:] // Derive user handle from peer id
            // Unmarshal message
            ip_msg := ip.protocol.ProcessInterplanteryMessage(msg.Data)
            if ip_msg == nil {
                log.WithFields(log.Fields {
                    "msg_bytes": msg.Data,
                }).Warn("Failed to process incoming message!")
                continue
            }

            // Feed processsed messages to outgoing message pipe
            inboundMessageChannel <- fmt.Sprintf("<%s>: %s\n", member_handle, *ip_msg.Text)
        }
    }
}

func (ip *InterplanteryChat) StartReceiving(ctx context.Context, inboundMessageChannel chan string) {
    // Start message loop - only call this once
    go ip.inboundMessageLoop(ctx, inboundMessageChannel)
}

func (ip *InterplanteryChat) StartSending(ctx context.Context, outboundMessageChannel chan string) {
    // Start message loop - only call this once
    go ip.outboundMessageLoop(ctx, outboundMessageChannel)
}


func main() {
    // Setup logger stuff
	var err error
    //log.SetLevel(log.DebugLevel)
	f, err := os.OpenFile("interplantery-chat.log", os.O_WRONLY | os.O_CREATE, 0755)
	if err != nil {
		panic("Can not set up logging!")
	}
	log.SetOutput(f)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

    // Setup chat client
    chat_room := flag.String("room", "BlueHat2020Lobby", "Chat room to join")
    room_passphrase := flag.String("passphrase", "", "Passphrase (Optional)")
    flag.Parse()

    ip_chat := NewInterplanteryChat(*chat_room, *room_passphrase)

    err = ip_chat.Init(ctx)
    if err != nil {
        log.WithError(err).Panic("Failed to init chat :(")
    }

    err = ip_chat.Bootstrap(ctx)
    if err != nil {
        log.WithError(err).Panic("Failed to bootstrap chat :(")
    }

    message_in_chan := make(chan string, 100)
    message_out_chan := make(chan string, 100)

    ip_chat.StartReceiving(ctx, message_in_chan)
    ip_chat.StartSending(ctx, message_out_chan)

    // Setup UI
	ui := NewChatUI(ip_chat.chat_room, message_in_chan, message_out_chan)
	ui.Start()

}
