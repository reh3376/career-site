package dtreport

// Summary is the handful of figures stored beside a report and shown in
// the admin console: enough to see what a participant was told without
// opening the PDF. It holds nothing about any question.
type Summary struct {
	Template string `json:"template"`
	Compared bool   `json:"compared"`
	// Why there is no ranking: small_group, repeat, unplaced or
	// synthetic. Empty when there is one.
	Reason string `json:"reason,omitempty"`
	// First-time participants the run could be compared with, not
	// counting itself.
	Others int `json:"others"`

	Score int `json:"score"`
	Total int `json:"total"`
	// Zero when not compared.
	Rank       int     `json:"rank,omitempty"`
	Tied       int     `json:"tied,omitempty"`
	GroupSize  int     `json:"group_size,omitempty"`
	Percentile float64 `json:"percentile,omitempty"`

	Rated          int      `json:"rated"`
	MeanConfidence *float64 `json:"mean_confidence"`
	Gap            *float64 `json:"gap"`
	MeanSeconds    *float64 `json:"mean_seconds"`
	Expired        int      `json:"expired"`
	Minutes        int      `json:"minutes"`
}

// Summary reduces the figures to what is stored.
func (f *Figures) Summary() Summary {
	s := Summary{
		Template: TemplateVersion, Compared: f.Compared, Reason: f.Reason, Others: f.Others,
		Score: f.Score, Total: f.Total, Rated: f.Rated,
		MeanConfidence: opt(f.me.meanConf), Gap: opt(f.me.gap), MeanSeconds: opt(f.me.lat),
		Expired: f.me.expired, Minutes: f.Minutes,
	}
	if f.Compared {
		s.Rank, s.Tied, s.GroupSize, s.Percentile = f.ScorePlace.Rank, f.ScorePlace.Tied, f.N, f.Percentile
	}
	return s
}
