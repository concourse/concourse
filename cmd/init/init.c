/**
 * An init program that sets signal disposition for SIGCHLD (the signal that
 * parents usually receive when their children `exit`) to include SA_NOCLDWAIT
 * (so that we don't need to `wait` for children to have them not becoming
 * zombies when they `exit`).
 */

#include <stdio.h>
#include <unistd.h>
#include <signal.h>
#include <stddef.h>

int
main(void)
{
	struct sigaction act;

	// retrieve the action that is currently associated with SIGCHLD
	//
	if (!~sigaction(SIGCHLD, NULL, &act)) {
		perror("sigaction SIGCHLD");
		return 1;
	}

	// add SA_NOCLDWAIT ("I will not bother `wait`ing for any children) to
	// the set of flags associated with this signal so that we do not need
	// to `wait` on a child process to have it reaped once it finishes.
	//
	act.sa_flags |= SA_NOCLDWAIT;

	// set the action with our new flag set
	// 
	if (!~sigaction(SIGCHLD, &act, NULL)) {
		perror("sigaction SIGCHLD SA_NOCLDWAIT");
		return 1;
	}

	// wait for a signal to come.
	//
	pause();
}
