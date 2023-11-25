package task

const (
	PENDING  = "PENDING"
	SUCCEDED = "SUCCEDED"
	FAILED   = "FAILED"
)

type Status interface {
	Pending() bool
	Failed() bool
	Succeeded() bool
	StartedAt() int64
}

type Pending struct {
	StartedAt_ int64 `json:"started_at"`
}

func (s Pending) Pending() bool    { return true }
func (s Pending) Failed() bool     { return false }
func (s Pending) Succeeded() bool  { return false }
func (s Pending) StartedAt() int64 { return s.StartedAt_ }

type ResultStatus struct {
	StartedAt_ int64       `json:"started_at"`
	FinishedAt int64       `json:"finished_at"`
	Details    interface{} `json:"details"`
}
type Failed struct {
	ResultStatus
}

func (s Failed) Pending() bool    { return false }
func (s Failed) Failed() bool     { return true }
func (s Failed) Succeeded() bool  { return false }
func (s Failed) StartedAt() int64 { return s.StartedAt_ }

type Succeeded struct {
	ResultStatus
}

func (s Succeeded) Pending() bool    { return false }
func (s Succeeded) Failed() bool     { return false }
func (s Succeeded) Succeeded() bool  { return true }
func (s Succeeded) StartedAt() int64 { return s.StartedAt_ }

type PreparedStatus struct {
	Status     string      `json:"status"`
	StartedAt_ int64       `json:"started_at"`
	FinishedAt *int64      `json:"finished_at,omitempty"`
	Details    interface{} `json:"details,omitempty"`
}

func PrepareStatus(status Status) *PreparedStatus {
	switch ts := status.(type) {
	case Pending:
		return &PreparedStatus{
			Status:     PENDING,
			StartedAt_: ts.StartedAt_,
		}
	case Failed:
		return &PreparedStatus{
			Status:     FAILED,
			StartedAt_: ts.StartedAt_,
			FinishedAt: &ts.FinishedAt,
			Details:    ts.Details,
		}
	case Succeeded:
		return &PreparedStatus{
			Status:     SUCCEDED,
			StartedAt_: ts.StartedAt_,
			FinishedAt: &ts.FinishedAt,
			Details:    ts.Details,
		}
	}

	return nil
}
