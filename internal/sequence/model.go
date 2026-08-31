package sequence

type Kind uint8

const (
	Request Kind = iota
	Return
)

type Participant struct {
	ID    string
	Label string
}

type Message struct {
	From  int
	To    int
	Label string
	Kind  Kind
}

type Diagram struct {
	Participants []Participant
	Messages     []Message
}

type Limits struct {
	MaxSourceBytes  int
	MaxLines        int
	MaxParticipants int
	MaxMessages     int
	MaxIDBytes      int
	MaxLabelCells   int
}

func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes:  256 * 1024,
		MaxLines:        2048,
		MaxParticipants: 16,
		MaxMessages:     96,
		MaxIDBytes:      64,
		MaxLabelCells:   96,
	}
}
