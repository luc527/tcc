## Functional requirements

| Code | Description |
| ---- | ----------- |
| FR1  | The user can join a room |
| FR2  | The user can choose their display name in a room |
| FR3  | The user can send messages to a room |
| FR4  | The user can exit a room |
| FR5  | The system can send messages to an user |

## Non-functional requirements

`TODO`: I guess the non-functional requirements are actually "server must disconnect inactive clients" and "server must continuously check for client availability with a heartbeat";
the fact that we use `ping` and `pong` messages and the specific times are business rules, as they relate to the protocol which is our "business".
Well, maybe the specific times are actually non-functional requirements, it's just the message types that are business rules.

`TODO`: rate limiting, timeouts -- const.go except buffer sizes which are specific to the Go impl.
maybe even "the server CAN avoid sending a message to a room member if they're not available at the moment".
but it's more like "the server MAY rate limit" since the OS itself has send and receive buffers and so on, it's not something we can fully test for (I think?).
or rather the server MUST NOT process messages at a rate higher than X althoug allowing bursts of Y messages, something like that.

| Code  | Description |
| ----  | ----------- |
| NFR01 | The server periodically sends a `ping` message to each user, with an interval of 20 seconds. |
| NFR02 | Upon receiving a `ping`, the client host should immediatly send a `pong` back to the server. |
| NFR03 | If nothing is read from a client host for 30 seconds, the server has to close the connection. |

## Business rules

First, some important definitions:
- From the point of view of the server, each TCP client connection corresponds to exactly a single _user_.
- An user is said to be a _member_ of a room from the moment they join the room until the moment they exit the room.

| Code | Description |
| ---- | ----------- |
| BR01 | Users can be members of at most 256 rooms at any given moment. |
| BR02 | Room display names can't be empty, can't exceed 24 characters and can't contain the newline character `'\n'`.[^1] |
| BR03 | Messages can't be empty and can't exceed 2048 characters. |
| BR04 | Rooms can have at most 256 members at any given moment. |
| BR05 | Rooms are identified by integer numbers ranging from 0 to 2<sup>32</sup>-1. |
| BR06 | No two users can have the same display name in the same room at the same time. |
| BR07 | Users can send messages only to rooms they are currently members of. |
| BR08 | Users can join rooms with `join` messages containing the room number and the chosen display name. |
| BR09 | When an user joins a room, the server notifies every member with a `jned` message containing the room number and user's chosen display name. This includes the user themselves. |
| BR10 | Users can exit rooms with `exit` messages containing the room number. |
| BR11 | When an user exits a room, the server notifies every member with an `exed` message containing the room number and the users's former display name. This does **not** include the user themselves. |
| BR12 | Users can send messages to rooms with `talk` messages. |
| BR13 | When an user sends a message to a room, the server notifies every member with a `hear` message containing the room number, user display name in the room, and message contents. This includes the user themselves. |
| BR14 | Users can request a list of the rooms they are members of, along with the respective display names, with a `lsro` message. |
| BR15 | The server responds a `lsro` request with a `rols` message containing the list in CSV format. The first line is `room,name`, and each subsequent line has the form `<room>,<name>`, where `<room>` is the room number and `<name>` the display name the user chose for the room. |
| BR16 | When the server can't perform an action requested by the user, the server responds with a `prob` message. This includes both responses to invalid messages (e.g. name too loong, talking to a room the user hasn't joined, etc.) and internal server errors (i.e. transient errors).|
| BR17 | The system must send messages from a given room to clients in the order they were processed. |

[^1]: This is because the server returns the room list in CSV format.

### Notes

Technically, it's not required that the client host sends a `pong` back to the server when a `ping` is received.
Any message will reset the 30-second counter.