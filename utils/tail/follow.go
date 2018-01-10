package tail

import (
	"context"
	"sync"
	"time"
)

func FollowFiles(ctx context.Context, opts ...TailFilesOpt) {
	var wg sync.WaitGroup
	env := TailFilesOpts{
		nbLines: 10,
		period:  time.Second,
	}

	for _, opt := range opts {
		opt(&env)
	}

	if env.errors != nil {
		defer func() {
			wg.Wait()
			close(env.errors)
		}()
	}

	if env.results != nil {
		defer func() {
			wg.Wait()
			close(env.results)
		}()
	}

	filenames := env.allFiles()
	if len(filenames) == 0 {
		close(env.results)
		return
	}

	var fspecs fileSpecs = make([]*fileSpec, 0, len(filenames))
	var initTailWg sync.WaitGroup

	for _, filename := range filenames {
		fname := filename

		resultsChan := make(chan string)
		prefixLine(resultsChan, env.results, fname, &wg)
		errChan := make(chan error)
		prefixErrors(errChan, env.errors, fname, &wg)

		fspec := makeFspec(ctx, fname, resultsChan, errChan)
		defer fspec.close()
		fspecs = append(fspecs, fspec)
		initTailWg.Add(1)
		go fspec.initTail(ctx, &initTailWg, env.nbLines)
	}

	initTailWg.Wait()
	select {
	case <-ctx.Done():
		return
	default:
	}

	notifySpecs, classicalSpecs := fspecs.sort()

	var followWg sync.WaitGroup
	for _, fspec := range classicalSpecs {
		followClassical(ctx.Done(), &followWg, fspec, env.period)
	}

	n, err := newNotifier(env.errors)
	if err != nil && env.errors != nil {
		// meh, getcwd fails ?!
		env.errors <- err
	} else if err == nil {
		n.AddFiles(notifySpecs)
		n.Start()

		<-ctx.Done()    // wait for the context to be cancelled by the caller
		n.Stop()        // returns when the notifier has stopped
		followWg.Wait() // returns when the classical followers have stopped
	}
}

func FollowFile(ctx context.Context, opts ...TailFileOpt) {
	env := TailFileOpts{
		nbLines: 10,
		period:  time.Second,
	}

	for _, opt := range opts {
		opt(&env)
	}

	if len(env.filename) == 0 {
		return
	}

	fspec := makeFspec(ctx, env.filename, env.results, env.errors)
	defer fspec.close()

	fspec.initTail(ctx, nil, env.nbLines)

	select {
	case <-ctx.Done():
		return
	default:
	}

	if fspec.hasClassicalFollow() {
		var followWg sync.WaitGroup
		followClassical(ctx.Done(), &followWg, fspec, env.period)
		followWg.Wait()
	} else {
		n, err := newNotifier(env.errors)
		if err != nil && env.errors != nil {
			env.errors <- err
		} else if err != nil {
			n.AddFile(fspec)
			n.Start()
			<-ctx.Done()
			n.Stop()
		}
	}
}
