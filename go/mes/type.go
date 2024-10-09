package mes

import "fmt"

type Type uint8

const (
	PongType = Type(0x00)
	PingType = Type(0x80)
	TalkType = Type(0x01)
	HearType = Type(0x81)
	JoinType = Type(0x02)
	JnedType = Type(0x82)
	ExitType = Type(0x04)
	ExedType = Type(0x84)
	LsroType = Type(0x08)
	RolsType = Type(0x88)
	ProbType = Type(0x90)
	// not real types
	BegcType = Type(0x40)
	EndcType = Type(0x41)
)

type Sender uint8

const (
	ClientSender = Sender(iota)
	ServerSender
	NoneSender
)

func (t Type) SentBy() Sender {
	switch t {
	case PingType, HearType, JnedType, ExedType, RolsType:
		return ServerSender
	case PongType, TalkType, JoinType, ExitType, LsroType:
		return ClientSender
	default:
		return NoneSender
	}
}

func (t Type) Valid() bool {
	return false ||
		t == PingType || t == PongType ||
		t == TalkType || t == HearType ||
		t == JoinType || t == JnedType ||
		t == ExitType || t == ExedType ||
		t == LsroType || t == RolsType ||
		t == ProbType
}

func (t Type) HasRoom() bool {
	return false ||
		t == TalkType || t == HearType ||
		t == JoinType || t == JnedType ||
		t == ExitType || t == ExedType ||
		t == ProbType
}

func (t Type) HasName() bool {
	return false ||
		t == JoinType || t == JnedType || t == ExedType || t == HearType
}

func (t Type) HasText() bool {
	return t == TalkType || t == HearType || t == RolsType
}

func (t Type) String() string {
	switch t {
	case JoinType:
		return "join"
	case ExitType:
		return "exit"
	case TalkType:
		return "talk"
	case HearType:
		return "hear"
	case PingType:
		return "ping"
	case PongType:
		return "pong"
	case JnedType:
		return "jned"
	case ExedType:
		return "exed"
	case ProbType:
		return "prob"
	case LsroType:
		return "lsro"
	case RolsType:
		return "rols"
	case BegcType:
		return "begc"
	case EndcType:
		return "endc"
	default:
		return ""
	}
}

func ParseType(s string) (Type, error) {
	switch s {
	case "join":
		return JoinType, nil
	case "exit":
		return ExitType, nil
	case "talk":
		return TalkType, nil
	case "hear":
		return HearType, nil
	case "ping":
		return PingType, nil
	case "pong":
		return PongType, nil
	case "jned":
		return JnedType, nil
	case "exed":
		return ExedType, nil
	case "prob":
		return ProbType, nil
	case "lsro":
		return LsroType, nil
	case "rols":
		return RolsType, nil
	case "begc":
		return BegcType, nil
	case "endc":
		return EndcType, nil
	default:
		return 0, fmt.Errorf("invalid Type string %q", s)
	}
}
