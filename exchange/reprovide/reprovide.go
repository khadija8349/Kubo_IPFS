package reprovide

import (
	"fmt"
	"time"

	blocks "github.com/ipfs/go-ipfs/blocks/blockstore"
	backoff "gx/ipfs/QmPJUtEJsm5YLUWhF6imvyCH8KZXRJa9Wup7FDMwTy5Ufz/backoff"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	routing "gx/ipfs/QmYQadj3iegqmRPWjaWMRc8DG52hZa2HMkmyPkto5chDvs/go-libp2p-routing"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var log = logging.Logger("reprovider")

type Reprovider struct {
	// The routing system to provide values through
	rsys routing.ContentRouting

	// The backing store for blocks to be provided
	bstore blocks.Blockstore
}

func NewReprovider(rsys routing.ContentRouting, bstore blocks.Blockstore) *Reprovider {
	return &Reprovider{
		rsys:   rsys,
		bstore: bstore,
	}
}

func (rp *Reprovider) ProvideEvery(ctx context.Context, tick time.Duration) {
	// dont reprovide immediately.
	// may have just started the daemon and shutting it down immediately.
	// probability( up another minute | uptime ) increases with uptime.
	after := time.After(time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-after:
			err := rp.Reprovide(ctx)
			if err != nil {
				log.Debug(err)
			}
			after = time.After(tick)
		}
	}
}

func (rp *Reprovider) Reprovide(ctx context.Context) error {
	keychan, err := rp.bstore.AllKeysChan(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get key chan from blockstore: %s", err)
	}
	for k := range keychan {
		op := func() error {
			err := rp.rsys.Provide(ctx, k)
			if err != nil {
				log.Debugf("Failed to provide key: %s", err)
			}
			return err
		}

		// TODO: this backoff library does not respect our context, we should
		// eventually work contexts into it. low priority.
		err := backoff.Retry(op, backoff.NewExponentialBackOff())
		if err != nil {
			log.Debugf("Providing failed after number of retries: %s", err)
			return err
		}
	}
	return nil
}
