package routing

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/ipfs/go-datastore"
	drc "github.com/ipfs/go-delegated-routing/client"
	drp "github.com/ipfs/go-delegated-routing/gen/proto"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/kubo/config"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multicodec"
)

var log = logging.Logger("routing/delegated")

func Parse(routers config.Routers, methods config.Methods, extraDHT *ExtraDHTParams, extraReframe *ExtraReframeParams) (routing.Routing, error) {
	createdRouters := make(map[string]routing.Routing)
	processLater := make(config.Routers)
	log.Info("starting to parse ", len(routers), " routers")
	for k, r := range routers {
		if !r.Enabled.WithDefault(true) {
			continue
		}

		if r.Type == config.RouterTypeSequential ||
			r.Type == config.RouterTypeParallel {
			processLater[k] = r
			continue
		}
		log.Info("creating router ", k)
		router, err := routingFromConfig(r.Router, extraDHT, extraReframe, nil, nil)
		if err != nil {
			return nil, err
		}

		log.Info("router ", k, " created with params ", r.Parameters)

		createdRouters[k] = router
	}

	// using the createdRouters, instantiate all parallel and sequential routers
	for k, r := range processLater {
		crp, ok := r.Router.Parameters.(*config.ComposableRouterParams)
		if !ok {
			return nil, fmt.Errorf("problem getting composable router Parameters from router %q", k)
		}

		log.Info("creating router helper ", k)
		router, err := routingFromConfig(r.Router, extraDHT, extraReframe, crp, createdRouters)
		if err != nil {
			return nil, err
		}

		createdRouters[k] = router

		log.Info("router ", k, " created with params ", r.Parameters)
	}

	if err := methods.Check(); err != nil {
		return nil, err
	}

	finalRouter := &Composer{}
	for mn, m := range methods {
		router, ok := createdRouters[m.RouterName]
		if !ok {
			return nil, fmt.Errorf("router with name %q not found for method %q", m.RouterName, mn)
		}
		switch mn {
		case config.MethodNamePutIPNS:
			finalRouter.PutValueRouter = router
		case config.MethodNameGetIPNS:
			finalRouter.GetValueRouter = router
		case config.MethodNameFindPeers:
			finalRouter.FindPeersRouter = router
		case config.MethodNameFindProviders:
			finalRouter.FindProvidersRouter = router
		case config.MethodNameProvide:
			finalRouter.ProvideRouter = router
		}

		log.Info("using method ", mn, " with router ", m.RouterName)
	}

	return finalRouter, nil
}

func routingFromConfig(conf config.Router,
	extraDHT *ExtraDHTParams,
	extraReframe *ExtraReframeParams,
	extraComposableParams *config.ComposableRouterParams,
	routers map[string]routing.Routing,
) (routing.Routing, error) {
	var router routing.Routing
	var err error
	switch conf.Type {
	case config.RouterTypeReframe:
		router, err = reframeRoutingFromConfig(conf, extraReframe)
	case config.RouterTypeDHT:
		router, err = dhtRoutingFromConfig(conf, extraDHT)
	case config.RouterTypeParallel:
		if extraComposableParams == nil || routers == nil {
			err = fmt.Errorf("missing params needed to create a composable router")
			break
		}
		var pr []*routinghelpers.ParallelRouter
		for _, cr := range extraComposableParams.Routers {
			ri, ok := routers[cr.RouterName]
			if !ok {
				err = fmt.Errorf("router with name %q not found. If you have a router with this name, "+
					"check routers order in configuration. Take into account that nested parallel and/or sequential "+
					"routers are not supported", cr.RouterName)
				break
			}

			pr = append(pr, &routinghelpers.ParallelRouter{
				Router:       ri,
				IgnoreError:  cr.IgnoreErrors,
				Timeout:      cr.Timeout.Duration,
				ExecuteAfter: cr.ExecuteAfter.WithDefault(0),
			})
		}

		router = routinghelpers.NewComposableParallel(pr)
	case config.RouterTypeSequential:
		if extraComposableParams == nil || routers == nil {
			err = fmt.Errorf("missing params needed to create a composable router")
			break
		}
		var sr []*routinghelpers.SequentialRouter
		for _, cr := range extraComposableParams.Routers {
			ri, ok := routers[cr.RouterName]
			if !ok {
				err = fmt.Errorf("router with name %q not found", cr.RouterName)
				break
			}

			sr = append(sr, &routinghelpers.SequentialRouter{
				Router:      ri,
				IgnoreError: cr.IgnoreErrors,
				Timeout:     cr.Timeout.Duration,
			})
		}

		router = routinghelpers.NewComposableSequential(sr)
	default:
		return nil, fmt.Errorf("unknown router type %q", conf.Type)
	}

	return router, err
}

type ExtraReframeParams struct {
	PeerID     string
	Addrs      []string
	PrivKeyB64 string
}

func reframeRoutingFromConfig(conf config.Router, extraReframe *ExtraReframeParams) (routing.Routing, error) {
	var dr drp.DelegatedRouting_Client

	params := conf.Parameters.(*config.ReframeRouterParams)

	if params.Endpoint == "" {
		return nil, NewParamNeededErr("Endpoint", conf.Type)
	}

	// Increase per-host connection pool since we are making lots of concurrent requests.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 500
	transport.MaxIdleConnsPerHost = 100

	delegateHTTPClient := &http.Client{
		Transport: transport,
	}
	dr, err := drp.New_DelegatedRouting_Client(params.Endpoint,
		drp.DelegatedRouting_Client_WithHTTPClient(delegateHTTPClient),
	)
	if err != nil {
		return nil, err
	}

	var c *drc.Client

	// this path is for tests only
	if extraReframe == nil {
		c, err = drc.NewClient(dr, nil, nil)
		if err != nil {
			return nil, err
		}
	} else {
		prov, err := createProvider(extraReframe.PeerID, extraReframe.Addrs)
		if err != nil {
			return nil, err
		}

		key, err := decodePrivKey(extraReframe.PrivKeyB64)
		if err != nil {
			return nil, err
		}

		c, err = drc.NewClient(dr, prov, key)
		if err != nil {
			return nil, err
		}
	}

	crc := drc.NewContentRoutingClient(c)
	return &reframeRoutingWrapper{
		Client:               c,
		ContentRoutingClient: crc,
	}, nil
}

func decodePrivKey(keyB64 string) (ic.PrivKey, error) {
	pk, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, err
	}

	return ic.UnmarshalPrivateKey(pk)
}

func createProvider(peerID string, addrs []string) (*drc.Provider, error) {
	pID, err := peer.Decode(peerID)
	if err != nil {
		return nil, err
	}

	var mas []ma.Multiaddr
	for _, a := range addrs {
		m, err := ma.NewMultiaddr(a)
		if err != nil {
			return nil, err
		}

		mas = append(mas, m)
	}

	return &drc.Provider{
		Peer: peer.AddrInfo{
			ID:    pID,
			Addrs: mas,
		},
		ProviderProto: []drc.TransferProtocol{
			{Codec: multicodec.TransportBitswap},
		},
	}, nil
}

type ExtraDHTParams struct {
	BootstrapPeers []peer.AddrInfo
	Host           host.Host
	Validator      record.Validator
	Datastore      datastore.Batching
	Context        context.Context
}

func dhtRoutingFromConfig(conf config.Router, extra *ExtraDHTParams) (routing.Routing, error) {
	params, ok := conf.Parameters.(*config.DHTRouterParams)
	if !ok {
		return nil, errors.New("incorrect params for DHT router")
	}

	if params.AcceleratedDHTClient {
		return createFullRT(extra)
	}

	var mode dht.ModeOpt
	switch params.Mode {
	case config.DHTModeAuto:
		mode = dht.ModeAuto
	case config.DHTModeClient:
		mode = dht.ModeClient
	case config.DHTModeServer:
		mode = dht.ModeServer
	default:
		return nil, fmt.Errorf("invalid DHT mode: %q", params.Mode)
	}

	return createDHT(extra, params.PublicIPNetwork, mode)
}

func createDHT(params *ExtraDHTParams, public bool, mode dht.ModeOpt) (routing.Routing, error) {
	var opts []dht.Option

	if public {
		opts = append(opts, dht.QueryFilter(dht.PublicQueryFilter),
			dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			dht.RoutingTablePeerDiversityFilter(dht.NewRTPeerDiversityFilter(params.Host, 2, 3)))
	} else {
		opts = append(opts, dht.ProtocolExtension(dual.LanExtension),
			dht.QueryFilter(dht.PrivateQueryFilter),
			dht.RoutingTableFilter(dht.PrivateRoutingTableFilter))
	}

	opts = append(opts,
		dht.Concurrency(10),
		dht.Mode(mode),
		dht.Datastore(params.Datastore),
		dht.Validator(params.Validator),
		dht.BootstrapPeers(params.BootstrapPeers...))

	return dht.New(
		params.Context, params.Host, opts...,
	)
}

func createFullRT(params *ExtraDHTParams) (routing.Routing, error) {
	return fullrt.NewFullRT(params.Host,
		dht.DefaultPrefix,
		fullrt.DHTOption(
			dht.Validator(params.Validator),
			dht.Datastore(params.Datastore),
			dht.BootstrapPeers(params.BootstrapPeers...),
			dht.BucketSize(20),
		),
	)
}
