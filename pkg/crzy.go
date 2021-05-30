package pkg

import (
	"context"
	"errors"
	"os"

	log "github.com/go-crzy/crzy/logr"
	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
)

var (
	ErrVersionRequested   = errors.New("version")
	ErrWronglyInitialized = errors.New("wronginit")
)

// DefaultRunner holds Crzy configuration. Options are embedded, instances
// should be created with the NewCrzy function.
type DefaultRunner struct {
	log       logr.Logger
	container container
}

// NewCrzy creates the DefaultRunner with the various configuration options.
func NewCrzy() *DefaultRunner {
	log := log.NewLogger("", log.OptionColor)
	container := &defaultContainer{
		log:    log,
		out:    os.Stdout,
		parser: &argsParser{},
	}
	return &DefaultRunner{
		log:       log,
		container: container,
	}
}

// Run starts the DefaultRunner and runs Crzy
func (c *DefaultRunner) Run(ctx context.Context) error {
	if c.log == nil {
		return ErrWronglyInitialized
	}
	log := c.log.WithName("main")
	err := c.container.load()
	if err != nil {
		return err
	}
	heading(log)
	group, ctx := errgroup.WithContext(ctx)
	store, err := c.container.createStore()
	if err != nil {
		log.Error(err, "could not create store")
		return err
	}
	defer store.delete()
	state := c.container.newStateManager()
	gitCommand, err := c.container.newDefaultGitCommand(*store)
	if err != nil {
		log.Error(err, "could not get git")
		return err
	}
	trigger := make(chan event)
	defer close(trigger)
	release := make(chan event)
	defer close(release)
	gitServer, err := c.container.newGitServer(*store, state, trigger, release)
	if err != nil {
		log.Error(err, "could not initialize git")
		return err
	}
	listener1, err := c.container.newHTTPListener(listenerAPIAddr)
	if err != nil {
		log.Error(err, "could not start git listener")
		return err
	}
	upstream := newUpstream(state.state)
	f := upstream.setDefault
	proxy := newReverseProxy(upstream)
	listener2, err := c.container.newHTTPListener(listenerProxyAddr)
	if err != nil {
		log.Error(err, "could not start proxy listener")
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	group.Go(func() error { return c.container.newSignalHandler().run(ctx, cancel) })
	group.Go(func() error { return listener1.run(ctx, *gitServer.ghx) })
	group.Go(func() error { return listener2.run(ctx, proxy) })
	group.Go(func() error { return c.container.createAndStartWorkflows(ctx, state, gitCommand, trigger, release, f) })
	err = group.Wait()
	return err
}

func heading(log logr.Logger) {
	log.Info("")
	log.Info(" █▀▀ █▀▀█ ▀▀█ █░░█")
	log.Info(" █░░ █▄▄▀ ▄▀░ █▄▄█")
	log.Info(" ▀▀▀ ▀░▀▀ ▀▀▀ ▄▄▄█")
	log.Info("")
}
