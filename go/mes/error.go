package mes

import "fmt"

type Error uint32

// TODO: IF ANY OF THIS IS CHANGED, UPDATE THE requirements.md DOC

const (
	ErrBadType = (Error(^ProbType)) << 8 // invalid message type

	// tried to join a room but...
	ErrJoined    = (Error(JoinType) << 8) | 0x01 // you're already a member
	ErrBadName   = (Error(JoinType) << 8) | 0x02 // name is empty or too long
	ErrNameInUse = (Error(JoinType) << 8) | 0x03 // someone is already using that name
	ErrRoomLimit = (Error(JoinType) << 8) | 0x04 // you've reached your limit and can't join any more rooms
	ErrRoomFull  = (Error(JoinType) << 8) | 0x05 // it's full

	// tried to talk to a room but...
	ErrBadMessage = (Error(TalkType) << 8) | 0x01 // message is empty or too long

	// tried to talk to a room or exit from a roomS but...
	ErrBadRoom = (Error(TalkType|ExitType) << 8) | 0x01 // you haven't joined that room

	transientSuffix = Error(0xFF)
)

func ErrTransient(t Type) Error {
	return (Error(t) << 8) | transientSuffix
}

func ErrTransientExtra(t Type, pre uint8) Error {
	return (Error(pre) << 16) | ErrTransient(t)
}

func (e Error) Transient() bool {
	return e&0xFF == transientSuffix
}

func (e Error) Extra() uint8 {
	return uint8(e>>16) & 0xFF
}

var desc = map[Error]string{
	ErrBadType:    "bad message type",
	ErrJoined:     "you've already joined this room",
	ErrBadName:    "bad name, empty or too long",
	ErrNameInUse:  "name in use",
	ErrRoomLimit:  "you can't join any more rooms",
	ErrRoomFull:   "this room is full",
	ErrBadMessage: "bad message, empty or too long",
	ErrBadRoom:    "you haven't joined this room",
}

func (e Error) String() string {
	s, ok := desc[e]
	if ok {
		return s
	}
	if e.Transient() {
		origin := Error(e>>8) & 0xFF
		extra := ""
		if x := e.Extra(); x != 0 {
			extra = fmt.Sprintf(" (%02x)", x)
		}
		return fmt.Sprintf("%v: transient error%s, try again", origin, extra)
	}
	return fmt.Sprintf("0x%02x undefined error", uint32(e))
}

func (e Error) Error() string {
	return e.String()
}

var _ fmt.Stringer = ErrBadType
var _ error = ErrBadType
