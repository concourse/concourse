package docs

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	prom "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/vito/booklit"
)

func (p Plugin) PromethusDocs(sample string) (booklit.Content, error) {
	parser := new(expfmt.TextParser)

	mfs, err := parser.TextToMetricFamilies(bytes.NewBufferString(sample))
	if err != nil {
		return nil, fmt.Errorf("failed to parse prometheus sample: %w", err)
	}

	type metric struct {
		name   string
		family *prom.MetricFamily
	}

	metrics := booklit.Sequence{}

	sorted := []metric{}
	for metricName, family := range mfs {
		sorted = append(sorted, metric{
			name:   metricName,
			family: family,
		})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].name < sorted[j].name
	})

	for _, metric := range sorted {
		metricName := metric.name
		family := metric.family

		distinctLabels := map[string]bool{}

		labels := booklit.Sequence{}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				labelName := label.GetName()

				if distinctLabels[labelName] {
					continue
				}

				distinctLabels[labelName] = true

				labels = append(labels, booklit.String(labelName))
			}
		}

		metricType := strings.ToLower(family.GetType().String())

		metrics = append(metrics, booklit.Styled{
			Style:   "prometheus-metric",
			Content: booklit.String(family.GetHelp()),
			Partials: booklit.Partials{
				"Name":   booklit.String(metricName),
				"Type":   booklit.String(metricType),
				"Labels": labels,
			},
		})
	}

	return metrics, nil
}

func (p Plugin) autoReferenceType(type_ string) booklit.Content {
	if strings.HasPrefix(type_, "[") && strings.HasSuffix(type_, "]") {
		subType := strings.TrimPrefix(strings.TrimSuffix(type_, "]"), "[")
		return booklit.Sequence{
			booklit.String("["),
			p.autoReferenceType(subType),
			booklit.String("]"),
		}
	}

	if strings.HasPrefix(type_, "{") && strings.HasSuffix(type_, "}") {
		subType := strings.TrimPrefix(strings.TrimSuffix(type_, "}"), "{")
		return booklit.Sequence{
			booklit.String("{"),
			p.autoReferenceType(subType),
			booklit.String("}"),
		}
	}

	for _, punc := range []string{" | ", ": ", ", "} {
		if strings.Contains(type_, punc) {
			ors := strings.Split(type_, punc)

			seq := booklit.Sequence{}
			for i, t := range ors {
				seq = append(seq, p.autoReferenceType(t))

				if i+1 < len(ors) {
					seq = append(seq, booklit.String(punc))
				}
			}

			return seq
		}
	}

	if strings.HasPrefix(type_, "`") && strings.HasSuffix(type_, "`") {
		scalar := strings.TrimPrefix(strings.TrimSuffix(type_, "`"), "`")
		return booklit.Styled{
			Style:   "schema-scalar",
			Content: booklit.String(scalar),
		}
	}

	return &booklit.Reference{
		TagName:  "schema." + type_,
		Location: p.section.InvokeLocation,
		Content: booklit.Styled{
			Style:   booklit.StyleBold,
			Content: booklit.String(type_),
		},
	}
}
