package runtime

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const (
	uidMap = "/proc/self/uid_map"
	gidMap = "/proc/self/gid_map"
)

type userNamespace struct{}

func NewUserNamespace() UserNamespace {
	return &userNamespace{}
}

func (s *userNamespace) MaxValidIds() (uint32, uint32, error) {
	maxValidUid, err := maxValidFromFile(uidMap)
	if err != nil {
		return 0, 0, err
	}
	maxValidGid, err := maxValidFromFile(gidMap)
	if err != nil {
		return 0, 0, err
	}
	return maxValidUid, maxValidGid, nil
}

func maxValidFromFile(fname string) (uint32, error) {
	f, err := os.Open(uidMap)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", uidMap, err)
	}
	defer f.Close()

	return MaxValid(f)
}

// MaxValid computes what the highest possible id in a permission map is.
//
// For example, given the following mapping from /proc/self/uid_map:
//
// 	0 1001 10
// 	| |    |
// 	| |    max number of ids inside this mapping (3)
// 	| id outside (usually, the host)             (2)
// 	id inside the container                      (1)
//
// it determines that the maximum valid user id in this mapping is 9.
//
//
// More information about semantics of `uid_map` and `gid_map` can be found in
// [user_namespaces], but here's a summary assuming the processes reading the
// file is in the same usernamespace as `$pid`:
//
//   - each line specifies a 1:1 mapping of a range of contiguous user/group IDs
//     between two user namespaces
//
//       - (1) is the start of the range of ids in the user namespace of the
//             process $pid
//       - (2) is the start of the range of ids to which the user IDs specified
//             in (1) map to in the parent user namespace
//       - (3) is the length of the range of user/group IDs that is mapped
//             between the two user namespaces
//
//     where:
//        i. (1), (2), and (3) are uint32, with (3) having to be > 0
//       ii. the max number of lines is eiter 5 (linux <= 4.14) or 350 (linux >
//           4.14)
//      iii. range of ids in each line cannot overlap with the ranges in any
//           other lines
//       iv. at least one line must exist
//
//
// [user_namespaces]: http://man7.org/linux/man-pages/man7/user_namespaces.7.html
//
func MaxValid(r io.Reader) (uint32, error) {
	scanner := bufio.NewScanner(r)

	var (
		inside, outside, size uint32
		val                   uint32
		lines                 uint32
	)

	for scanner.Scan() {
		_, err := fmt.Sscanf(
			scanner.Text(),
			"%d %d %d",
			&inside, &outside, &size,
		)
		if err != nil {
			return 0, fmt.Errorf("scanf: %w", err)
		}

		val = maxUint(val, inside+size-1)
		lines++
	}

	err := scanner.Err()
	if err != nil {
		return 0, fmt.Errorf("scanning: %w", err)
	}

	if lines == 0 {
		return 0, fmt.Errorf("empty reader")
	}

	return val, nil
}

func maxUint(a, b uint32) uint32 {
	if a > b {
		return a
	}

	return b
}
