package pkg

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
)

const (
	triggeredMessage string = "triggered"
	deployedMessage  string = "deployed"
)

type event struct {
	id   string
	envs envVars
}

func (r *runContainer) createAndStartWorkflows(
	ctx context.Context,
	state *stateManager,
	git gitCommand,
	startTrigger chan event,
	switchUpstream func(string)) error {
	err := git.cloneRepository()
	if err != nil {
		r.Log.Error(err, "error cloning repository")
		return err
	}
	g, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	install := r.Config.Deploy.Install
	install.name = "install"
	test := r.Config.Deploy.Test
	test.name = "test"
	preBuild := r.Config.Deploy.PreBuild
	preBuild.name = "prebuild"
	build := r.Config.Deploy.Build
	build.name = "build"
	deploy := &deployWorkflow{
		deployStruct: r.Config.Deploy,
		workspace:    git.getWorkspace(),
		execdir:      git.getExecdir(),
		log:          r.Log,
		keys: map[string]execStruct{
			"install":   install,
			"test":      test,
			"pre_build": preBuild,
			"build":     build,
		},
		flow:  []string{"install", "test", "pre_build", "build"},
		state: &stateDefaultClient{notifier: state.notifier},
	}
	trigger := &triggerWorkflow{
		triggerStruct: r.Config.Trigger,
		head:          r.Config.Main.Head,
		log:           r.Log,
		git:           git,
		command:       &defaultTriggerCommand{},
		state:         &stateDefaultClient{notifier: state.notifier},
	}
	run := r.Config.Release.Run
	run.name = "run"
	release := &releaseWorkflow{
		releaseStruct: r.Config.Release,
		log:           r.Log,
		execdir:       git.getExecdir(),
		keys: map[string]execStruct{
			"run": run,
		},
		flow:           "run",
		processes:      map[string]*os.Process{},
		switchUpstream: switchUpstream,
		state:          &stateDefaultClient{notifier: state.notifier},
	}
	startDeploy := make(chan event)
	defer close(startDeploy)
	startRelease := make(chan event)
	defer close(startRelease)
	g.Go(func() error { return state.start(ctx) })
	g.Go(func() error { return trigger.start(ctx, startTrigger, startDeploy) })
	g.Go(func() error { return deploy.start(ctx, startDeploy, startRelease, startTrigger) })
	g.Go(func() error { return release.start(ctx, startRelease) })
	<-ctx.Done()
	cancel()
	return g.Wait()
}

type workflow struct {
	log     logr.Logger
	version string
	name    string
	basedir string
	envs    envVars
	state   stateClient
}

func (w *workflow) execute(e execStruct) (*envVar, error) {
	cmd, err := e.prepare(w.basedir, w.envs)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	output, err := cmd.CombinedOutput()
	status := runnerStatusDone
	duration := time.Since(start)
	if err != nil {
		status = runnerStatusFailed
	}
	w.state.notifyStep(
		w.version,
		w.name,
		status,
		step{
			execStruct: e,
			Name:       e.name,
			StartTime:  &start,
			Duration:   &duration,
		})
	results := strings.Split(string(output), "\n")
	for _, v := range results {
		w.log.Info(v)
	}
	if err != nil {
		return nil, err
	}
	if e.Output != "" {
		return &envVar{Name: e.Output, Value: results[0]}, nil
	}
	return nil, nil
}

func (w *workflow) start(e execStruct) (*os.Process, error) {
	cmd, err := e.prepare(w.basedir, w.envs)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	err = cmd.Start()
	status := runnerStatusStarted
	w.state.notifyStep(
		w.version,
		w.name,
		status,
		step{
			execStruct: e,
			Name:       e.name,
			StartTime:  &start,
		})
	return cmd.Process, err
}
