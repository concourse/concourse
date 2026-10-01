package volume

import "github.com/concourse/concourse/worker/baggageclaim/uidgid"

func NewRepositoryWithStreamers(
	filesystem Filesystem,
	locker LockManager,
	privilegedNamespacer uidgid.Namespacer,
	unprivilegedNamespacer uidgid.Namespacer,
	gzipStreamer, zstdStreamer, s2Streamer, rawStreamer Streamer,
) Repository {
	return &repository{
		filesystem:   filesystem,
		locker:       locker,
		gzipStreamer: gzipStreamer,
		zstdStreamer: zstdStreamer,
		s2Streamer:   s2Streamer,
		rawStreamer:  rawStreamer,
		namespacer: func(privileged bool) uidgid.Namespacer {
			if privileged {
				return privilegedNamespacer
			}
			return unprivilegedNamespacer
		},
	}
}
