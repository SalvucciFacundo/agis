package memory

import (
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// userKeyPrefix is the topic_key prefix that marks an observation as a durable
// fact about the user (as opposed to a project/technical note).
const userKeyPrefix = "user/"

// AggregateUserModel is a pure function that folds user/* observations into
// user-model rows.
//
// Only observations whose topic_key starts with "user/" participate. The
// output key is the observation's full topic_key. For a key absent from
// existing, confidence = clamp(importance/5, 0, 1). For a key already present
// (in existing or produced earlier in this batch), confidence =
// clamp(0.7*old + 0.3*new, 0, 1). The value is the observation's content;
// later writes for the same key win.
//
// Existing rows are preserved and re-emitted (with blended confidence when an
// observation touches them); new keys are appended in first-seen order.
func AggregateUserModel(existing []core.UserModel, obs []core.Observation) []core.UserModel {
	rows := make(map[string]core.UserModel, len(existing))
	order := make([]string, 0, len(existing)+len(obs))
	seen := make(map[string]bool, len(existing)+len(obs))

	add := func(u core.UserModel) {
		if !seen[u.Key] {
			seen[u.Key] = true
			order = append(order, u.Key)
		}
		rows[u.Key] = u
	}

	for _, u := range existing {
		add(u)
	}
	for _, o := range obs {
		if !strings.HasPrefix(o.TopicKey, userKeyPrefix) {
			continue
		}
		key := o.TopicKey
		conf := clamp01(float64(o.Importance) / 5.0)
		if old, ok := rows[key]; ok {
			conf = clamp01(0.7*old.Confidence + 0.3*conf)
		}
		add(core.UserModel{Key: key, Value: o.Content, Confidence: conf})
	}

	out := make([]core.UserModel, 0, len(order))
	for _, k := range order {
		out = append(out, rows[k])
	}
	return out
}

// clamp01 bounds a confidence score to [0,1].
func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
