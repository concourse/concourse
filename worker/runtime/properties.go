package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
)

// propertiesToLabels converts a set of properties to a set of labels,
// splitting up the property values into multiple labels if they would exceed
// containerd's restriction on the length of the key+value for a single label.
//
// each label key is of the form: `key.SEQUENCE_NUMBER`, where SEQUENCE_NUMBER
// starts from 0 and counts up. for instance, a property `key: ...` may be
// stored in multiple labels like so:
//
// key.0: first chunk of value...
// key.1: ...second chunk of value...
// ...
// key.n: ...last chunk of value
//
func propertiesToLabels(properties garden.Properties) (map[string]string, error) {
	// Hard restriction on the total length of key + value imposed by
	// containerd on a per-label basis.
	const maxLabelLen = 4096

	// Restrict the key length to no more than half the label length.
	// This ratio is arbitrary, but helps ensure that:
	// 1. The key + sequence number suffix cannot exceed maxLabelLen, and
	// 2. We can fit a reasonable amount of data from the value in each chunk
	const maxKeyLen = maxLabelLen / 2

	labelSet := map[string]string{}
	for key, value := range properties {
		sequenceNum := 0
		if len(key) > maxKeyLen {
			return nil, fmt.Errorf("property name %q is too long", key[:32]+"...")
		}
		for {
			chunkKey := key + "." + strconv.Itoa(sequenceNum)
			valueLen := maxLabelLen - len(chunkKey)
			if valueLen > len(value) {
				valueLen = len(value)
			}

			labelSet[chunkKey] = value[:valueLen]
			value = value[valueLen:]

			if len(value) == 0 {
				break
			}

			sequenceNum++
		}
	}
	return labelSet, nil
}

// labelsToProperties is the inverse of propertiesToLabels. It combines all of
// the related labels into a single property by concatenating them together in
// order.
//
// Any labels that aren't of the correct format (i.e. key.n) will be ignored.
//
func labelsToProperties(labels map[string]string) garden.Properties {
	properties := garden.Properties{}
	for len(labels) > 0 {
		var key string
		// Pick an arbitrary chunk from the labels
		for k := range labels {
			key = k
			break
		}

		chunkSequenceStart := strings.LastIndexByte(key, '.')
		if chunkSequenceStart < 0 {
			// Not a properly formatted chunk. Just ignore.
			delete(labels, key)
			continue
		}

		propertyName := key[:chunkSequenceStart]

		var property strings.Builder
		for sequenceNum := 0; ; sequenceNum++ {
			chunkKey := propertyName + "." + strconv.Itoa(sequenceNum)
			chunkValue, ok := labels[chunkKey]
			if !ok {
				break
			}
			delete(labels, chunkKey)
			property.WriteString(chunkValue)
		}

		if property.Len() == 0 {
			// External components may add labels to containers that contain
			// '.' but aren't chunked properties. If we encounter a label that
			// has no chunks, just ignore it.
			delete(labels, key)
			continue
		}

		properties[propertyName] = property.String()
	}

	return properties
}

// propertiesToFilterList converts a set of garden properties to a list of
// filters as expected by containerd.
//
// containerd filters are in the form of
//
//           <what>.<field><operator><value>
//
// which, in our very specific case of properties, means
//
//           labels.foo==bar
//           |      |  | value
//           |      |  equality
//           |      key
//           what
//
// note that the key in this case represents the label key, which is not the
// same as the property key - refer to propertiesToLabels.
//
func propertiesToFilterList(properties garden.Properties) ([]string, error) {
	for k, v := range properties {
		if k == "" || v == "" {
			return nil, fmt.Errorf("key or value must not be empty")
		}
	}

	labels, err := propertiesToLabels(properties)
	if err != nil {
		return nil, err
	}

	filters := make([]string, 0, len(labels))

	for k, v := range labels {
		filters = append(filters, "labels."+k+"=="+v)
	}

	return filters, nil
}
