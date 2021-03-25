package docs

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
	"github.com/vito/booklit"
)

const resolutionMerge = "resolution/merge"
const resolutionPostpone = "resolution/postpone"
const resolutionClose = "resolution/close"

var cachedRFCs []RFC
var cacheOnce = new(sync.Once)

type RFC struct {
	URL       string
	Number    int
	Title     string
	IsDraft   bool
	CreatedAt time.Time

	Labels    []GitHubLabel
	Reactions []GitHubReaction

	CommentCount int
	ReviewCount  int

	Proposal Proposal
}

type Proposal struct {
	URL string

	QuestionsURL  string
	QuestionCount int
}

func (pr RFC) ByTotalReactions() int {
	var s int
	for _, reaction := range pr.Reactions {
		s += reaction.Count
	}

	return s
}

func (pr RFC) ByOpenQuestions() int {
	return pr.Proposal.QuestionCount
}

func (pr RFC) ByCreatedAt() int {
	return int(pr.CreatedAt.Unix())
}

func (pr RFC) ByReviews() int {
	return pr.ReviewCount
}

func (pr RFC) Resolving() bool {
	return pr.HasLabel(resolutionMerge) ||
		pr.HasLabel(resolutionPostpone) ||
		pr.HasLabel(resolutionClose)
}

func (pr RFC) HasLabel(name string) bool {
	for _, label := range pr.Labels {
		if label.Name == name {
			return true
		}
	}

	return false
}

type GitHubLabel struct {
	Name  string
	Color string
}

type GitHubReaction struct {
	Emoji string
	Count int
}

func (p *Plugin) RfcsTable(countStr string, sortBy string) (booklit.Content, error) {
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return nil, fmt.Errorf("invalid rfc count: %w", err)
	}

	ctx := context.Background()

	cacheOnce.Do(func() {
		cachedRFCs, err = p.fetchRFCs(ctx)
	})
	if err != nil {
		return nil, err
	}

	rfcPRs := make([]RFC, len(cachedRFCs))
	copy(rfcPRs, cachedRFCs)

	sorter := prsBy{sortBy, rfcPRs}
	sort.Sort(sort.Reverse(sorter))

	rfcs := booklit.Sequence{}
	for i, rfc := range sorter.PRs {
		if i > count {
			break
		}

		rfcs = append(rfcs, rfcRow(rfc))
	}

	return booklit.Styled{
		Style:   "rfcs-table",
		Content: rfcs,
		Block:   true,
	}, nil
}

func rfcRow(rfc RFC) booklit.Content {
	var status booklit.Content
	switch {
	case rfc.HasLabel(resolutionMerge):
		status = booklit.Styled{
			Style:   "rfc-status",
			Content: booklit.String("merging"),
			Partials: booklit.Partials{
				"Class": booklit.String("pending-merge"),
			},
		}
	case rfc.HasLabel(resolutionClose):
		status = booklit.Styled{
			Style:   "rfc-status",
			Content: booklit.String("closing"),
			Partials: booklit.Partials{
				"Class": booklit.String("pending-close"),
			},
		}
	case rfc.HasLabel(resolutionPostpone):
		status = booklit.Styled{
			Style:   "rfc-status",
			Content: booklit.String("postponing"),
			Partials: booklit.Partials{
				"Class": booklit.String("pending-postpone"),
			},
		}
	default:
		status = booklit.Styled{
			Style: "rfc-status",
			Content: booklit.Link{
				Target:  rfc.URL + "/files",
				Content: booklit.String("open"),
			},
			Partials: booklit.Partials{
				"Class": booklit.String("open"),
			},
		}
	}

	reactions := booklit.Sequence{}
	for _, reaction := range rfc.Reactions {
		reactions = append(reactions, booklit.Styled{
			Style:   "rfc-reaction",
			Content: booklit.String(reaction.Emoji),
			Partials: booklit.Partials{
				"Count": booklit.String(strconv.Itoa(reaction.Count)),
			},
		})
	}

	var questions booklit.Content = booklit.Empty
	if rfc.Proposal.QuestionCount > 0 {
		questions = booklit.Styled{
			Style:   "rfc-questions",
			Content: booklit.String(strconv.Itoa(rfc.Proposal.QuestionCount)),
			Partials: booklit.Partials{
				"QuestionsURL": booklit.String(rfc.Proposal.QuestionsURL),
			},
		}
	}

	return booklit.Styled{
		Style: "rfc",
		Content: booklit.Link{
			Target:  rfc.URL,
			Content: booklit.String(rfc.Title),
		},
		Partials: booklit.Partials{
			"Number":      booklit.String(strconv.Itoa(rfc.Number)),
			"Status":      status,
			"Reactions":   reactions,
			"Questions":   questions,
			"ProposalURL": booklit.String(rfc.Proposal.URL),
		},
	}
}

var reactionEmoji = map[string]string{
	"THUMBS_UP":   "👍",
	"THUMBS_DOWN": "👎",
	"LAUGH":       "😆",
	"HOORAY":      "🙌",
	"CONFUSED":    "😕",
	"HEART":       "❤️",
	"ROCKET":      "🚀",
	"EYES":        "👀",
}

func (p *Plugin) fetchRFCs(ctx context.Context) ([]RFC, error) {
	client, ok := p.githubClient(ctx)
	if !ok {
		logrus.Warn("no $GITHUB_TOKEN set; using filler RFCs")
		return fillerRFCs, nil
	}

	logrus.Info("fetching RFCs")

	type repoId struct {
		Name  string
		Owner struct {
			Login string
		}
	}

	var prsQuery struct {
		Repository struct {
			PullRequests struct {
				Nodes []struct {
					Number  int
					Title   string
					IsDraft bool
					Url     string

					CreatedAt time.Time

					Labels struct {
						Nodes []GitHubLabel
					} `graphql:"labels(first:10)"`

					HeadRefName string
					HeadRefOid  string

					BaseRepository *repoId
					HeadRepository *repoId

					Files struct {
						Nodes []struct {
							Path string
						}
					} `graphql:"files(first:50)"`

					ReactionGroups []struct {
						Content string
						Users   struct {
							TotalCount int
						}
					}

					Comments struct {
						TotalCount int
					}

					Reviews struct {
						TotalCount int
					}
				}
			} `graphql:"pullRequests(first: 100, states: [OPEN])"`
		} `graphql:"repository(owner: \"concourse\", name: \"rfcs\")"`
	}

	err := client.Query(ctx, &prsQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch rfcs: %w", err)
	}

	pulls := []RFC{}
	for _, node := range prsQuery.Repository.PullRequests.Nodes {
		if node.IsDraft {
			// don't put drafts on the website; they're not ready for public
			// consumption. drafts are for more targeted feedback from individuals.
			continue
		}

		reactions := []GitHubReaction{}
		for _, rg := range node.ReactionGroups {
			if rg.Users.TotalCount == 0 {
				continue
			}

			emoji, found := reactionEmoji[rg.Content]
			if !found {
				return nil, fmt.Errorf("unknown reaction group: %s", rg.Content)
			}

			reactions = append(reactions, GitHubReaction{
				Emoji: emoji,
				Count: rg.Users.TotalCount,
			})
		}

		var proposalPath string
		for _, file := range node.Files.Nodes {
			if strings.HasSuffix(file.Path, "proposal.md") {
				proposalPath = file.Path
			}
		}

		var repo repoId
		if node.HeadRepository != nil {
			repo = *node.HeadRepository
		} else {
			repo = *node.BaseRepository
		}

		proposalURL := fmt.Sprintf(
			"https://github.com/%s/%s/blob/%s/%s",
			repo.Owner.Login,
			repo.Name,
			node.HeadRefOid,
			proposalPath,
		)

		proposal, found, err := fetchProposal(proposalURL)
		if err != nil {
			return nil, fmt.Errorf("count open questions: %w", err)
		}

		if !found {
			continue
		}

		pulls = append(pulls, RFC{
			URL:       node.Url,
			Number:    node.Number,
			CreatedAt: node.CreatedAt,
			Title:     node.Title,
			IsDraft:   node.IsDraft,
			Labels:    node.Labels.Nodes,

			Proposal: proposal,

			Reactions:    reactions,
			CommentCount: node.Comments.TotalCount,
			ReviewCount:  node.Reviews.TotalCount,
		})
	}

	return pulls, nil
}

func fetchProposal(proposalURL string) (Proposal, bool, error) {
	resp, err := http.Get(proposalURL)
	if err != nil {
		return Proposal{}, false, fmt.Errorf("get %s: %w", proposalURL, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return Proposal{}, false, nil
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return Proposal{}, false, fmt.Errorf("bad response for %s: %s", proposalURL, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Proposal{}, false, fmt.Errorf("query HTML: %w", err)
	}

	proposal := Proposal{
		URL: proposalURL,
	}

	doc.Find("#readme").Find("h1").Each(func(i int, sel *goquery.Selection) {
		if sel.Text() == "Open Questions" {
			anchor, found := sel.Find("a.anchor").Attr("href")
			if found {
				proposal.QuestionsURL = proposalURL + anchor
				logrus.Debugf("open questions: %s", proposal.QuestionsURL)
			}

			questions := sel.NextUntil("h1")

			countQuestion := func(i int, sel *goquery.Selection) {
				if strings.Contains(sel.Text(), "?") {
					logrus.WithFields(logrus.Fields{
						"question": sel.Text(),
					}).Debug("found question")

					proposal.QuestionCount++
				}
			}

			questions.Filter("h2").Each(countQuestion)
			questions.Find("li").Each(countQuestion)
			questions.Find("p").Each(countQuestion)

			logrus.Debugf("total questions: %d", proposal.QuestionCount)
		}
	})

	return proposal, true, nil
}

type prsBy struct {
	Method string
	PRs    []RFC
}

func (by prsBy) Len() int      { return len(by.PRs) }
func (by prsBy) Swap(i, j int) { by.PRs[i], by.PRs[j] = by.PRs[j], by.PRs[i] }

func (by prsBy) Less(i, j int) bool {
	pri := by.PRs[i]
	prj := by.PRs[j]

	// regardless of order, show resolving PRs first
	switch {
	case pri.Resolving() && !prj.Resolving():
		return false
	case !pri.Resolving() && prj.Resolving():
		return true
	}

	ret := reflect.ValueOf(pri).MethodByName(by.Method).Call(nil)
	a := ret[0].Interface().(int)

	ret = reflect.ValueOf(prj).MethodByName(by.Method).Call(nil)
	b := ret[0].Interface().(int)

	return a < b
}

var fillerRFCs = []RFC{
	{
		URL:    "https://example.com",
		Number: 42,
		Title:  "Open RFC",

		Proposal: Proposal{
			URL:           "https://example.com/#proposal",
			QuestionsURL:  "https://example.com/#questions",
			QuestionCount: 3,
		},

		Reactions: []GitHubReaction{
			{
				Emoji: reactionEmoji["ROCKET"],
				Count: 5,
			},
		},
	},
	{
		URL:    "https://example.com",
		Number: 42,
		Title:  "Another Open RFC",

		Proposal: Proposal{
			URL:           "https://example.com/#proposal",
			QuestionsURL:  "https://example.com/#questions",
			QuestionCount: 3,
		},

		Reactions: []GitHubReaction{
			{
				Emoji: reactionEmoji["THUMBS_UP"],
				Count: 12,
			},
		},
	},
	{
		URL:    "https://example.com",
		Number: 43,
		Title:  "Merging RFC",

		Proposal: Proposal{
			URL:           "https://example.com/#proposal",
			QuestionsURL:  "https://example.com/#questions",
			QuestionCount: 0,
		},

		Labels: []GitHubLabel{
			{
				Name:  resolutionMerge,
				Color: "#ff00ff",
			},
		},

		Reactions: []GitHubReaction{
			{
				Emoji: reactionEmoji["ROCKET"],
				Count: 10,
			},
		},
	},
	{
		URL:    "https://example.com",
		Number: 12,
		Title:  "Closing RFC",

		Proposal: Proposal{
			URL:           "https://example.com/#proposal",
			QuestionsURL:  "https://example.com/#questions",
			QuestionCount: 1,
		},

		Labels: []GitHubLabel{
			{
				Name:  resolutionClose,
				Color: "#ff00ff",
			},
		},

		Reactions: []GitHubReaction{
			{
				Emoji: reactionEmoji["THUMBS_DOWN"],
				Count: 1,
			},
		},
	},
	{
		URL:    "https://example.com",
		Number: 2,
		Title:  "Postponing RFC",

		Proposal: Proposal{
			URL:           "https://example.com/#proposal",
			QuestionsURL:  "https://example.com/#questions",
			QuestionCount: 3,
		},

		Labels: []GitHubLabel{
			{
				Name:  resolutionPostpone,
				Color: "#ff00ff",
			},
		},

		Reactions: []GitHubReaction{
			{
				Emoji: reactionEmoji["CONFUSED"],
				Count: 2,
			},
		},
	},
}
